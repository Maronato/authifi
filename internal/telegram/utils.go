package telegram

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// ErrFailedToReadData is returned when the data from the message could not be read.
var ErrFailedToReadData = fmt.Errorf("failed to read data from message")

// createRandomID creates a random ID with the specified size.
func createRandomID() string {
	// Create a byte slice to hold the random data.
	randomBytes := make([]byte, RandomIDLength)

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
