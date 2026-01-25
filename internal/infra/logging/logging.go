package logging

import (
	"log/slog"
	"os"
)

func New(logFormat, logLevel string) *slog.Logger {
	// Setup logging
	var level slog.Level

	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "info":
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler

	switch logFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	case "text":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)

	return logger
}
