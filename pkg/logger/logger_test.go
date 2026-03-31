package logger

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	Init(true, 0)
	if Verbosity() != 2 {
		t.Fatalf("expected verbosity 2 when debug=true and verbosity=0, got %d", Verbosity())
	}
	Init(false, 1)
	if Verbosity() != 1 {
		t.Fatalf("expected verbosity 1 when debug=false and verbosity=1, got %d", Verbosity())
	}
}

func TestLoggingTransportLevel1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	Init(false, 1)
	defer Init(false, 0)

	var buf strings.Builder
	tr := &LoggingTransport{Base: http.DefaultTransport}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_ = buf // output goes to stderr in production; test verifies no crash/error
}

func TestLoggingTransportSensitiveHeaderRedacted(t *testing.T) {
	if !isSensitiveHeader("Authorization") {
		t.Fatal("Authorization should be sensitive")
	}
	if !isSensitiveHeader("authorization") {
		t.Fatal("authorization (lower) should be sensitive")
	}
	if isSensitiveHeader("Content-Type") {
		t.Fatal("Content-Type should not be sensitive")
	}
}

func TestLoggingTransportPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	Init(false, 0)
	defer Init(false, 0)

	tr := NewLoggingTransport(nil) // nil → http.DefaultTransport
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := tr.(*LoggingTransport).RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", resp.StatusCode)
	}
}
