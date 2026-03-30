package logger

import (
	"log/slog"
	"os"
)

var enabled bool

// Init configures structured logging. Writes to stderr; all output is suppressed unless debug=true.
func Init(debug bool) {
	enabled = debug
	level := slog.Level(100) // above Error — silences all output
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func Enabled() bool {
	return enabled
}

// Debug logs at debug level with optional key-value pairs (e.g. logger.Debug("loaded", "path", p)).
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}
