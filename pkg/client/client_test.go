package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

func TestSensitiveHTTPHeaders(t *testing.T) {
	t.Parallel()

	for _, h := range []string{"authorization", "x-api-key", "cookie", "set-cookie"} {
		if !sensitiveHTTPHeaders[h] {
			t.Errorf("sensitiveHTTPHeaders missing %q", h)
		}
	}
}

func TestAuthHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "classic api token", token: "dt0c01.abc.secret", want: "Api-Token dt0c01.abc.secret"},
		{name: "platform token", token: "dt0s16.abc.secret", want: "Bearer dt0s16.abc.secret"},
		{name: "oauth token", token: "oauth-token", want: "Bearer oauth-token"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := authHeader(tt.token); got != tt.want {
				t.Fatalf("authHeader(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	c, err := New("https://abc.live.dynatrace.com", "https://abc.apps.dynatrace.com", "dt0c01.classic", "dt0s16.platform", 0)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if got := c.Classic.BaseURL(); got != "https://abc.live.dynatrace.com" {
		t.Errorf("Classic.BaseURL() = %q, want classic URL", got)
	}
	if got := c.Platform.BaseURL(); got != "https://abc.apps.dynatrace.com" {
		t.Errorf("Platform.BaseURL() = %q, want platform URL", got)
	}
	if got := c.Classic.HTTP().Header.Get("Authorization"); got != "Api-Token dt0c01.classic" {
		t.Errorf("classic Authorization = %q, want Api-Token header", got)
	}
	if got := c.Platform.HTTP().Header.Get("Authorization"); got != "Bearer dt0s16.platform" {
		t.Errorf("platform Authorization = %q, want Bearer header", got)
	}
}

func TestNewRequiresBaseURLs(t *testing.T) {
	t.Parallel()

	if _, err := New("", "https://abc.apps.dynatrace.com", "classic", "platform", 0); err == nil {
		t.Fatal("expected error for missing classic URL")
	}
	if _, err := New("https://abc.live.dynatrace.com", "", "classic", "platform", 0); err == nil {
		t.Fatal("expected error for missing platform URL")
	}
}

func TestNewRestyClient_VerboseLoggingRedactsSensitiveHeadersAndTruncatesBody(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 2100)))
	}))
	defer srv.Close()

	rc := newRestyClient(srv.URL, "Bearer secret-token", 2)
	_, err = rc.R().SetHeader("X-Api-Key", "api-key").SetHeader("Cookie", "session-id").Get("/")
	if err != nil {
		t.Fatalf("request returned error: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	text := string(out)
	for _, want := range []string{"===> REQUEST <===", "===> RESPONSE <===", "BODY (first 2048 of 2100 bytes):", "Authorization: [REDACTED]", "X-Api-Key: [REDACTED]", "Cookie: [REDACTED]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("verbose output missing %q:\n%s", want, text)
		}
	}
	for _, secret := range []string{"secret-token", "api-key", "session-id"} {
		if strings.Contains(text, secret) {
			t.Fatalf("verbose output leaked %q:\n%s", secret, text)
		}
	}
}

func TestNewRestyClient_Settings(t *testing.T) {
	t.Parallel()

	rc := newRestyClient("https://example.com", "Api-Token tok", 0)

	if rc.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", rc.RetryCount)
	}
	if rc.RetryWaitTime != time.Second {
		t.Errorf("RetryWaitTime = %v, want 1s", rc.RetryWaitTime)
	}
	if rc.RetryMaxWaitTime != 10*time.Second {
		t.Errorf("RetryMaxWaitTime = %v, want 10s", rc.RetryMaxWaitTime)
	}
	if rc.GetClient().Timeout != 6*time.Minute {
		t.Errorf("Timeout = %v, want 6m", rc.GetClient().Timeout)
	}
	if got := rc.Header.Get("Authorization"); got != "Api-Token tok" {
		t.Errorf("Authorization = %q, want %q", got, "Api-Token tok")
	}
	if got := rc.Header.Get("User-Agent"); got != "dtwiz/"+version.Version {
		t.Errorf("User-Agent = %q, want %q", got, "dtwiz/"+version.Version)
	}
	if got := rc.Header.Get("Accept-Encoding"); got != "gzip" {
		t.Errorf("Accept-Encoding = %q, want gzip", got)
	}
}

func TestNewRestyClient_RetryOn429(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rc := newRestyClient(srv.URL, "token", 0)
	rc.RetryWaitTime = time.Millisecond
	rc.RetryMaxWaitTime = time.Millisecond

	if _, err := rc.R().Get("/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("server calls = %d, want 2 (1 retry after 429)", n)
	}
}

func TestNewRestyClient_RetryOn5xx(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rc := newRestyClient(srv.URL, "token", 0)
	rc.RetryWaitTime = time.Millisecond
	rc.RetryMaxWaitTime = time.Millisecond

	if _, err := rc.R().Get("/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("server calls = %d, want 2 (1 retry after 500)", n)
	}
}

func TestNewRestyClient_NoRetryOn4xx(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	rc := newRestyClient(srv.URL, "token", 0)
	rc.RetryWaitTime = time.Millisecond

	if _, err := rc.R().Get("/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("server calls = %d, want 1 (no retry on 404)", n)
	}
}
