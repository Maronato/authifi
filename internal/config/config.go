package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

type VerboseLevel int

const (
	VerboseLevelQuiet VerboseLevel = iota - 1
	VerboseLevelInfo
	VerboseLevelAccessLogs
	VerboseLevelDebug
)

const (
	// DefaultProd is the default mode.
	DefaultProd = false
	// DefaultHost is the default host to listen on.
	DefaultHost = "localhost"
	// DefaultPort is the default port to listen on.
	DefaultPort = "1812"
	// DefaultDatabaseFilePath is the default file path to the database definition file.
	DefaultDatabaseFilePath = "database.yaml"
	// DefaultVerbose is the default verbosity level.
	DefaultVerbose = VerboseLevelInfo
	// DefaultQuiet is the default quiet mode.
	DefaultQuiet = false
)

// ErrInvalidConfig is returned when the config is invalid.
var ErrInvalidConfig = errors.New("invalid config")

type Config struct {
	// Prod is a flag that indicates if the server is running in production mode.
	Prod bool
	// Debug defines whether or not the server should run in debug mode.
	Debug bool
	// Host is the host to listen on.
	Host string
	// Port is the port to listen on.
	Port string
	// DatabaseFilePath is the path to the database definition file.
	DatabaseFilePath string
	// RadiusSecret is the secret used to authenticate RADIUS requests.
	RadiusSecret string
	// Verbose defines the verbosity level.
	Verbose VerboseLevel `json:"verbose"`
	// Quiet defines whether or not the server should be quiet.
	Quiet bool
	// TelegramBotToken is the token used to authenticate with the Telegram bot API.
	TelegramBotToken string
	// TelegramChatIDs is a list of chat IDs that are allowed to interact with the bot.
	TelegramChatIDs []string
}

func NewConfig() *Config {
	return &Config{
		Prod:             DefaultProd,
		Host:             DefaultHost,
		Port:             DefaultPort,
		DatabaseFilePath: DefaultDatabaseFilePath,
		Verbose:          DefaultVerbose,
		Quiet:            DefaultQuiet,
	}
}

func (c *Config) GetAddr() string {
	return net.JoinHostPort(c.Host, c.Port)
}

func (c *Config) Validate() error {
	// Host and port have to be valid.
	if _, err := url.ParseRequestURI("http://" + net.JoinHostPort(c.Host, c.Port)); err != nil {
		return fmt.Errorf("invalid host and/or port: %w", ErrInvalidConfig)
	}

	// Verbose has to be valid.
	if c.Verbose < VerboseLevelInfo || c.Verbose > VerboseLevelDebug {
		return fmt.Errorf("invalid verbosity level (%d): %w", c.Verbose, ErrInvalidConfig)
	}

	if c.Verbose != VerboseLevelInfo && c.Quiet {
		return fmt.Errorf("cannot set both verbose and quiet: %w", ErrInvalidConfig)
	}

	// If verbose == 2, set debug to true for backwards compatibility.
	if c.Verbose >= VerboseLevelDebug {
		c.Debug = true
	}

	// If quiet is true, set verbose to -1.
	if c.Quiet {
		c.Verbose = VerboseLevelQuiet
	}

	if c.DatabaseFilePath == "" {
		return fmt.Errorf("%w: database file path is empty", ErrInvalidConfig)
	}

	if c.RadiusSecret == "" {
		return fmt.Errorf("%w: RADIUS secret is empty", ErrInvalidConfig)
	}

	// Make sure all chat IDs are integers.
	for _, chatID := range c.TelegramChatIDs {
		if _, err := strconv.Atoi(chatID); err != nil {
			return fmt.Errorf("%w: invalid chat ID: %s", ErrInvalidConfig, chatID)
		}
	}

	return nil
}
