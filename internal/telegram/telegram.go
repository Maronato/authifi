package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/maronato/authifi/internal/config"
	"github.com/maronato/authifi/internal/database"
	"github.com/maronato/authifi/internal/logging"
	"golang.org/x/sync/errgroup"
	tele "gopkg.in/telebot.v3"
	telemiddleware "gopkg.in/telebot.v3/middleware"
)

const (
	// PollerTimeout is the default timeout for the bot poller.
	PollerTimeout = 10 * time.Second
	// VLANSelectCacheSize is the default size of the VLAN select cache.
	VLANSelectCacheSize = 100
	// RandomIDLength is the default length of the random IDs.
	RandomIDLength = 32
)

// BotServer is a Telegram bot server.
type BotServer struct {
	// bot is the Telegram bot.
	bot *tele.Bot
	// chatIDs is a list of chat IDs that the bot is allowed to interact with.
	chatIDs []int64
	// db is the database.
	db database.Database
	// l is the logger.
	l *slog.Logger
	// createNewDeviceMessage creates a notification message for a new device.
	createNewDeviceMessage func(data *newDeviceData) (string, *tele.ReplyMarkup)
}

// NewBotServer creates a new BotServer.
func NewBotServer(ctx context.Context, cfg *config.Config, db database.Database) (*BotServer, error) {
	l := logging.FromCtx(ctx)

	onTextHandlers := []tele.HandlerFunc{}

	// Create the bot
	bot, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: PollerTimeout},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}

	// Setup error recovery middleware
	bot.Use(telemiddleware.Recover())

	// Setup chat allowlist
	chatIDs := make([]int64, len(cfg.TelegramChatIDs))

	for i, id := range cfg.TelegramChatIDs {
		intID, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("error converting chat ID to int: %w", err)
		}

		chatIDs[i] = int64(intID)
	}

	bot.Use(telemiddleware.Whitelist(chatIDs...))

	// Setup access logs middleware
	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if cfg.Verbose >= config.VerboseLevelAccessLogs {
				msgData := slog.Group("message",
					slog.String("username", c.Chat().Username),
					slog.Int64("chat_id", c.Chat().ID),
					slog.Int("message_id", c.Message().ID),
					slog.String("text", c.Text()))

				l := l.With(msgData)
				l.Info("Received message", "chatID", c.Chat().ID, "text", c.Text())
			}

			return hf(c)
		}
	})

	bot.Handle("/start", func(c tele.Context) error {
		err := bot.SetCommands(
			[]tele.Command{
				{Text: "/list", Description: "List all the devices"},
				{Text: "/edit", Description: "Edit a device"},
				{Text: "/help", Description: "Show help message"},
			},
			tele.CommandScope{Type: tele.CommandScopeChat, ChatID: c.Chat().ID},
		)
		if err != nil {
			return fmt.Errorf("error setting commands: %w", err)
		}

		// Send the welcome message and the menu
		if err := c.Send("Welcome to Authifi! Use /help to see the available commands.", tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		return nil
	})

	helpMessage := `*🤖 Authifi Bot Help 🤖*

	Welcome to the Auhtifi Bot!

	Now that it's setup, you will receive alerts when a new device connects to your networks. You can choose to add, ignore, or block the device using the inline commands.
	
	*Commands:*
	- /start - Start interacting with the bot.
	- /list - List all the devices.
	- /edit <device> - Edit a device by its name or username.
	- /help - Show this help message.
	Other commands *may* be implemented in the future.

	Update the database file directly to manually add, remove, or modify devices.`

	bot.Handle("/help", func(c tele.Context) error {
		if err := c.Send(helpMessage, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		return nil
	})

	bot.Handle("/list", func(c tele.Context) error {
		devices, err := db.GetUsers()
		if err != nil {
			return fmt.Errorf("error getting devices: %w", err)
		}

		blockedDevices, err := db.GetBlockedUsers()
		if err != nil {
			return fmt.Errorf("error getting blocked devices: %w", err)
		}

		vlans, err := db.GetVLANs()
		if err != nil {
			return fmt.Errorf("error getting VLANs: %w", err)
		}

		// Create a map of VLANs for easy access
		vlanMap := make(map[string]string, len(vlans))
		for _, vlan := range vlans {
			vlanMap[vlan.ID] = vlan.Name
		}

		msg := "*📋 Device List 📋*\n\n"

		for _, device := range devices {
			if device.Description == "" {
				msg += fmt.Sprintf("• *%s* - %s\n", device.Username, vlanMap[device.VlanID])
			} else {
				msg += fmt.Sprintf("• *%s* (%s) - %s\n", device.Description, device.Username, vlanMap[device.VlanID])
			}
		}

		msg += "\n*🚫 Blocked Devices 🚫*\n\n"
		for _, device := range blockedDevices {
			msg += fmt.Sprintf("• *%s*\n", device.Username)
		}

		if err := c.Send(msg, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		return nil
	})

	// Setup new device handlers and cache
	createNewDeviceMessage := registerNewDeviceFlow(bot, db, &onTextHandlers)

	// Setup edit device handlers
	registerEditDeviceFlow(bot, db, &onTextHandlers)

	// Handle onText events
	bot.Handle(tele.OnText, func(c tele.Context) error {
		for _, handler := range onTextHandlers {
			if err := handler(c); err != nil {
				return fmt.Errorf("error handling text: %w", err)
			}
		}

		return nil
	})

	// Replace everything after ":" and before the last 5 characters with "*"
	privacyToken := cfg.TelegramBotToken

	tokenSplit := strings.Split(privacyToken, ":")
	if len(tokenSplit) > 1 {
		starLen := len(tokenSplit[1]) - 5 //nolint:gomnd // Won't create a constant for this
		privacyToken = tokenSplit[0] + ":" + strings.Repeat("*", starLen) + tokenSplit[1][starLen:]
	} else {
		privacyToken = strings.Repeat("*", len(privacyToken))
	}

	l.Debug("Bot setup complete", slog.Any("chatIDs", chatIDs), slog.Int("cacheSize", VLANSelectCacheSize), slog.Int("randomIDLength", RandomIDLength), slog.Duration("pollerTimeout", PollerTimeout), slog.String("token", privacyToken))

	return &BotServer{bot: bot, chatIDs: chatIDs, db: db, createNewDeviceMessage: createNewDeviceMessage, l: l}, nil
}

// StartBot starts the Telegram bot.
func (bs *BotServer) StartBot(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)

	l := logging.FromCtx(ctx)

	eg.Go(func() error {
		l.Info("Starting Telegram bot with " + fmt.Sprint(len(bs.chatIDs)) + " allowed chat IDs")

		bs.bot.Start()

		return nil
	})

	eg.Go(func() error {
		<-egCtx.Done()

		l.Debug("Shutting down Telegram bot")

		bs.bot.Stop()

		return nil
	})

	// Wait for the server to exit and check for errors that
	// are not caused by the context being canceled.
	if err := eg.Wait(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("bot exited with error: %w", err)
	}

	return nil
}

// NotifyLoginAttempt sends a message to all the chat IDs when a login attempt is detected.
func (bs *BotServer) NotifyLoginAttempt(username, password, macAddress string) {
	bs.l.Debug("Sending login attempt notification", slog.String("username", username), slog.String("macAddress", macAddress))

	data := &newDeviceData{
		Username:   username,
		Password:   password,
		MacAddress: macAddress,
	}

	for _, chatID := range bs.chatIDs {
		recipient := tele.ChatID(chatID)

		msg, markup := bs.createNewDeviceMessage(data)

		if _, err := bs.bot.Send(recipient, msg, markup, tele.ModeMarkdown); err != nil {
			bs.l.Error("Error sending message", slog.Any("error", err), slog.Int64("chatID", chatID), slog.String("message", msg))
		}
	}
}
