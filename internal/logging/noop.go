package logging

import (
	"context"
	"log/slog"
)

type NoOpHandler struct{}

func NewNoOpHandler() *NoOpHandler {
	return &NoOpHandler{}
}

func (h *NoOpHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

func (h *NoOpHandler) Handle(_ context.Context, _ slog.Record) error { //nolint:gocritic // This is the signature of the interface.
	return nil
}

func (h *NoOpHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return NewNoOpHandler()
}

func (h *NoOpHandler) WithGroup(_ string) slog.Handler {
	return NewNoOpHandler()
}
