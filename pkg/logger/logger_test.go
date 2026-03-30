package logger

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestInitDebugEnabled(t *testing.T) {
	Init(true)
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
	Init(false)
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
