package cmd

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSensitiveHTTPHeaders(t *testing.T) {
	for _, h := range []string{"authorization", "x-api-key", "cookie", "set-cookie"} {
		if !sensitiveHTTPHeaders[h] {
			t.Errorf("sensitiveHTTPHeaders missing %q", h)
		}
	}
}

func TestNewRestyClient_Settings(t *testing.T) {
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
	if got := rc.Header.Get("User-Agent"); got != "dtwiz/"+Version {
		t.Errorf("User-Agent = %q, want %q", got, "dtwiz/"+Version)
	}
	if got := rc.Header.Get("Accept-Encoding"); got != "gzip" {
		t.Errorf("Accept-Encoding = %q, want gzip", got)
	}
}

func TestNewRestyClient_RetryOn429(t *testing.T) {
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

func TestNewHTTPClient_MissingEnvironment(t *testing.T) {
	t.Setenv("DT_ENVIRONMENT", "")
	t.Setenv("DT_ACCESS_TOKEN", "")
	t.Setenv("DT_PLATFORM_TOKEN", "")
	environmentFlag, accessTokenFlag, platformTokenFlag = "", "", ""

	if _, err := NewHTTPClient(); err == nil {
		t.Fatal("expected error when DT_ENVIRONMENT is not set")
	}
}

func TestNewHTTPClient_MissingAccessToken(t *testing.T) {
	t.Setenv("DT_ENVIRONMENT", "https://abc123.apps.dynatrace.com")
	t.Setenv("DT_ACCESS_TOKEN", "")
	t.Setenv("DT_PLATFORM_TOKEN", "")
	environmentFlag, accessTokenFlag, platformTokenFlag = "", "", ""

	if _, err := NewHTTPClient(); err == nil {
		t.Fatal("expected error when DT_ACCESS_TOKEN is not set")
	}
}

func TestNewHTTPClient_MissingPlatformToken(t *testing.T) {
	t.Setenv("DT_ENVIRONMENT", "https://abc123.apps.dynatrace.com")
	t.Setenv("DT_ACCESS_TOKEN", "dt0c01.test")
	t.Setenv("DT_PLATFORM_TOKEN", "")
	environmentFlag, accessTokenFlag, platformTokenFlag = "", "", ""

	if _, err := NewHTTPClient(); err == nil {
		t.Fatal("expected error when DT_PLATFORM_TOKEN is not set")
	}
}

func TestNewHTTPClient_Success(t *testing.T) {
	t.Setenv("DT_ENVIRONMENT", "https://abc123.apps.dynatrace.com")
	t.Setenv("DT_ACCESS_TOKEN", "dt0c01.test")
	t.Setenv("DT_PLATFORM_TOKEN", "dt0s16.test")
	environmentFlag, accessTokenFlag, platformTokenFlag = "", "", ""

	c, err := NewHTTPClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Classic == nil || c.Platform == nil {
		t.Fatal("Classic or Platform is nil")
	}

	const wantClassic = "https://abc123.live.dynatrace.com"
	if got := c.Classic.BaseURL(); got != wantClassic {
		t.Errorf("Classic.BaseURL() = %q, want %q", got, wantClassic)
	}
	const wantPlatform = "https://abc123.apps.dynatrace.com"
	if got := c.Platform.BaseURL(); got != wantPlatform {
		t.Errorf("Platform.BaseURL() = %q, want %q", got, wantPlatform)
	}
}
