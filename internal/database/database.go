package database

import "context"

type VLAN struct {
	ID               string `json:"id"                         yaml:"id"`
	Name             string `json:"name"                       yaml:"name"`
	Default          bool   `json:"default,omitempty"          yaml:"default,omitempty"`
	TunnelType       uint32 `json:"tunnelType,omitempty"       yaml:"tunnelType,omitempty"`
	TunnelMediumType uint32 `json:"tunnelMediumType,omitempty" yaml:"tunnelMediumType,omitempty"`
}

type User struct {
	Username    string `json:"username"              yaml:"username"`
	Password    string `json:"password"              yaml:"password"`
	VlanID      string `json:"vlan"                  yaml:"vlan"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type BlockedUser struct {
	Username string `json:"username" yaml:"username"`
}

// Database is the interface that wraps the basic database operations.
type Database interface {
	// GetVLANs returns all the VLANs.
	GetVLANs() ([]VLAN, error)
	// GetVLAN returns a VLAN by its ID.
	GetVLAN(id string) (VLAN, error)
	// CreateVLAN creates a new VLAN.
	CreateVLAN(v VLAN) error
	// UpdateVLAN updates a VLAN.
	UpdateVLAN(v VLAN) error
	// DeleteVLAN deletes a VLAN by its ID.
	DeleteVLAN(id string) error
	// GetDefaultVLAN returns the default VLAN.
	GetDefaultVLAN() (VLAN, error)

	// GetUsers returns all the users.
	GetUsers() ([]User, error)
	// GetUser returns a user by its username.
	GetUser(username string) (User, error)
	// GetUserByDescription returns a user by its description.
	GetUserByDescription(description string) (User, error)
	// CreateUser creates a new user.
	CreateUser(u User) error
	// UpdateUser updates a user.
	UpdateUser(u User) error
	// DeleteUser deletes a user by its username.
	DeleteUser(username string) error

	// GetBlockedUsers returns all the blocked users.
	GetBlockedUsers() ([]BlockedUser, error)
	// IsUserBlocked checks if a user is blocked by its username.
	IsUserBlocked(username string) (bool, error)
	// BlockUser blocks a user by its username.
	BlockUser(username string) error
	// UnblockUser unblocks a user by its username.
	UnblockUser(username string) error

	// Init initializes the database.
	Open(ctx context.Context) error
	// Close closes the database.
	Close(ctx context.Context) error
}
