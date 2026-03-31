package logger

import (
	"bytes"
	"log/slog"
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
	Init(true, 0)
	if Verbosity() != 2 {
		t.Fatalf("expected verbosity 2 when debug=true and verbosity=0, got %d", Verbosity())
	}
	Init(false, 1)
	if Verbosity() != 1 {
		t.Fatalf("expected verbosity 1 when debug=false and verbosity=1, got %d", Verbosity())
	}
}
