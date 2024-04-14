package telegram

import (
	"crypto/rand"
	"encoding/base64"
)

// createRandomID creates a random ID with the specified size.
func createRandomID(size int) string {
	// Create a byte slice to hold the random data.
	randomBytes := make([]byte, size)

	// Read random data from the crypto/rand package.
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Handle the error if reading random data fails.
		panic(err)
	}

	// Encode the random data as a base64 string.
	randomID := base64.URLEncoding.EncodeToString(randomBytes)

	return randomID
}

type VLANSelectData struct {
	// Username is the username of the user.
	Username string
	// Password is the password of the user.
	Password string
	// VlanID is the ID of the VLAN. It's empty if the user didn't select any VLAN.
	VlanID string
	// MacAddress is the MAC address of the user.
	MacAddress string
}
