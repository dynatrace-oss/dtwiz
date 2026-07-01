package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
)

func TestInstallOtelNodeCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range installCmd.Commands() {
		if cmd.Use == "otel-node" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(installCmd.Commands()))
		for _, cmd := range installCmd.Commands() {
			names = append(names, cmd.Use)
		}
		t.Errorf("expected otel-node subcommand to be registered, found: %v", names)
	}
}

func TestInstallDockerCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range installCmd.Commands() {
		if cmd.Use == "docker" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected docker subcommand to be registered under install")
	}
}

func TestInstallDockerCmd_HiddenByDefault(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	if !installDockerCmd.Hidden {
		t.Error("expected docker subcommand to be hidden when experimental is not enabled")
	}
}

func TestInstallDockerCmd_VisibleWhenExperimental(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
	// Simulate what the HelpFunc does: update Hidden based on the flag.
	installDockerCmd.Hidden = !featureflags.IsEnabled(featureflags.Experimental)
	t.Cleanup(func() { installDockerCmd.Hidden = true })

	if installDockerCmd.Hidden {
		t.Error("expected docker subcommand to be visible when experimental is enabled")
	}
}

func TestInstallDockerCmd_RunE_BlockedWithoutExperimental(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	err := installDockerCmd.RunE(installDockerCmd, nil)
	if err == nil {
		t.Fatal("expected error when running docker without experimental flag")
	}
	want := "docker installation is an experimental feature; enable it with --experimental or DTWIZ_EXPERIMENTAL=true"
	if err.Error() != want {
		t.Errorf("unexpected error message:\n got:  %s\n want: %s", err.Error(), want)
	}
}

func TestInstallAzureCmd_RunE_ValidatesPlatformToken(t *testing.T) {
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

	err := installAzureCmd.RunE(installAzureCmd, nil)
	if err == nil {
		t.Fatal("expected platform token validation error, got nil")
	}
	if !validationCalled {
		t.Fatal("expected Azure install command to validate the platform token")
	}
}
