package logger

import (
	"log/slog"
	"os"
)

var enabled bool
var verbosityLevel int

// Init configures structured logging. Writes to stderr; all output is suppressed unless debug=true.
// verbosity controls output detail: 1 = summary, 2 = full. When debug is true, verbosity is promoted to at least 2.
func Init(debug bool, verbosity int) {
	verbosityLevel = verbosity
	if debug && verbosityLevel < 2 {
		verbosityLevel = 2
	}
	enabled = debug || verbosityLevel > 0
	level := slog.Level(100) // above Error — silences all output
	if enabled {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func Enabled() bool {
	return enabled
}

// Verbosity returns the current verbosity level (0 = off, 1 = summary, 2 = full).
func Verbosity() int {
	return verbosityLevel
}

// Debug logs at debug level with optional key-value pairs (e.g. logger.Debug("loaded", "path", p)).
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}
