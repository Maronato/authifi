package memorydatabase

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/maronato/authifi/internal/database"
)

// MemoryDatabase implements the Database interface using an in-memory map.
type MemoryDatabase struct {
	// users is a map of usernames to users.
	users map[string]*database.User
	// vlans is a map of VLAN IDs to VLANs.
	vlans map[string]*database.VLAN
	// blockedUsers is a map of usernames to blocked users.
	blockedUsers map[string]*database.BlockedUser
	// defaultVLAN is the default VLAN.
	defaultVLAN *database.VLAN
}

// NewMemoryDatabase creates a new MemoryDatabase.
func NewMemoryDatabase() *MemoryDatabase {
	return &MemoryDatabase{
		users:        make(map[string]*database.User),
		vlans:        make(map[string]*database.VLAN),
		blockedUsers: make(map[string]*database.BlockedUser),
		defaultVLAN:  nil,
	}
}

// GetVLANs returns all the VLANs.
func (d *MemoryDatabase) GetVLANs() ([]database.VLAN, error) {
	vlans := make([]database.VLAN, 0, len(d.vlans))
	for _, vlan := range d.vlans {
		vlans = append(vlans, *vlan)
	}

	// Sort VLANs by their ID
	slices.SortFunc(vlans, func(a, b database.VLAN) int {
		// Convert IDs to integers for comparison
		idA, _ := strconv.Atoi(a.ID)
		idB, _ := strconv.Atoi(b.ID)

		return cmp.Compare(idA, idB)
	})

	return vlans, nil
}

// GetVLAN returns a VLAN by its ID.
func (d *MemoryDatabase) GetVLAN(id string) (database.VLAN, error) {
	vlan, ok := d.vlans[id]
	if !ok {
		return database.VLAN{}, fmt.Errorf("error getting VLAN %s: %w", id, database.ErrVLANNotFound)
	}

	return *vlan, nil
}

// CreateVLAN creates a new VLAN.
func (d *MemoryDatabase) CreateVLAN(v database.VLAN) error {
	if _, ok := d.vlans[v.ID]; ok {
		return fmt.Errorf("error creating VLAN %s: %w", v.ID, database.ErrVLANAlreadyExists)
	}

	// If the VLAN is the default VLAN, set it
	if v.Default {
		if d.defaultVLAN != nil {
			return fmt.Errorf("error creating VLAN %s: %w", v.ID, database.ErrDefaultVLANAlreadyExists)
		}

		d.defaultVLAN = &v
	}

	d.vlans[v.ID] = &v

	return nil
}

// UpdateVLAN updates a VLAN.
func (d *MemoryDatabase) UpdateVLAN(v database.VLAN) error {
	if _, ok := d.vlans[v.ID]; !ok {
		return fmt.Errorf("error updating VLAN %s: %w", v.ID, database.ErrVLANNotFound)
	}

	d.vlans[v.ID] = &v

	return nil
}

// DeleteVLAN deletes a VLAN by its ID.
func (d *MemoryDatabase) DeleteVLAN(id string) error {
	if _, ok := d.vlans[id]; !ok {
		return fmt.Errorf("error deleting VLAN %s: %w", id, database.ErrVLANNotFound)
	}

	delete(d.vlans, id)

	return nil
}

// GetDefaultVLAN returns the default VLAN.
func (d *MemoryDatabase) GetDefaultVLAN() (database.VLAN, error) {
	if d.defaultVLAN == nil {
		return database.VLAN{}, fmt.Errorf("error getting default VLAN: %w", database.ErrDefaultVLANNotFound)
	}

	return *d.defaultVLAN, nil
}

// GetUsers returns all the users.
func (d *MemoryDatabase) GetUsers() ([]database.User, error) {
	users := make([]database.User, 0, len(d.users))
	for _, user := range d.users {
		users = append(users, *user)
	}

	// Sort users by their username
	slices.SortFunc(users, func(a, b database.User) int {
		return cmp.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username))
	})

	return users, nil
}

// GetUser returns a user by its username.
func (d *MemoryDatabase) GetUser(username string) (database.User, error) {
	user, ok := d.users[username]
	if !ok {
		return database.User{}, fmt.Errorf("error getting user %s: %w", username, database.ErrUserNotFound)
	}

	return *user, nil
}

// GetUserByDescription returns a user by its description.
func (d *MemoryDatabase) GetUserByDescription(description string) (database.User, error) {
	for _, user := range d.users {
		if user.Description == description {
			return *user, nil
		}
	}

	return database.User{}, fmt.Errorf("error getting user by description %s: %w", description, database.ErrUserNotFound)
}

// CreateUser creates a new user.
func (d *MemoryDatabase) CreateUser(u database.User) error {
	if _, ok := d.users[u.Username]; ok {
		return fmt.Errorf("error creating user %s: %w", u.Username, database.ErrUserAlreadyExists)
	}

	// Validate the VLAN
	if _, err := d.GetVLAN(u.VlanID); err != nil {
		return fmt.Errorf("error creating user: %w", err)
	}

	d.users[u.Username] = &u

	return nil
}

// UpdateUser updates a user.
func (d *MemoryDatabase) UpdateUser(u database.User) error {
	if _, ok := d.users[u.Username]; !ok {
		return fmt.Errorf("error updating user %s: %w", u.Username, database.ErrUserNotFound)
	}

	d.users[u.Username] = &u

	return nil
}

// DeleteUser deletes a user by its username.
func (d *MemoryDatabase) DeleteUser(username string) error {
	if _, ok := d.users[username]; !ok {
		return fmt.Errorf("error deleting user %s: %w", username, database.ErrUserNotFound)
	}

	delete(d.users, username)

	// Also delete the user from the blocked users
	delete(d.blockedUsers, username)

	return nil
}

// GetBlockedUsers returns all the blocked users.
func (d *MemoryDatabase) GetBlockedUsers() ([]database.BlockedUser, error) {
	blockedUsers := make([]database.BlockedUser, 0, len(d.blockedUsers))
	for _, blockedUser := range d.blockedUsers {
		blockedUsers = append(blockedUsers, *blockedUser)
	}

	// Sort blocked users by their username
	slices.SortFunc(blockedUsers, func(a, b database.BlockedUser) int {
		return cmp.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username))
	})

	return blockedUsers, nil
}

// BlockUser blocks a user by its username.
func (d *MemoryDatabase) BlockUser(username string) error {
	if _, ok := d.blockedUsers[username]; ok {
		return fmt.Errorf("error blocking user %s: %w", username, database.ErrUserAlreadyBlocked)
	}

	d.blockedUsers[username] = &database.BlockedUser{Username: username}

	return nil
}

// UnblockUser unblocks a user by its username.
func (d *MemoryDatabase) UnblockUser(username string) error {
	if _, ok := d.blockedUsers[username]; !ok {
		return fmt.Errorf("error unblocking user %s: %w", username, database.ErrBlockedUserNotFound)
	}

	delete(d.blockedUsers, username)

	// Create a new user if it does not exist and assign it to the default or first VLAN
	if _, ok := d.users[username]; !ok { //nolint:nestif // Nested if statements are used for clarity
		// Get default VLAN
		defaultVLAN, err := d.GetDefaultVLAN()
		if err != nil {
			// Get first VLAN if the default VLAN does not exist
			vlans, err := d.GetVLANs()
			if err != nil {
				return fmt.Errorf("error unblocking user: %w", err)
			}

			if len(vlans) == 0 {
				return fmt.Errorf("error unblocking user: %w", database.ErrVLANNotFound)
			}

			defaultVLAN = vlans[0]
		}

		d.users[username] = &database.User{Username: username, VlanID: defaultVLAN.ID, Password: username}
	}

	return nil
}

// IsUserBlocked checks if a user is blocked by its username.
func (d *MemoryDatabase) IsUserBlocked(username string) (bool, error) {
	_, ok := d.blockedUsers[username]

	return ok, nil
}

// Open initializes the database.
func (d *MemoryDatabase) Open(_ context.Context) error {
	return nil
}

// Close closes the database.
func (d *MemoryDatabase) Close(_ context.Context) error {
	return nil
}
