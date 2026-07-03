package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateAzureCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range updateCmd.Commands() {
		if cmd.Use == "azure" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(updateCmd.Commands()))
		for _, cmd := range updateCmd.Commands() {
			names = append(names, cmd.Use)
		}
		t.Errorf("expected azure subcommand to be registered under update, found: %v", names)
	}
}

func TestUpdateAzureCmd_RunE_ValidatesPlatformToken(t *testing.T) {
	origCredentialHTTPClient := credentialHTTPClient
	origEnvironmentFlag := environmentFlag
	origPlatformTokenFlag := platformTokenFlag
	origAccessTokenFlag := accessTokenFlag
	t.Cleanup(func() {
		credentialHTTPClient = origCredentialHTTPClient
		environmentFlag = origEnvironmentFlag
		platformTokenFlag = origPlatformTokenFlag
		accessTokenFlag = origAccessTokenFlag
	})

	validationCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validationCalled = true
		if r.URL.Path != "/platform/storage/query/v1/query:execute" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dt0s16.platform" {
			t.Fatalf("Authorization header = %q, want platform bearer token", got)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	credentialHTTPClient = srv.Client()
	environmentFlag = srv.URL
	platformTokenFlag = "dt0s16.platform"
	accessTokenFlag = "dt0c01.access"

	err := updateAzureCmd.RunE(updateAzureCmd, nil)
	if err == nil {
		t.Fatal("expected platform token validation error, got nil")
	}
	if !validationCalled {
		t.Fatal("expected Azure update command to validate the platform token")
	}
}
