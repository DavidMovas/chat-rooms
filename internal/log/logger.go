package log

import (
	"context"
	"log/slog"
	"os"
)

type contextKey struct{}

var contextLoggerKey = contextKey{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextLoggerKey, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextLoggerKey).(*slog.Logger); ok {
		return logger
	}

	return slog.Default()
}

func SetupLogger(isLocal bool, level string) (*slog.Logger, error) {
	l := newLevelFromString(level)
	opts := &slog.HandlerOptions{Level: l}

	var handler slog.Handler
	if isLocal {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler), nil
}

func newLevelFromString(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
