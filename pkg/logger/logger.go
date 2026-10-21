package logger

import (
	"io"
	"log/slog"
	"os"
)

// Output receives the log lines. The display package replaces it so log lines
// written while a step spinner is animating clear the spinner line first.
var Output io.Writer = os.Stderr

// liveWriter resolves Output at write time, so replacing it takes effect for
// handlers that were already created by Init.
type liveWriter struct{}

func (liveWriter) Write(p []byte) (int, error) { return Output.Write(p) }

var enabled bool
var debugMode bool
var verbosityLevel int

// Init configures structured logging. Writes to stderr.
//   - default (no flags): all output suppressed
//   - -v: shows Verbose (Info) messages
//   - --debug: shows both Verbose and Debug messages (superset of -v)
func Init(debug bool, verbosity int) {
	verbosityLevel = verbosity
	debugMode = debug
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
	handler := slog.NewTextHandler(liveWriter{}, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func Enabled() bool {
	return enabled
}

// IsDebug returns true when --debug was passed.
func IsDebug() bool {
	return debugMode
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
