package logging

import (
	"log"
	"log/slog"
)

// Define the SlogAdapter struct that wraps the slog.Logger.
type SlogAdapter struct {
	logger *slog.Logger
}

// Write implements the io.Writer interface required by log.Logger
// This method will be called whenever log.Logger's Print functions are called.
func (adapter SlogAdapter) Write(p []byte) (n int, err error) {
	// Convert the byte slice to a string and log it using slog.Logger's Info method
	adapter.logger.Info(string(p))

	return len(p), nil
}

// AsStdLogger creates a new log.Logger that uses the SlogAdapter to log messages.
func AsStdLogger(slogger *slog.Logger) *log.Logger {
	// Create an instance of SlogAdapter with the provided slog.Logger
	adapter := SlogAdapter{logger: slogger}

	// Create a new log.Logger using the adapter
	// You can configure the prefix and flags according to your requirements.
	return log.New(adapter, "INFO: ", log.LstdFlags)
}
