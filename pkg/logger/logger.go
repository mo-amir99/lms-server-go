package logger

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// New creates a structured slog.Logger based on the provided level string.
// Logs to files in logs/ directory and only shows important messages to console
func New(level string) (*slog.Logger, error) {
	handlerLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}

	// Open log files
	errorFile, err := os.OpenFile(filepath.Join("logs", "error.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	infoFile, err := os.OpenFile(filepath.Join("logs", "info.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	// Create handlers:
	// - Console: text format for readability
	// - Files: JSON format for parsing
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: handlerLevel})
	infoFileHandler := slog.NewJSONHandler(infoFile, &slog.HandlerOptions{Level: handlerLevel})
	errorFileHandler := slog.NewJSONHandler(errorFile, &slog.HandlerOptions{Level: slog.LevelError})

	// Create a custom handler that routes logs to console and files
	handler := NewMultiLevelHandler(consoleHandler, infoFileHandler, errorFileHandler)
	return slog.New(handler), nil
}

// MultiLevelHandler routes logs to multiple handlers (console + files)
type MultiLevelHandler struct {
	consoleHandler   slog.Handler
	infoFileHandler  slog.Handler
	errorFileHandler slog.Handler
	level            slog.Leveler
}

func NewMultiLevelHandler(consoleHandler, infoFileHandler, errorFileHandler slog.Handler) *MultiLevelHandler {
	return &MultiLevelHandler{
		consoleHandler:   consoleHandler,
		infoFileHandler:  infoFileHandler,
		errorFileHandler: errorFileHandler,
		level:            slog.LevelInfo,
	}
}

func (h *MultiLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *MultiLevelHandler) Handle(ctx context.Context, r slog.Record) error {
	// Always write to console
	if err := h.consoleHandler.Handle(ctx, r); err != nil {
		return err
	}

	// Write to info file
	if err := h.infoFileHandler.Handle(ctx, r); err != nil {
		return err
	}

	// Also write errors to error file
	if r.Level >= slog.LevelError {
		return h.errorFileHandler.Handle(ctx, r)
	}

	return nil
}

func (h *MultiLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MultiLevelHandler{
		consoleHandler:   h.consoleHandler.WithAttrs(attrs),
		infoFileHandler:  h.infoFileHandler.WithAttrs(attrs),
		errorFileHandler: h.errorFileHandler.WithAttrs(attrs),
		level:            h.level,
	}
}

func (h *MultiLevelHandler) WithGroup(name string) slog.Handler {
	return &MultiLevelHandler{
		consoleHandler:   h.consoleHandler.WithGroup(name),
		infoFileHandler:  h.infoFileHandler.WithGroup(name),
		errorFileHandler: h.errorFileHandler.WithGroup(name),
		level:            h.level,
	}
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
