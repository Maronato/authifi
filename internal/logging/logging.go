package logging

import (
	"context"
	"io"
	"log/slog"

	"github.com/maronato/authifi/internal/config"
)

type logCtxKey struct{}

func NewLogger(w io.Writer, cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	addSource := false

	if cfg.Verbose >= config.VerboseLevelDebug {
		level = slog.LevelDebug
		addSource = true
	}

	if cfg.Verbose <= config.VerboseLevelQuiet {
		level = slog.LevelError
	}

	if cfg.Prod {
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level:     level,
			AddSource: addSource,
		}))
	}

	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	}))
}

func FromCtx(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(logCtxKey{}).(*slog.Logger)
	if !ok {
		return slog.New(NewNoOpHandler())
	}

	return l
}

func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, logCtxKey{}, l)
}
