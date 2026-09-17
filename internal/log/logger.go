package log

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates and returns a configured slog.Logger.
// level can be "debug", "info", "warn", "error".
// output determines handler type; empty string uses stderr.
func NewLogger(level, output string) *slog.Logger {
	var handler slog.Handler
	var logLevel slog.Level

	// Parse log level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Determine output destination
	var w io.Writer
	switch output {
	case "stdout":
		w = os.Stdout
	case "stderr", "":
		w = os.Stderr
	default:
		// Try to open file
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			// Fallback to stderr on error
			w = os.Stderr
		} else {
			w = f
		}
	}

	// Use JSON handler for machine-readable logs
	handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: logLevel,
	})

	return slog.New(handler)
}

// NewTextLogger creates a text-formatted logger (useful for CLI).
func NewTextLogger(level string) *slog.Logger {
	var logLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})

	return slog.New(handler)
}
