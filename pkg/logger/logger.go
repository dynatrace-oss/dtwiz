package logger

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var enabled bool
var verbosityLevel int

// Init configures structured logging. Writes to stderr; all output is suppressed unless debug=true.
// verbosity controls HTTP tracing: 1 = compact request/response summary, 2 = full headers and bodies.
// When debug is true, verbosity is promoted to at least 2.
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

// sensitiveHeaders lists headers that should always be redacted in debug output.
var sensitiveHeaders = []string{"authorization", "x-api-key", "cookie", "set-cookie"}

func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)
	for _, h := range sensitiveHeaders {
		if lower == h {
			return true
		}
	}
	return false
}

// LoggingTransport is an http.RoundTripper that logs requests and responses
// when verbose/debug mode is active.
//
// Level 1: compact one-liner per request — "METHOD URL → STATUS (time)"
// Level 2: full request/response with headers and bodies; sensitive headers are
// always shown as [REDACTED].
type LoggingTransport struct {
	Base http.RoundTripper
}

// NewLoggingTransport wraps base (falls back to http.DefaultTransport when nil)
// in a LoggingTransport.
func NewLoggingTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &LoggingTransport{Base: base}
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if verbosityLevel <= 0 {
		return t.Base.RoundTrip(req)
	}

	if verbosityLevel >= 2 {
		var sb strings.Builder
		sb.WriteString("===> REQUEST <===\n")
		sb.WriteString(fmt.Sprintf("%s %s\n", req.Method, req.URL))
		sb.WriteString("HEADERS:\n")
		for k, v := range req.Header {
			if isSensitiveHeader(k) {
				sb.WriteString(fmt.Sprintf("    %s: [REDACTED]\n", k))
			} else {
				sb.WriteString(fmt.Sprintf("    %s: %s\n", k, strings.Join(v, ", ")))
			}
		}
		if bodyText := readRequestBodyForDebug(req); bodyText != "" {
			sb.WriteString(fmt.Sprintf("BODY:\n%s\n", bodyText))
		}
		fmt.Fprint(os.Stderr, sb.String())
	}

	start := time.Now()
	resp, err := t.Base.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		if verbosityLevel >= 1 {
			fmt.Fprintf(os.Stderr, "%s %s → error: %v\n", req.Method, req.URL, err)
		}
		return resp, err
	}

	if verbosityLevel == 1 {
		fmt.Fprintf(os.Stderr, "%s %s → %s (%s)\n", req.Method, req.URL, resp.Status, elapsed.Round(time.Millisecond))
	} else {
		var sb strings.Builder
		sb.WriteString("===> RESPONSE <===\n")
		sb.WriteString(fmt.Sprintf("STATUS: %d %s\n", resp.StatusCode, resp.Status))
		sb.WriteString(fmt.Sprintf("TIME: %s\n", elapsed.Round(time.Millisecond)))
		sb.WriteString("HEADERS:\n")
		for k, v := range resp.Header {
			if isSensitiveHeader(k) {
				sb.WriteString(fmt.Sprintf("    %s: [REDACTED]\n", k))
			} else {
				sb.WriteString(fmt.Sprintf("    %s: %s\n", k, strings.Join(v, ", ")))
			}
		}
		if resp.Body != nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil {
				sb.WriteString(fmt.Sprintf("BODY:\n%s\n", string(body)))
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}
		fmt.Fprint(os.Stderr, sb.String())
	}

	return resp, nil
}

// readRequestBodyForDebug reads the request body without consuming it.
// It uses GetBody (the clone function) when available; otherwise it reads and
// restores req.Body directly.
func readRequestBodyForDebug(req *http.Request) string {
	if req.GetBody != nil {
		clone, err := req.GetBody()
		if err != nil || clone == nil {
			return ""
		}
		defer clone.Close()
		body, err := io.ReadAll(clone)
		if err != nil || len(body) == 0 {
			return ""
		}
		return string(body)
	}

	if req.Body == nil {
		return ""
	}
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil || len(body) == 0 {
		req.Body = http.NoBody
		return ""
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return string(body)
}
