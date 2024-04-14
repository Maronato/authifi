package cmd

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/maronato/authifi/internal/config"
	yamldatabase "github.com/maronato/authifi/internal/database/yaml"
	"github.com/maronato/authifi/internal/logging"
	"github.com/maronato/authifi/internal/radiusserver"
	"github.com/maronato/authifi/internal/telegram"
	"github.com/peterbourgon/ff/v4"
	"golang.org/x/sync/errgroup"
)

func newServerCmd(cfg *config.Config) *ff.Command {
	return &ff.Command{
		Name:      "serve",
		Usage:     "serve [flags]",
		ShortHelp: "Start the authifi server",
		Exec: func(ctx context.Context, _ []string) error {
			// Validate config
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("error validating config: %w", err)
			}

			// Create a logger and add it to the context
			l := logging.NewLogger(os.Stderr, cfg)
			ctx = logging.WithLogger(ctx, l)

			// If the database file path is relative, make it absolute
			dbFilePath := cfg.DatabaseFilePath
			if !path.IsAbs(dbFilePath) {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("error getting working directory: %w", err)
				}

				dbFilePath = path.Join(wd, dbFilePath)
			}

			// Initialize the database
			db := yamldatabase.NewYAMLDatabase(dbFilePath)
			if err := db.Open(ctx); err != nil {
				return fmt.Errorf("error initializing database: %w", err)
			}
			defer db.Close(ctx)

			botServer, err := telegram.NewBotServer(ctx, cfg, db)
			if err != nil {
				return fmt.Errorf("error creating bot server: %w", err)
			}

			// Create an errgroup to run the server
			eg, egCtx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				if err := radiusserver.StartServer(egCtx, cfg, db, botServer); err != nil {
					return fmt.Errorf("server error: %w", err)
				}

				return nil
			})

			eg.Go(func() error {
				if err := botServer.StartBot(egCtx); err != nil {
					return fmt.Errorf("bot error: %w", err)
				}

				return nil
			})

			// Wait for the server to exit and check for errors that
			// are not caused by the context being canceled.
			if err := eg.Wait(); err != nil && ctx.Err() == nil {
				return fmt.Errorf("server exited with error: %w", err)
			}

			return nil
		},
	}
}
