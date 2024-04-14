package yamldatabase

import (
	"fmt"
	"os"
	"path"

	memorydatabase "github.com/maronato/authifi/internal/database/memory"
	"gopkg.in/yaml.v3"
)

var ErrRelativeFile = fmt.Errorf("database file path must be absolute")

// loadFile loads the YAML file into the in-memory database.
func loadFile(filePath string) (*memorydatabase.MemoryDatabase, error) {
	// If filePath is a relative path, return an error
	if !path.IsAbs(filePath) {
		return nil, fmt.Errorf("bad database file path (%s): %w", filePath, ErrRelativeFile)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer f.Close()

	var yf yamlFile
	if err := yaml.NewDecoder(f).Decode(&yf); err != nil {
		return nil, fmt.Errorf("error decoding file: %w", err)
	}

	db := memorydatabase.NewMemoryDatabase()

	for _, v := range yf.VLANs {
		if err := db.CreateVLAN(v); err != nil {
			return nil, fmt.Errorf("error creating VLAN: %w", err)
		}
	}

	for _, u := range yf.Users {
		if err := db.CreateUser(u); err != nil {
			return nil, fmt.Errorf("error creating user: %w", err)
		}
	}

	for _, bu := range yf.BlockedUsers {
		if err := db.BlockUser(bu.Username); err != nil {
			return nil, fmt.Errorf("error blocking user: %w", err)
		}
	}

	return db, nil
}

// dumpFile dumps the in-memory database into the YAML file.
func dumpFile(filePath string, db *memorydatabase.MemoryDatabase) error {
	// If filePath is a relative path, return an error
	if !path.IsAbs(filePath) {
		return fmt.Errorf("bad database file path (%s): %w", filePath, ErrRelativeFile)
	}

	// Create or overwrite the file
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer f.Close()

	// Create the YAML file
	users, err := db.GetUsers()
	if err != nil {
		return fmt.Errorf("error getting users: %w", err)
	}

	vlans, err := db.GetVLANs()
	if err != nil {
		return fmt.Errorf("error getting VLANs: %w", err)
	}

	blockedUsers, err := db.GetBlockedUsers()
	if err != nil {
		return fmt.Errorf("error getting blocked users: %w", err)
	}

	yf := yamlFile{
		Users:        users,
		VLANs:        vlans,
		BlockedUsers: blockedUsers,
	}

	// Encode the YAML file
	if err := yaml.NewEncoder(f).Encode(yf); err != nil {
		return fmt.Errorf("error encoding file: %w", err)
	}

	return nil
}
