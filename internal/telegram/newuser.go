package telegram

import (
	"fmt"
	"time"

	"github.com/maronato/authifi/internal/database"
	"github.com/maronato/authifi/internal/lru"
	tele "gopkg.in/telebot.v3"
)

const (
	// Inline reply buttons.
	btnAddUnique        = "add"
	btnSelectVLANUnique = "select-vlan"
	btnBackAddUnique    = "back-add"
	btnIgnoreUnique     = "ignore"
	btnBlocklistUnique  = "blocklist"
)

// ErrFailedToReadData is returned when the data from the message could not be read.
var ErrFailedToReadData = fmt.Errorf("failed to read data from message")

// createNotifyMessage creates a notification message for a new user.
func createNotifyMessage(bot *tele.Bot, cache *lru.Cache[string, *VLANSelectData], data *VLANSelectData) (string, *tele.ReplyMarkup) {
	m := bot.NewMarkup()

	dataID := createRandomID(RandomIDLength)
	cache.Set(dataID, data)

	btnAdd := m.Data("✅ Add User", btnAddUnique, dataID).Inline()
	btnIgnore := m.Data("❌ Ignore Request", btnIgnoreUnique, dataID).Inline()
	btnBlock := m.Data("🔒 Block User", btnBlocklistUnique, dataID).Inline()
	m.InlineKeyboard = [][]tele.InlineButton{{*btnAdd}, {*btnIgnore}, {*btnBlock}}

	// Markdown message
	msg := fmt.Sprintf(`*🚨 New Device Detected! 🚨*
	
	*Username:* `+"`%s`"+`
	*Mac Address:* `+"`%s`"+`
	*Connection time:* `+"`%s`"+`
	
	What would you like to do?`,
		data.Username, data.MacAddress, time.Now().Format(time.RFC1123))

	return msg, m
}

// registerNewUserHandler registers the handlers for the new user process.
func registerNewUserHandler(bot *tele.Bot, db database.Database, cache *lru.Cache[string, *VLANSelectData]) {
	// Handle the "Add" button
	bot.Handle(&tele.InlineButton{Unique: btnAddUnique}, func(c tele.Context) error {
		dataID := c.Data()

		data, ok := cache.Get(dataID)
		if !ok {
			return ErrFailedToReadData
		}

		// Show the VLAN selection menu
		m := bot.NewMarkup()

		// Get all VLANs to show in the menu
		vlans, err := db.GetVLANs()
		if err != nil {
			return fmt.Errorf("error getting VLANs: %w", err)
		}

		// Build the inline keyboard with the VLANs
		for i, vlan := range vlans {
			selectedData := &VLANSelectData{
				Username:   data.Username,
				Password:   data.Password,
				VlanID:     vlan.ID,
				MacAddress: data.MacAddress,
			}

			selectedDataID := createRandomID(RandomIDLength)
			cache.Set(selectedDataID, selectedData)

			btn := m.Data(vlan.Name, btnSelectVLANUnique, selectedDataID).Inline()
			// Add up to 3 buttons per row
			if i%3 == 0 {
				m.InlineKeyboard = append(m.InlineKeyboard, []tele.InlineButton{*btn})
			} else {
				m.InlineKeyboard[len(m.InlineKeyboard)-1] = append(m.InlineKeyboard[len(m.InlineKeyboard)-1], *btn)
			}
		}

		// Add a back button, reuse the same data
		btn := m.Data("⬅ Back", btnBackAddUnique, dataID).Inline()
		m.InlineKeyboard = append(m.InlineKeyboard, []tele.InlineButton{*btn})

		// Edit the message with the VLAN selection menu
		msg := fmt.Sprintf(`*👤 Add `+"`%s`"+` to Network*
		
		Please select which network you would like to add this device to:`,
			data.Username)

		err = c.Edit(msg, m, tele.ModeMarkdown)
		if err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the selection of a VLAN by the user
	bot.Handle(&tele.InlineButton{Unique: btnSelectVLANUnique}, func(c tele.Context) error {
		data, ok := cache.Get(c.Data())
		if !ok {
			return ErrFailedToReadData
		}

		// Get the selected VLAN
		vlan, err := db.GetVLAN(data.VlanID)
		if err != nil {
			return fmt.Errorf("error getting VLAN: %w", err)
		}

		// Create user
		if err := db.CreateUser(database.User{
			Username: data.Username,
			Password: data.Password,
			VlanID:   vlan.ID,
		}); err != nil {
			return fmt.Errorf("error creating user: %w", err)
		}

		// Edit the message with the success message
		msg := fmt.Sprintf(`*✅ Success! ✅*
		
		`+"`%s`"+` has been added to the *%s* network.`,
			data.Username, vlan.Name,
		)

		if err := c.Edit(msg, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the back button from the VLAN selection menu
	bot.Handle(&tele.InlineButton{Unique: btnBackAddUnique}, func(c tele.Context) error {
		data, ok := cache.Get(c.Data())
		if !ok {
			return ErrFailedToReadData
		}

		// Recreate the notification message
		msg, markup := createNotifyMessage(bot, cache, data)

		// Edit the message with the notification message
		if err := c.Edit(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the "Ignore" button
	bot.Handle(&tele.InlineButton{Unique: btnIgnoreUnique}, func(c tele.Context) error {
		data, ok := cache.Get(c.Data())
		if !ok {
			return ErrFailedToReadData
		}

		// Edit the message with the ignore message
		msg := fmt.Sprintf(`*🚫 Request Ignored 🚫*
		
		No action has been taken for `+"`%s`"+`.`,
			data.Username)

		if err := c.Edit(msg, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the "Block" button
	bot.Handle(&tele.InlineButton{Unique: btnBlocklistUnique}, func(c tele.Context) error {
		data, ok := cache.Get(c.Data())
		if !ok {
			return ErrFailedToReadData
		}

		// Block user
		if err := db.BlockUser(data.Username); err != nil {
			return fmt.Errorf("error blocking user: %w", err)
		}

		// Edit the message with the block message
		msg := fmt.Sprintf(`*🔒 User Blocked 🔒*
		
		`+"`%s`"+` has been blocked and further connections will be ignored.`,
			data.Username)

		if err := c.Edit(msg, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})
}
