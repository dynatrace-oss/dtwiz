package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"
	"github.com/spf13/cobra"
)

// TestMain disables credential-probe retries so 5xx and unreachable-host cases fail fast.
func TestMain(m *testing.M) {
	credentialClientOpts = []httpclient.Option{
		httpclient.WithTimeout(5 * time.Second),
		httpclient.WithRetry(0, 0, 0),
	}
	os.Exit(m.Run())
}

func TestCheckPlatformToken(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		wantErr     bool
		wantContain string
	}{
		{"200 ok", http.StatusOK, false, ""},
		{"401 unauthorized", http.StatusUnauthorized, true, "Platform token: authentication failed"},
		{"403 forbidden", http.StatusForbidden, true, "Platform token: insufficient permissions"},
		{"500 server error", http.StatusInternalServerError, true, "unexpected response 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if r.URL.Path != "/platform/storage/query/v1/query:verify" {
					t.Errorf("path = %s, want /platform/storage/query/v1/query:verify", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer dt0s16.testtoken" {
					t.Errorf("Authorization = %q, want %q", got, "Bearer dt0s16.testtoken")
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			err := checkPlatformToken(srv.URL, "dt0s16.testtoken")
			if (err != nil) != tt.wantErr {
				t.Errorf("checkPlatformToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantContain != "" && err != nil {
				if !strings.Contains(err.Error(), tt.wantContain) {
					t.Errorf("checkPlatformToken() error = %q, want it to contain %q", err.Error(), tt.wantContain)
				}
			}
		})
	}
}

func TestCheckAccessToken(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		wantErr     bool
		wantContain string
	}{
		{"200 ok", http.StatusOK, false, ""},
		{"401 unauthorized", http.StatusUnauthorized, true, "Access token: authentication failed"},
		{"403 forbidden", http.StatusForbidden, true, "Access token: insufficient permissions"},
		{"500 server error", http.StatusInternalServerError, true, "unexpected response 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if r.URL.Path != "/api/v2/apiTokens/lookup" {
					t.Errorf("path = %s, want /api/v2/apiTokens/lookup", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Api-Token dt0c01.testtoken" {
					t.Errorf("Authorization = %q, want %q", got, "Api-Token dt0c01.testtoken")
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			err := checkAccessToken(srv.URL, "dt0c01.testtoken")
			if (err != nil) != tt.wantErr {
				t.Errorf("checkAccessToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantContain != "" && err != nil {
				if !strings.Contains(err.Error(), tt.wantContain) {
					t.Errorf("checkAccessToken() error = %q, want it to contain %q", err.Error(), tt.wantContain)
				}
			}
		})
	}
}

func TestCheckClassicAccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"200 ok", http.StatusOK, false},
		{"404 not found — auth ok endpoint absent", http.StatusNotFound, false},
		{"500 server error — auth ok", http.StatusInternalServerError, false},
		{"401 unauthorized", http.StatusUnauthorized, true},
		{"403 forbidden", http.StatusForbidden, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			err := checkPlatformTokenClassicAccess(srv.URL, "dt0s16.testtoken")
			if (err != nil) != tt.wantErr {
				t.Errorf("checkPlatformTokenClassicAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckClassicAccess_NetworkFailure(t *testing.T) {
	// Point at a closed server so the TCP dial fails immediately.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	if err := checkPlatformTokenClassicAccess(srv.URL, "dt0s16.testtoken"); err == nil {
		t.Error("expected error for unreachable server, got nil")
	}
}

func TestAccessToken(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		envVar string
		want   string
	}{
		{"flag set returns flag value", "dt0c01.fromflag", "", "dt0c01.fromflag"},
		{"flag empty returns empty even when env var set", "", "dt0c01.fromenv", ""},
		{"flag wins, env var ignored", "dt0c01.fromflag", "dt0c01.fromenv", "dt0c01.fromflag"},
		{"both empty returns empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := accessTokenFlag
			accessTokenFlag = tt.flag
			defer func() { accessTokenFlag = orig }()
			t.Setenv("DT_ACCESS_TOKEN", tt.envVar)

			if got := accessToken(); got != tt.want {
				t.Errorf("accessToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetDtEnvironment_AccessTokenFlagOnly(t *testing.T) {
	t.Setenv("DT_ENVIRONMENT", "https://abc12345.dynatracelabs.com/")
	t.Setenv("DT_PLATFORM_TOKEN", "dt0s16.platform")
	// A leftover access-token env var must never activate access-token auth.
	t.Setenv("DT_ACCESS_TOKEN", "dt0c01.leftover")

	t.Run("flag unset — access token empty despite env var", func(t *testing.T) {
		orig := accessTokenFlag
		accessTokenFlag = ""
		defer func() { accessTokenFlag = orig }()

		_, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			t.Fatalf("getDtEnvironment() unexpected error: %v", err)
		}
		if accessTok != "" {
			t.Errorf("accessTok = %q, want empty (env var must not activate access-token auth)", accessTok)
		}
		if platformTok != "dt0s16.platform" {
			t.Errorf("platformTok = %q, want %q", platformTok, "dt0s16.platform")
		}
	})

	t.Run("flag set — access token from flag", func(t *testing.T) {
		orig := accessTokenFlag
		accessTokenFlag = "dt0c01.fromflag"
		defer func() { accessTokenFlag = orig }()

		_, accessTok, _, err := getDtEnvironment()
		if err != nil {
			t.Fatalf("getDtEnvironment() unexpected error: %v", err)
		}
		if accessTok != "dt0c01.fromflag" {
			t.Errorf("accessTok = %q, want %q", accessTok, "dt0c01.fromflag")
		}
	})
}

func TestValidateCredentials(t *testing.T) {
	const platformTok = "dt0s16.platform"
	const accessTok = "dt0c01.access"

	tests := []struct {
		name           string
		dqlStatus      int
		classicStatus  int
		accessTok      string
		wantClassicTok string
		wantErr        bool
	}{
		{
			name:           "no access token, platform token accepted by Classic API",
			dqlStatus:      http.StatusOK,
			classicStatus:  http.StatusOK,
			accessTok:      "",
			wantClassicTok: platformTok,
		},
		{
			name:           "access token set — takes precedence over platform token for Classic API",
			dqlStatus:      http.StatusOK,
			classicStatus:  http.StatusUnauthorized, // irrelevant, access token wins
			accessTok:      accessTok,
			wantClassicTok: accessTok,
		},
		{
			name:           "no access token, platform token rejected by Classic API — still used",
			dqlStatus:      http.StatusOK,
			classicStatus:  http.StatusUnauthorized,
			accessTok:      "",
			wantClassicTok: platformTok,
		},
		{
			name:           "no access token, 403 from Classic API — platform token still used",
			dqlStatus:      http.StatusOK,
			classicStatus:  http.StatusForbidden,
			accessTok:      "",
			wantClassicTok: platformTok,
		},
		{
			name:      "DQL validation fails — hard error regardless of access token",
			dqlStatus: http.StatusUnauthorized,
			// classicStatus is irrelevant here
			classicStatus: http.StatusOK,
			accessTok:     accessTok,
			wantErr:       true,
		},
		{
			name:      "DQL 403 is also a hard error",
			dqlStatus: http.StatusForbidden,
			accessTok: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/platform/storage/query/v1/query:verify" {
					w.WriteHeader(tt.dqlStatus)
				} else {
					w.WriteHeader(tt.classicStatus)
				}
			}))
			defer srv.Close()

			classicTok, err := validateCredentials(srv.URL, tt.accessTok, platformTok)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && classicTok != tt.wantClassicTok {
				t.Errorf("validateCredentials() classicTok = %q, want %q", classicTok, tt.wantClassicTok)
			}
		})
	}
}

func TestEnvironmentHint_NormalizesScheme(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		envVar string
		want   string
	}{
		{"bare host from flag", "abc12345.apps.dynatrace.com", "", "https://abc12345.apps.dynatrace.com"},
		{"bare host from env var", "", "abc12345.live.dynatrace.com", "https://abc12345.live.dynatrace.com"},
		{"https kept", "https://abc12345.apps.dynatrace.com", "", "https://abc12345.apps.dynatrace.com"},
		{"http kept", "http://localhost:8080", "", "http://localhost:8080"},
		{"unset stays empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := environmentFlag
			environmentFlag = tt.flag
			defer func() { environmentFlag = orig }()
			t.Setenv("DT_ENVIRONMENT", tt.envVar)

			if got := environmentHint(); got != tt.want {
				t.Errorf("environmentHint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckPlatformToken_ManagedURLHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := checkPlatformToken(srv.URL+"/e/abc12345", "dt0s16.testtoken")
	if err == nil {
		t.Fatal("expected error for Managed URL, got nil")
	}
	if !strings.Contains(err.Error(), "Dynatrace Managed or ActiveGate URL") {
		t.Errorf("error = %q, want it to contain the Managed URL hint", err.Error())
	}
}

func TestEnvironmentURLHints(t *testing.T) {
	tests := []struct {
		name    string
		appsURL string
		want    string
	}{
		{"platform URL has no hints", "https://abc12345.apps.dynatrace.com", ""},
		{"labs platform URL has no hints", "https://abc12345.sprint.apps.dynatracelabs.com", ""},
		{"managed URL gets a hint", "https://managed.example.com/e/abc12345", "Possible wrong environment URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := environmentURLHints(tt.appsURL)
			if tt.want == "" && got != "" {
				t.Errorf("environmentURLHints(%q) = %q, want no hints", tt.appsURL, got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("environmentURLHints(%q) = %q, want it to contain %q", tt.appsURL, got, tt.want)
			}
		})
	}
}

// assertRunEValidatesPlatformToken runs cmd against a server that rejects the
// platform token and checks that RunE probed the DQL endpoint and failed.
func assertRunEValidatesPlatformToken(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	origEnvironmentFlag := environmentFlag
	origPlatformTokenFlag := platformTokenFlag
	t.Cleanup(func() {
		environmentFlag = origEnvironmentFlag
		platformTokenFlag = origPlatformTokenFlag
	})

	validationCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/storage/query/v1/query:verify" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		validationCalled = true
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	environmentFlag = srv.URL
	platformTokenFlag = "dt0s16.platform"

	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected platform token validation error, got nil")
	}
	if !validationCalled {
		t.Fatalf("expected %q to validate the platform token", cmd.CommandPath())
	}
}

// The token probe must only VERIFY a statement, never execute one: query:verify needs no
// storage:* scope, so a token without storage:system:read (or any other table scope) passes
// when it authenticates.
func TestCheckPlatformToken_VerifiesWithoutExecuting(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "query:execute") {
			w.WriteHeader(http.StatusForbidden) // a token without table scopes
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := checkPlatformToken(srv.URL, "dt0s16.testtoken"); err != nil {
		t.Fatalf("checkPlatformToken() error = %v", err)
	}
	if len(paths) != 1 || paths[0] != "/platform/storage/query/v1/query:verify" {
		t.Errorf("probe paths = %v, want exactly [/platform/storage/query/v1/query:verify]", paths)
	}
}
