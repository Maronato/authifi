package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/maronato/authifi/internal/config"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

const appName = "authifi"

func Run(version string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Allow graceful shutdown
	trapSignalsCrossPlatform(cancel)

	cfg := &config.Config{}

	// Set production mode if not dev
	if version != "" && version != "dev" && version != "undefined" {
		cfg.Prod = true
	}

	// Create a new root command
	subcommands := []*ff.Command{
		newServerCmd(cfg),
		{
			Name:      "version",
			Usage:     "version",
			ShortHelp: "Prints the version",
			Exec: func(_ context.Context, _ []string) error {
				fmt.Println(version) //nolint:forbidigo // We want to print to stdout

				return nil
			},
		},
	}
	cmd := newRootCmd(version, cfg, subcommands)

	// Parse and run
	if err := cmd.ParseAndRun(ctx, os.Args[1:],
		ff.WithEnvVarPrefix("AI"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		if errors.Is(err, ff.ErrHelp) || errors.Is(err, ff.ErrNoExec) {
			fmt.Fprintf(os.Stderr, "\n%s\n", ffhelp.Command(cmd))

			return nil
		}

		return fmt.Errorf("error running command: %w", err)
	}

	return nil
}

// https://github.com/caddyserver/caddy/blob/fbb0ecfa322aa7710a3448453fd3ae40f037b8d1/sigtrap.go#L37
// trapSignalsCrossPlatform captures SIGINT or interrupt (depending
// on the OS), which initiates a graceful shutdown. A second SIGINT
// or interrupt will forcefully exit the process immediately.
func trapSignalsCrossPlatform(cancel context.CancelFunc) {
	go func() {
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, os.Interrupt, syscall.SIGINT)

		for i := 0; true; i++ {
			<-shutdown

			if i > 0 {
				fmt.Printf("\nForce quit\n") //nolint:forbidigo // We want to print to stdout
				os.Exit(1)
			}

			fmt.Printf("\nGracefully shutting down. Press Ctrl+C again to force quit\n") //nolint:forbidigo // We want to print to stdout
			cancel()
		}
	}()
}

// NewRootCmd parses the command line flags and returns a config.Config struct.
func newRootCmd(version string, cfg *config.Config, subcommands []*ff.Command) *ff.Command {
	fs := ff.NewFlagSet(appName)

	for _, cmd := range subcommands {
		cmd.Flags = ff.NewFlagSet(cmd.Name).SetParent(fs)
	}

	cmd := &ff.Command{
		Name:        appName,
		Usage:       fmt.Sprintf("%s <command> [flags]", appName),
		ShortHelp:   fmt.Sprintf("(%s) An authifi server", version),
		Flags:       fs,
		Subcommands: subcommands,
	}

	fs.BoolVar(&cfg.Quiet, 'q', "quiet", "enable quiet mode")
	// Verbose flag that accepts multiple values
	fs.IntVar((*int)(&cfg.Verbose), 'v', "verbose", int(config.DefaultVerbose), "set verbosity level")
	fs.StringVar(&cfg.Host, 'h', "host", config.DefaultHost, "Host to listen on")
	fs.StringVar(&cfg.Port, 'p', "port", config.DefaultPort, "Port to listen on")
	fs.StringVar(&cfg.DatabaseFilePath, 'f', "database-file", config.DefaultDatabaseFilePath, "Path to the database file")
	fs.StringVar(&cfg.RadiusSecret, 's', "radius-secret", "", "RADIUS secret")
	fs.StringVar(&cfg.TelegramBotToken, 't', "telegram-token", "", "Telegram bot token")
	fs.StringListVar(&cfg.TelegramChatIDs, 'i', "telegram-chat-ids", "Telegram chat IDs")
	// Optional config flag
	fs.String('c', "config", "", "config file")

	return cmd
}
