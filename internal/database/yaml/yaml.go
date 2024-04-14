package yamldatabase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/maronato/authifi/internal/database"
	memorydatabase "github.com/maronato/authifi/internal/database/memory"
	"github.com/maronato/authifi/internal/logging"
)

const (
	// ReloadTimeout is the default timeout to reload the database after a change.
	ReloadTimeout = 100 * time.Millisecond
)

// YAMLDatabase implements the Database interface using a YAML file.
type YAMLDatabase struct {
	// Path to the YAML file.
	filePath string
	// watcher is the file watcher.
	watcher *fsnotify.Watcher

	// memory is the in-memory database.
	memory *memorydatabase.MemoryDatabase
}

type yamlFile struct {
	Users        []database.User        `yaml:"users"`
	VLANs        []database.VLAN        `yaml:"vlans"`
	BlockedUsers []database.BlockedUser `yaml:"blocked"`
}

// NewYAMLDatabase creates a new YAMLDatabase.
func NewYAMLDatabase(filePath string) *YAMLDatabase {
	return &YAMLDatabase{
		filePath: filePath,
		memory:   memorydatabase.NewMemoryDatabase(),
	}
}

func (d *YAMLDatabase) load() error {
	db, err := loadFile(d.filePath)
	if err != nil {
		return fmt.Errorf("error loading database file: %w", err)
	}

	d.memory = db

	return nil
}

func (d *YAMLDatabase) save() error {
	if err := dumpFile(d.filePath, d.memory); err != nil {
		return fmt.Errorf("error saving database file: %w", err)
	}

	return nil
}

func (d *YAMLDatabase) watch(ctx context.Context) {
	l := logging.FromCtx(ctx)

	// Create the watcher
	w, err := fsnotify.NewWatcher()
	if err != nil {
		l.Error("error creating watcher: %v", err)
	}
	defer w.Close()

	d.watcher = w

	// Add the file to the watcher
	if err := d.watcher.Add(d.filePath); err != nil {
		l.Error("error watching database file: %v", err)
	}

	l.Debug("started yaml database watcher", slog.String("file", d.filePath))

	// Create a debounce timer to avoid multiple reloads
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			err := d.watcher.Close()
			if err != nil {
				l.Error("error closing watcher: %v", err)
			} else {
				l.Debug("stopped watching database file")
			}

			return
		case event, ok := <-d.watcher.Events:
			if !ok {
				l.Debug("watcher events channel closed")

				return
			}

			if event.Has(fsnotify.Write) {
				// If there's a timer running, stop it
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				// Reload the database after 100ms of inactivity
				debounceTimer = time.AfterFunc(ReloadTimeout, func() {
					debounceTimer = nil

					if err := d.load(); err != nil {
						l.Error("error loading database file: %v", err)
					} else {
						l.Info("database file reloaded")
					}
				})
			}
		case err, ok := <-d.watcher.Errors:
			if !ok {
				l.Debug("watcher errors channel closed")

				return
			}

			l.Error("error watching database file: %v", err)
		}
	}
}

// GetVLANs returns all the VLANs.
func (d *YAMLDatabase) GetVLANs() ([]database.VLAN, error) {
	vlans, err := d.memory.GetVLANs()
	if err != nil {
		return nil, fmt.Errorf("error getting VLANs from memory database: %w", err)
	}

	return vlans, nil
}

// GetVLAN returns a VLAN by its ID.
func (d *YAMLDatabase) GetVLAN(id string) (database.VLAN, error) {
	vlan, err := d.memory.GetVLAN(id)
	if err != nil {
		return database.VLAN{}, fmt.Errorf("error getting VLAN from memory database: %w", err)
	}

	return vlan, nil
}

// CreateVLAN creates a new VLAN.
func (d *YAMLDatabase) CreateVLAN(v database.VLAN) error {
	if err := d.memory.CreateVLAN(v); err != nil {
		return fmt.Errorf("error creating VLAN: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error creating VLAN: %w", err)
	}

	return nil
}

// UpdateVLAN updates a VLAN.
func (d *YAMLDatabase) UpdateVLAN(v database.VLAN) error {
	if err := d.memory.UpdateVLAN(v); err != nil {
		return fmt.Errorf("error updating VLAN: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error updating VLAN: %w", err)
	}

	return nil
}

// DeleteVLAN deletes a VLAN by its ID.
func (d *YAMLDatabase) DeleteVLAN(id string) error {
	if err := d.memory.DeleteVLAN(id); err != nil {
		return fmt.Errorf("error deleting VLAN: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error deleting VLAN: %w", err)
	}

	return nil
}

// GetUsers returns all the users.
func (d *YAMLDatabase) GetUsers() ([]database.User, error) {
	users, err := d.memory.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("error getting users from memory database: %w", err)
	}

	return users, nil
}

// GetUser returns a user by its username.
func (d *YAMLDatabase) GetUser(username string) (database.User, error) {
	user, err := d.memory.GetUser(username)
	if err != nil {
		return database.User{}, fmt.Errorf("error getting user from memory database: %w", err)
	}

	return user, nil
}

// CreateUser creates a new user.
func (d *YAMLDatabase) CreateUser(u database.User) error {
	if err := d.memory.CreateUser(u); err != nil {
		return fmt.Errorf("error creating user: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error creating user: %w", err)
	}

	return nil
}

// UpdateUser updates a user.
func (d *YAMLDatabase) UpdateUser(u database.User) error {
	if err := d.memory.UpdateUser(u); err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}

	return nil
}

// DeleteUser deletes a user by its username.
func (d *YAMLDatabase) DeleteUser(username string) error {
	if err := d.memory.DeleteUser(username); err != nil {
		return fmt.Errorf("error deleting user: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error deleting user: %w", err)
	}

	return nil
}

// GetBlockedUsers returns all the blocked users.
func (d *YAMLDatabase) GetBlockedUsers() ([]database.BlockedUser, error) {
	blockedUsers, err := d.memory.GetBlockedUsers()
	if err != nil {
		return nil, fmt.Errorf("error getting blocked users from memory database: %w", err)
	}

	return blockedUsers, nil
}

// IsUserBlocked checks if a user is blocked by its username.
func (d *YAMLDatabase) IsUserBlocked(username string) (bool, error) {
	blocked, err := d.memory.IsUserBlocked(username)
	if err != nil {
		return false, fmt.Errorf("error checking if user is blocked: %w", err)
	}

	return blocked, nil
}

// BlockUser blocks a user by its username.
func (d *YAMLDatabase) BlockUser(username string) error {
	if err := d.memory.BlockUser(username); err != nil {
		return fmt.Errorf("error blocking user: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error blocking user: %w", err)
	}

	return nil
}

// UnblockUser unblocks a user by its username.
func (d *YAMLDatabase) UnblockUser(username string) error {
	if err := d.memory.UnblockUser(username); err != nil {
		return fmt.Errorf("error unblocking user: %w", err)
	}

	if err := d.save(); err != nil {
		return fmt.Errorf("error unblocking user: %w", err)
	}

	return nil
}

// Open initializes the database.
func (d *YAMLDatabase) Open(ctx context.Context) error {
	l := logging.FromCtx(ctx)

	if err := d.load(); err != nil {
		return fmt.Errorf("error initializing database: %w", err)
	}

	// Start the watcher
	go d.watch(ctx)

	l.Debug("opened yaml database", slog.String("file", d.filePath))

	return nil
}

// Close closes the database.
func (d *YAMLDatabase) Close(ctx context.Context) error {
	l := logging.FromCtx(ctx)

	// Make sure the database is up to date before closing
	if err := d.save(); err != nil {
		return fmt.Errorf("error closing database: %w", err)
	}

	// Close the watcher
	if err := d.watcher.Close(); err != nil {
		return fmt.Errorf("error closing watcher: %w", err)
	}

	l.Debug("yaml database closed", slog.String("file", d.filePath))

	return nil
}

// GetDefaultVLAN returns the default VLAN.
func (d *YAMLDatabase) GetDefaultVLAN() (database.VLAN, error) {
	vlan, err := d.memory.GetDefaultVLAN()
	if err != nil {
		return database.VLAN{}, fmt.Errorf("error getting default VLAN from memory database: %w", err)
	}

	return vlan, nil
}
