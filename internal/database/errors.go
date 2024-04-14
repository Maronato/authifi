package database

import "errors"

var (
	// ErrUserNotFound is returned when a user is not found.
	ErrUserNotFound = errors.New("user not found")
	// ErrUserAlreadyExists is returned when a user already exists.
	ErrUserAlreadyExists = errors.New("user already exists")
	// ErrVLANNotFound is returned when a VLAN is not found.
	ErrVLANNotFound = errors.New("vlan not found")
	// ErrVLANAlreadyExists is returned when a VLAN already exists.
	ErrVLANAlreadyExists = errors.New("vlan already exists")
	// ErrDefaultVLANNotFound is returned when the default VLAN is not found.
	ErrDefaultVLANNotFound = errors.New("default vlan not found")
	// ErrDefaultVLANAlreadyExists is returned when the default VLAN already exists.
	ErrDefaultVLANAlreadyExists = errors.New("default vlan already exists")
	// ErrBlockedUserNotFound is returned when a blocked user is not found.
	ErrBlockedUserNotFound = errors.New("blocked user not found")
	// ErrUserAlreadyBlocked is returned when a user is already blocked.
	ErrUserAlreadyBlocked = errors.New("user already blocked")
)
