package logger

import (
	"log/slog"
	"os"
)

var enabled bool

// Init configures the default slog logger based on the debug flag.
// When debug is true the log level is set to Debug; otherwise it is set
// to Warn so that only warnings and errors are emitted.
// Output is written to stderr to keep stdout clean for program output.
func Init(debug bool) {
	enabled = debug
	level := slog.LevelWarn
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

// Enabled returns true when debug logging is active.
func Enabled() bool {
	return enabled
}

// Debug logs a message at debug level. Additional key-value pairs can be
// passed as structured context (e.g. logger.Debug("loaded config", "path", p)).
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}
