package logger

import (
	"log/slog"
	"os"
)

var enabled bool
var verbosityLevel int

// Init configures structured logging. Writes to stderr.
//   - default (no flags): all output suppressed
//   - -v: shows Verbose (Info) messages
//   - --debug: shows both Verbose and Debug messages (superset of -v)
func Init(debug bool, verbosity int) {
	verbosityLevel = verbosity
	if debug && verbosityLevel < 1 {
		verbosityLevel = 1
	}
	enabled = debug || verbosityLevel > 0
	level := slog.Level(100) // above Error — silences all output
	if debug {
		level = slog.LevelDebug // Verbose + Debug
	} else if verbosityLevel >= 1 {
		level = slog.LevelInfo // Verbose only
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func Enabled() bool {
	return enabled
}

// Verbosity returns the current verbosity level (0 = off, 1 = verbose+).
func Verbosity() int {
	return verbosityLevel
}

// Verbose logs at info level — visible with -v and --debug.
func Verbose(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Debug logs at debug level — visible only with --debug.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Warn logs at warn level with optional key-value pairs (e.g. logger.Warn("retrying", "attempt", n)).
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}
