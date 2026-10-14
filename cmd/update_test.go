package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
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

func TestUpdateGcpCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range updateCmd.Commands() {
		if cmd.Use == "gcp" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(updateCmd.Commands()))
		for _, cmd := range updateCmd.Commands() {
			names = append(names, cmd.Use)
		}
		t.Errorf("expected gcp subcommand to be registered under update, found: %v", names)
	}
}

func TestUpdateGcpCmd_RunE_ValidatesPlatformToken(t *testing.T) {
	origEnvironmentFlag := environmentFlag
	origPlatformTokenFlag := platformTokenFlag
	origAccessTokenFlag := accessTokenFlag
	t.Cleanup(func() {
		environmentFlag = origEnvironmentFlag
		platformTokenFlag = origPlatformTokenFlag
		accessTokenFlag = origAccessTokenFlag
	})

	validationCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validationCalled = true
		if r.URL.Path != "/platform/storage/query/v1/query:execute" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dt0s16.platform" {
			t.Errorf("Authorization header = %q, want platform bearer token", got)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	environmentFlag = srv.URL
	platformTokenFlag = "dt0s16.platform"
	accessTokenFlag = "dt0c01.access"

	err := updateGcpCmd.RunE(updateGcpCmd, nil)
	if err == nil {
		t.Fatal("expected platform token validation error, got nil")
	}
	if !validationCalled {
		t.Fatal("expected GCP update command to validate the platform token")
	}
}

func TestUpdateAzureCmd_RunE_ValidatesPlatformToken(t *testing.T) {
	origEnvironmentFlag := environmentFlag
	origPlatformTokenFlag := platformTokenFlag
	origAccessTokenFlag := accessTokenFlag
	t.Cleanup(func() {
		environmentFlag = origEnvironmentFlag
		platformTokenFlag = origPlatformTokenFlag
		accessTokenFlag = origAccessTokenFlag
	})

	validationCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validationCalled = true
		if r.URL.Path != "/platform/storage/query/v1/query:execute" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dt0s16.platform" {
			t.Errorf("Authorization header = %q, want platform bearer token", got)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

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

// TestUpdateCmd_OtelRegistered verifies the update otel subcommand is registered.
func TestUpdateCmd_OtelRegistered(t *testing.T) {
	found := false
	for _, c := range updateCmd.Commands() {
		if c.Use == "otel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected update otel subcommand to be registered")
	}
}

func TestUpdateOtelCmd_HiddenByDefault(t *testing.T) {
	t.Setenv("DTWIZ_EXPERIMENTAL", "")
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	if !updateOtelCmd.Hidden {
		t.Error("expected update otel subcommand to be hidden when experimental is not enabled")
	}
}

func TestUpdateOtelCmd_VisibleWhenExperimental(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
	// Simulate what the HelpFunc does: update Hidden based on the flag.
	updateOtelCmd.Hidden = !featureflags.IsEnabled(featureflags.Experimental)
	t.Cleanup(func() { updateOtelCmd.Hidden = true })

	if updateOtelCmd.Hidden {
		t.Error("expected update otel subcommand to be visible when experimental is enabled")
	}
}

func TestUpdateOtelCmd_RunE_BlockedWithoutExperimental(t *testing.T) {
	t.Setenv("DTWIZ_EXPERIMENTAL", "")
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	err := updateOtelCmd.RunE(updateOtelCmd, nil)
	if err == nil {
		t.Fatal("expected error when running update otel without experimental flag")
	}
	want := "otel update is an experimental feature; enable it with --experimental or DTWIZ_EXPERIMENTAL=true"
	if err.Error() != want {
		t.Errorf("unexpected error message:\n got:  %s\n want: %s", err.Error(), want)
	}
}
