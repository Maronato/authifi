package telegram

import (
	"fmt"
	"regexp"
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

	// Edit inline reply buttons.
	btnEditChangeVLANUnique = "edit-change-vlan"
	btnEditBlockUnique      = "edit-block"
	btnEditUnblockUnique    = "edit-unblock"
	btnEditDeleteUnique     = "edit-delete"
	btnEditBackUnique       = "edit-back"
	btnEditSelectVLANUnique = "edit-select-vlan"

	// newDeviceDataCacheSize is the default size of the new device data cache.
	newDeviceDataCacheSize = 100
	// editDeviceDataCacheSize is the default size of the edit device data cache.
	editDeviceDataCacheSize = 10
)

type newDeviceData struct {
	// Username is the username of the device.
	Username string
	// Password is the password of the device.
	Password string
	// VlanID is the ID of the VLAN. It's empty if the user didn't select any VLAN.
	VlanID string
	// MacAddress is the MAC address of the device.
	MacAddress string
	// Description is the custom assigned name of the device. It's empty by default.
	Description string
}

func extractUsernameFromNewDeviceMessage(text string) string {
	re := regexp.MustCompile(`\s+(.*?) has been added to the`)

	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func extractUsernameFromEditDeviceMessage(text string) string {
	// Regex that finds "Username: <username>"
	re := regexp.MustCompile(`Username: (.*?)\n`)

	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func buildVLANSelectMenu(bot *tele.Bot, db database.Database, selectVLANUnique string, getDataID func(vlanID string) string) (*tele.ReplyMarkup, error) {
	// Show the VLAN selection menu
	m := bot.NewMarkup()

	// Get all VLANs to show in the menu
	vlans, err := db.GetVLANs()
	if err != nil {
		return nil, fmt.Errorf("error getting VLANs: %w", err)
	}

	// Build the inline keyboard with the VLANs
	for i, vlan := range vlans {
		selectedDataID := getDataID(vlan.ID)

		btn := m.Data(vlan.Name, selectVLANUnique, selectedDataID).Inline()
		// Add up to 3 buttons per row
		if i%3 == 0 {
			m.InlineKeyboard = append(m.InlineKeyboard, []tele.InlineButton{*btn})
		} else {
			m.InlineKeyboard[len(m.InlineKeyboard)-1] = append(m.InlineKeyboard[len(m.InlineKeyboard)-1], *btn)
		}
	}

	return m, nil
}

// registerNewDeviceFlow registers the handlers for the new device flow.
func registerNewDeviceFlow(bot *tele.Bot, db database.Database, onTextHandlers *[]tele.HandlerFunc) func(data *newDeviceData) (string, *tele.ReplyMarkup) { //nolint:maintidx // I want to keep the function signature as is
	// Create the cache that will persist new user data across the new user flow
	newDeviceCache := lru.NewLRUCache[string, *newDeviceData](newDeviceDataCacheSize)

	// createNotifyMessage creates a notification message for a new user.
	createNotifyMessage := func(data *newDeviceData) (string, *tele.ReplyMarkup) {
		m := bot.NewMarkup()

		dataID := createRandomID()
		newDeviceCache.Set(dataID, data)

		btnAdd := m.Data("✅ Add Device", btnAddUnique, dataID).Inline()
		btnIgnore := m.Data("❌ Ignore Request", btnIgnoreUnique, dataID).Inline()
		btnBlock := m.Data("🔒 Block Device", btnBlocklistUnique, dataID).Inline()
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

	// Handle the "Add" button
	bot.Handle(&tele.InlineButton{Unique: btnAddUnique}, func(c tele.Context) error {
		dataID := c.Data()

		data, ok := newDeviceCache.Get(dataID)
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
			selectedData := &newDeviceData{
				Username:   data.Username,
				Password:   data.Password,
				VlanID:     vlan.ID,
				MacAddress: data.MacAddress,
			}

			selectedDataID := createRandomID()
			newDeviceCache.Set(selectedDataID, selectedData)

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
		data, ok := newDeviceCache.Get(c.Data())
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
		
		`+"`%s`"+` has been added to the *%s* network.
		
		You may reply to this message with a name to assign to this device.`,
			data.Username, vlan.Name,
		)

		if err := c.Edit(msg, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle replies to the message with the device name
	*onTextHandlers = append(*onTextHandlers, func(c tele.Context) error {
		if c.Message().IsReply() {
			reply := c.Message()
			original := c.Message().ReplyTo

			// Extract username from the original message using regex
			username := extractUsernameFromNewDeviceMessage(original.Text)

			// Abort if username is empty
			if username == "" {
				return nil
			}

			// Get the user
			user, err := db.GetUser(username)
			if err != nil {
				// Ignore if the user doesn't exist
				return nil //nolint:nilerr // Fail silently
			}

			// Update the user with the description
			user.Description = reply.Text
			if err := db.UpdateUser(user); err != nil {
				return fmt.Errorf("error updating user: %w", err)
			}

			// Send the success message
			msg := fmt.Sprintf(`Saved the name *%s* for the device *%s*.`,
				user.Description, user.Username)

			if err := c.Send(msg, tele.ModeMarkdown); err != nil {
				return fmt.Errorf("error sending message: %w", err)
			}

			return nil
		}

		return nil
	})

	// Handle the back button from the VLAN selection menu
	bot.Handle(&tele.InlineButton{Unique: btnBackAddUnique}, func(c tele.Context) error {
		data, ok := newDeviceCache.Get(c.Data())
		if !ok {
			return ErrFailedToReadData
		}

		// Recreate the notification message
		msg, markup := createNotifyMessage(data)

		// Edit the message with the notification message
		if err := c.Edit(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the "Ignore" button
	bot.Handle(&tele.InlineButton{Unique: btnIgnoreUnique}, func(c tele.Context) error {
		data, ok := newDeviceCache.Get(c.Data())
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
		data, ok := newDeviceCache.Get(c.Data())
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

	// Return function to create a message for admin notification
	return createNotifyMessage
}

type editDeviceData struct {
	// Username is the username of the device.
	Username string
	// VlanID is the ID of the VLAN. It's empty if the user didn't select any VLAN.
	VlanID string
}

func registerEditDeviceFlow(bot *tele.Bot, db database.Database, onTextHandlers *[]tele.HandlerFunc) { //nolint:gocyclo,maintidx // big func good
	editDeviceCache := lru.NewLRUCache[string, *editDeviceData](editDeviceDataCacheSize)

	buildEditMessage := func(username string) (string, *tele.ReplyMarkup, error) {
		// Check if the user is blocked
		blocked, err := db.IsUserBlocked(username)
		if err != nil {
			return "", nil, fmt.Errorf("error checking if user is blocked: %w", err)
		}

		msg := "*📝 Edit Device 📝*\n"

		if blocked {
			msg += "\n*🔒 This device is blocked 🔒*\n"
		}

		user, err := db.GetUser(username)
		if err != nil && !blocked {
			// Send message that the user doesn't exist
			return fmt.Sprintf("*🚫 User Not Found 🚫*\n\n`%s` does not exist.", username), nil, nil
		}

		vlan, err := db.GetVLAN(user.VlanID)
		if err != nil {
			// If the user doesn't have a VLAN, create a temp VLAN with no name
			if user.VlanID == "" {
				vlan = database.VLAN{}
			} else {
				return "", nil, fmt.Errorf("error getting VLAN: %w", err)
			}
		}

		msg += fmt.Sprintf(`
		*Name:* %s
		*Username:* %s
		*VLAN:* %s
		
		You may reply to this message with a new name for this device.`, user.Description, username, vlan.Name)

		m := bot.NewMarkup()

		dataID := createRandomID()
		editDeviceCache.Set(dataID, &editDeviceData{Username: username})

		btnChangeVLAN := m.Data("🔄 Change VLAN", btnEditChangeVLANUnique, dataID).Inline()
		btnBlock := m.Data("🔒 Block", btnEditBlockUnique, dataID).Inline()
		btnUnblock := m.Data("🔓 Unblock", btnEditUnblockUnique, dataID).Inline()
		btnDelete := m.Data("🗑 Delete", btnEditDeleteUnique, dataID).Inline()

		if blocked {
			m.InlineKeyboard = [][]tele.InlineButton{{*btnUnblock}, {*btnDelete}}
		} else {
			m.InlineKeyboard = [][]tele.InlineButton{{*btnChangeVLAN}, {*btnBlock}, {*btnDelete}}
		}

		return msg, m, nil
	}

	// Handle edit command
	bot.Handle("/edit", func(c tele.Context) error {
		username := c.Message().Payload

		// Handle empty payload
		if username == "" {
			if err := c.Send("Please provide a name or username to edit. Usage:\n`/edit <device>`", tele.ModeMarkdown); err != nil {
				return fmt.Errorf("error sending message: %w", err)
			}

			return nil
		}

		// Maybe it's the description
		user, err := db.GetUserByDescription(username)
		if err == nil {
			username = user.Username
		}

		msg, markup, err := buildEditMessage(username)
		if err != nil {
			return fmt.Errorf("error building edit message: %w", err)
		}

		if err := c.Send(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		return nil
	})

	// Handle the change VLAN button
	bot.Handle(&tele.InlineButton{Unique: btnEditChangeVLANUnique}, func(c tele.Context) error {
		dataID := c.Data()

		data, ok := editDeviceCache.Get(dataID)
		if !ok {
			return ErrFailedToReadData
		}

		markup, err := buildVLANSelectMenu(bot, db, btnEditSelectVLANUnique, func(vlanID string) string {
			dataID := createRandomID()
			editDeviceCache.Set(dataID, &editDeviceData{Username: data.Username, VlanID: vlanID})

			return dataID
		})
		if err != nil {
			return fmt.Errorf("error building VLAN select menu: %w", err)
		}

		// Add a back button
		btn := markup.Data("⬅ Back", btnEditBackUnique, dataID).Inline()
		markup.InlineKeyboard = append(markup.InlineKeyboard, []tele.InlineButton{*btn})

		msg := fmt.Sprintf(`*📝 Edit Device 📝*

		Please select the new VLAN for *%s*`, data.Username)

		if err := c.Edit(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the block button
	bot.Handle(&tele.InlineButton{Unique: btnEditBlockUnique}, func(c tele.Context) error {
		dataID := c.Data()

		data, ok := editDeviceCache.Get(dataID)
		if !ok {
			return ErrFailedToReadData
		}

		if err := db.BlockUser(data.Username); err != nil {
			return fmt.Errorf("error blocking user: %w", err)
		}

		msg, markup, err := buildEditMessage(data.Username)
		if err != nil {
			return fmt.Errorf("error building edit message: %w", err)
		}

		if err := c.Edit(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the unblock button
	bot.Handle(&tele.InlineButton{Unique: btnEditUnblockUnique}, func(c tele.Context) error {
		dataID := c.Data()

		data, ok := editDeviceCache.Get(dataID)
		if !ok {
			return ErrFailedToReadData
		}

		if err := db.UnblockUser(data.Username); err != nil {
			return fmt.Errorf("error unblocking user: %w", err)
		}

		msg, markup, err := buildEditMessage(data.Username)
		if err != nil {
			return fmt.Errorf("error building edit message: %w", err)
		}

		if err := c.Edit(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle the delete button
	bot.Handle(&tele.InlineButton{Unique: btnEditDeleteUnique}, func(c tele.Context) error {
		dataID := c.Data()

		data, ok := editDeviceCache.Get(dataID)
		if !ok {
			return ErrFailedToReadData
		}

		if err := db.DeleteUser(data.Username); err != nil {
			return fmt.Errorf("error deleting user: %w", err)
		}

		msg := fmt.Sprintf("*🗑 User Deleted 🗑*\n\n`%s` has been deleted.", data.Username)

		if err := c.Edit(msg, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle replies to the message with the device name
	*onTextHandlers = append(*onTextHandlers, func(c tele.Context) error {
		if c.Message().IsReply() { //nolint:nestif // nest if good
			reply := c.Message()
			original := c.Message().ReplyTo

			// Extract username from the original message using regex
			username := extractUsernameFromEditDeviceMessage(original.Text)
			// Abort if username is empty
			if username == "" {
				return nil
			}

			// Get the user
			user, err := db.GetUser(username)
			if err != nil {
				// If the user does not exist, check if the user is blocked
				blocked, err := db.IsUserBlocked(username)
				if err != nil || !blocked {
					return nil //nolint:nilerr // Fail silently
				}

				// Unblock and block again so the user is updated
				if err := db.UnblockUser(username); err != nil {
					return fmt.Errorf("error unblocking user: %w", err)
				}

				if err := db.BlockUser(username); err != nil {
					return fmt.Errorf("error blocking user: %w", err)
				}

				// Get the user again
				user, err = db.GetUser(username)
				if err != nil {
					return fmt.Errorf("error getting user: %w", err)
				}
			}

			// Update the user with the description
			user.Description = reply.Text
			if err := db.UpdateUser(user); err != nil {
				return fmt.Errorf("error updating user: %w", err)
			}

			// Send the success message
			msg := fmt.Sprintf(`Saved the name *%s* for the device *%s*.`,
				user.Description, user.Username)

			if err := c.Send(msg, tele.ModeMarkdown); err != nil {
				return fmt.Errorf("error sending message: %w", err)
			}

			return nil
		}

		return nil
	})

	// Handle the back button from the VLAN selection menu
	bot.Handle(&tele.InlineButton{Unique: btnEditBackUnique}, func(c tele.Context) error {
		data, ok := editDeviceCache.Get(c.Data())
		if !ok {
			return ErrFailedToReadData
		}

		// Recreate the notification message
		msg, markup, err := buildEditMessage(data.Username)
		if err != nil {
			return fmt.Errorf("error building edit message: %w", err)
		}

		// Edit the message with the notification message
		if err := c.Edit(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})

	// Handle VLAN selection
	bot.Handle(&tele.InlineButton{Unique: btnEditSelectVLANUnique}, func(c tele.Context) error {
		data, ok := editDeviceCache.Get(c.Data())
		if !ok {
			return ErrFailedToReadData
		}

		// Get the selected VLAN
		vlan, err := db.GetVLAN(data.VlanID)
		if err != nil {
			return fmt.Errorf("error getting VLAN: %w", err)
		}

		// Update the user with the new VLAN
		user, err := db.GetUser(data.Username)
		if err != nil {
			return fmt.Errorf("error getting user: %w", err)
		}

		user.VlanID = vlan.ID
		if err := db.UpdateUser(user); err != nil {
			return fmt.Errorf("error updating user: %w", err)
		}

		// Edit the message with the success message
		msg, markup, err := buildEditMessage(data.Username)
		if err != nil {
			return fmt.Errorf("error building edit message: %w", err)
		}

		if err := c.Edit(msg, markup, tele.ModeMarkdown); err != nil {
			return fmt.Errorf("error editing message: %w", err)
		}

		return nil
	})
}
