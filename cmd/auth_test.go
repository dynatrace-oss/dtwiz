package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

			orig := credentialHTTPClient
			credentialHTTPClient = srv.Client()
			defer func() { credentialHTTPClient = orig }()

			err := checkClassicAccess(srv.URL, "dt0s16.testtoken")
			if (err != nil) != tt.wantErr {
				t.Errorf("checkClassicAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckClassicAccess_NetworkFailure(t *testing.T) {
	orig := credentialHTTPClient
	credentialHTTPClient = &http.Client{}
	defer func() { credentialHTTPClient = orig }()

	// Point at a closed server so the TCP dial fails immediately.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	if err := checkClassicAccess(srv.URL, "dt0s16.testtoken"); err == nil {
		t.Error("expected error for unreachable server, got nil")
	}
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
				if r.URL.Path == "/platform/storage/query/v1/query:execute" {
					w.WriteHeader(tt.dqlStatus)
				} else {
					w.WriteHeader(tt.classicStatus)
				}
			}))
			defer srv.Close()

			orig := credentialHTTPClient
			credentialHTTPClient = srv.Client()
			defer func() { credentialHTTPClient = orig }()

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
