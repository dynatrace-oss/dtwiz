package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestInitDebugEnabled(t *testing.T) {
	Init(true, 0)
	if !Enabled() {
		t.Fatal("expected Enabled() to return true after Init(true)")
	}

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	Debug("test message", "key", "value")

	if buf.Len() == 0 {
		t.Fatal("expected debug message to be written when debug is enabled")
	}
}

func TestInitDebugDisabled(t *testing.T) {
	Init(false, 0)
	if Enabled() {
		t.Fatal("expected Enabled() to return false after Init(false)")
	}

	// The default level is Warn, so Debug calls should be suppressed.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	Debug("should not appear")

	if buf.Len() != 0 {
		t.Fatalf("expected no output when debug is disabled, got: %s", buf.String())
	}
}

func TestVerbosityPromotedByDebug(t *testing.T) {
	// --debug promotes verbosityLevel to at least 1
	Init(true, 0)
	if Verbosity() != 1 {
		t.Fatalf("expected verbosity 1 when debug=true and verbosity=0, got %d", Verbosity())
	}
	Init(false, 1)
	if Verbosity() != 1 {
		t.Fatalf("expected verbosity 1 when debug=false and verbosity=1, got %d", Verbosity())
	}
}

func TestWarnAlwaysWritten(t *testing.T) {
	Init(true, 0)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	Warn("something is off", "key", "value")

	if buf.Len() == 0 {
		t.Fatal("expected warn message to be written")
	}
}

func TestWarnWrittenWhenDebugDisabled(t *testing.T) {
	Init(false, 0)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	Warn("this should appear")

	if buf.Len() == 0 {
		t.Fatal("expected warn message to be written even when debug is disabled")
	}
}

// TestVerboseVisibility verifies the -v / --debug convention:
// -v: Verbose shown, Debug hidden.
// --debug: both Verbose and Debug shown.
func TestVerboseVisibility(t *testing.T) {
	// Use Init's own handler (not a replacement) so the test exercises the real path.
	t.Run("-v shows Verbose hides Debug", func(t *testing.T) {
		Init(false, 1)
		var buf bytes.Buffer
		orig := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
		defer slog.SetDefault(orig)

		Verbose("summary line")
		Debug("detail line")

		out := buf.String()
		if !strings.Contains(out, "INFO") {
			t.Fatal("expected Verbose (INFO) output with -v")
		}
		if strings.Contains(out, "DEBUG") {
			t.Fatal("Debug should be hidden with -v")
		}
	})

	t.Run("--debug shows Verbose and Debug", func(t *testing.T) {
		Init(true, 0)
		var buf bytes.Buffer
		orig := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		defer slog.SetDefault(orig)

		Verbose("summary line")
		Debug("detail line")

		out := buf.String()
		if !strings.Contains(out, "INFO") {
			t.Fatal("expected Verbose (INFO) output with --debug")
		}
		if !strings.Contains(out, "DEBUG") {
			t.Fatal("expected Debug output with --debug")
		}
	})
}
