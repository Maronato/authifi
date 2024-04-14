package telegram

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/maronato/authifi/internal/config"
	"github.com/maronato/authifi/internal/database"
	"github.com/maronato/authifi/internal/logging"
	"github.com/maronato/authifi/internal/lru"
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
	// cache is the cache for the VLAN select data.
	cache *lru.Cache[string, *VLANSelectData]
}

// NewBotServer creates a new BotServer.
func NewBotServer(ctx context.Context, cfg *config.Config, db database.Database) (*BotServer, error) {
	l := logging.FromCtx(ctx)

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

	// This will be the default menu for the bot
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	btnHelp := menu.Text("Help")

	menu.Reply(
		menu.Row(btnHelp),
	)

	bot.Handle("/start", func(c tele.Context) error {
		// Send the welcome message and the menu
		err := c.Send("Welcome to Authifi! Use /help to see the available commands.", menu, tele.ModeMarkdown)
		if err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		return nil
	})

	helpMessage := `*🤖 Authifi Bot Help 🤖*

	Welcome to the Auhtifi Bot!

	Now that it's setup, you will receive alerts when a new device connects to your networks. You can choose to add, ignore, or block the device using the inline commands.
	
	*Commands:*
	- /start - Start interacting with the bot.
	- /help - Show this help message.
	Other commands *may* be implemented in the future.

	Update the database file directly to manually add, remove, or modify devices.`

	bot.Handle("/help", func(c tele.Context) error {
		if err := c.Send(helpMessage, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		return nil
	})

	bot.Handle(&btnHelp, func(c tele.Context) error {
		if err := c.Send(helpMessage, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		return nil
	})

	// Setup new user handlers and cache
	cache := lru.NewLRUCache[string, *VLANSelectData](VLANSelectCacheSize)
	registerNewUserHandler(bot, db, cache)

	return &BotServer{bot: bot, chatIDs: chatIDs, db: db, cache: cache}, nil
}

// StartBot starts the Telegram bot.
func (bs *BotServer) StartBot(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)

	l := logging.FromCtx(ctx)

	eg.Go(func() error {
		l.Debug("Starting Telegram bot")

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
	for _, chatID := range bs.chatIDs {
		recipient := tele.ChatID(chatID)
		data := &VLANSelectData{
			Username:   username,
			Password:   password,
			MacAddress: macAddress,
		}

		msg, markup := createNotifyMessage(bs.bot, bs.cache, data)

		if _, err := bs.bot.Send(recipient, msg, markup, tele.ModeMarkdown); err != nil {
			log.Printf("error sending message to chat %d: %v\n", chatID, err)
		}
	}
}
