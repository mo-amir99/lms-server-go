package logger

import (
	"errors"
	"log/slog"
	"os"
	"strings"
)

// New creates a structured slog.Logger based on the provided level string.
func New(level string) (*slog.Logger, error) {
	handlerLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: handlerLevel})
	return slog.New(handler), nil
}

func parseLevel(level string) (slog.Leveler, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return nil, errors.New("invalid log level")
	}
}
