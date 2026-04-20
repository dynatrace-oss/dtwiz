package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/fatih/color"
)

// captureOutput redirects color.Output (the writer used by fatih/color and
// display.PrintStatusLine) for the duration of fn and returns what was written.
// Colors are disabled, so assertions are not fragile against terminal detection.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer

	origOutput := color.Output
	color.Output = &buf
	t.Cleanup(func() { color.Output = origOutput })

	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	fn()

	return buf.String()
}

// errVerify is a verifyFn that always returns an error.
func errVerify(_, _ string) error { return errors.New("authentication failed") }

// okVerify is a verifyFn that always succeeds.
func okVerify(_, _ string) error { return nil }

// Auth Credential Status Printing
func TestPrintCredentialStatus_TokenNotSet(t *testing.T) {
	got := captureOutput(t, func() {
		printCredentialStatus("Access Token", "https://abc.live.com", CredentialToken{
			value:         "",
			cliName:       "access-token",
			envName:       "DT_ACCESS_TOKEN",
			tokenVerifyFn: okVerify,
			getUrlFn:      installer.APIURL,
		})
	})

	want := "✗ not set (use --access-token or DT_ACCESS_TOKEN)"
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q, got %q", want, got)
	}
}

func TestPrintCredentialStatus_TokenSetNoEnvURL(t *testing.T) {
	got := captureOutput(t, func() {
		printCredentialStatus("Access Token", "", CredentialToken{
			value:         "dt0c01.some-token",
			cliName:       "access-token",
			envName:       "DT_ACCESS_TOKEN",
			tokenVerifyFn: errVerify,
			getUrlFn:      installer.APIURL,
		})
	})

	want := "✓ configured (skipped validation — no environment URL)"
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q, got %q", want, got)
	}
}

func TestPrintCredentialStatus_TokenSetEnvURLVerifyOK(t *testing.T) {
	got := captureOutput(t, func() {
		printCredentialStatus("Access Token", "https://abc.live.com", CredentialToken{
			value:         "dt0c01.some-token",
			cliName:       "access-token",
			envName:       "DT_ACCESS_TOKEN",
			tokenVerifyFn: okVerify,
			getUrlFn:      installer.APIURL,
		})
	})

	if !strings.Contains(got, "✓ valid") {
		t.Errorf("expected output to contain %q, got %q", "✓ valid", got)
	}
	if !strings.Contains(got, "abc.live.com") {
		t.Errorf("expected output to contain the environment URL, got %q", got)
	}
}

func TestPrintCredentialStatus_TokenSetEnvURLVerifyFails(t *testing.T) {
	got := captureOutput(t, func() {
		printCredentialStatus("Access Token", "https://abc.live.com", CredentialToken{
			value:         "dt0c01.bad-token",
			cliName:       "access-token",
			envName:       "DT_ACCESS_TOKEN",
			tokenVerifyFn: errVerify,
			getUrlFn:      installer.APIURL,
		})
	})

	want := "authentication failed"
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q, got %q", want, got)
	}
}

func TestPrintCredentialStatus_LabelAppearsInOutput(t *testing.T) {
	got := captureOutput(t, func() {
		printCredentialStatus("Platform Token", "", CredentialToken{
			value:         "dt0s16.some-token",
			cliName:       "platform-token",
			envName:       "DT_PLATFORM_TOKEN",
			tokenVerifyFn: okVerify,
			getUrlFn:      installer.APIURL,
		})
	})

	if !strings.Contains(got, "Platform Token:") {
		t.Errorf("expected label %q in output, got %q", "Platform Token", got)
	}
}

func TestPrintCredentialStatus_VerifyNotCalledWhenNoEnvURL(t *testing.T) {
	called := false
	spyVerify := func(_, _ string) error {
		called = true
		return nil
	}

	captureOutput(t, func() {
		printCredentialStatus("Access Token", "", CredentialToken{
			value:         "dt0c01.some-token",
			cliName:       "access-token",
			envName:       "DT_ACCESS_TOKEN",
			tokenVerifyFn: spyVerify,
			getUrlFn:      installer.APIURL,
		})
	})

	if called {
		t.Error("verifyFn should not be called when envURL is empty")
	}
}

func TestPrintCredentialStatus_VerifyCalledWithCorrectArgs(t *testing.T) {
	const wantEnvURL = "https://abc.live.com"
	const wantToken = "dt0c01.my-token"

	var gotEnvURL, gotToken string
	spyVerify := func(envURL, token string) error {
		gotEnvURL = envURL
		gotToken = token
		return nil
	}

	captureOutput(t, func() {
		printCredentialStatus("Access Token", wantEnvURL, CredentialToken{
			value:         wantToken,
			cliName:       "access-token",
			envName:       "DT_ACCESS_TOKEN",
			tokenVerifyFn: spyVerify,
			getUrlFn:      installer.APIURL,
		})
	})

	if gotEnvURL != wantEnvURL {
		t.Errorf("verifyFn called with envURL=%q, want %q", gotEnvURL, wantEnvURL)
	}
	if gotToken != wantToken {
		t.Errorf("verifyFn called with token=%q, want %q", gotToken, wantToken)
	}
}

// Feature Flag Printing
func TestFeatureFlags_NoFlagsEnabled_SectionOmitted(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")

	got := captureOutput(t, func() {
		printFeatureFlags()
	})

	if strings.Contains(got, "Feature Flags") {
		t.Errorf("expected no Feature Flags section when no flags are enabled, got %q", got)
	}
}

func TestFeatureFlags_FlagEnabledViaCLI_SectionAppears(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.AllRuntimes, true)

	got := captureOutput(t, func() {
		printFeatureFlags()
	})

	if !strings.Contains(got, "Feature Flags") {
		t.Errorf("expected Feature Flags header, got %q", got)
	}
	if !strings.Contains(got, "DTWIZ_ALL_RUNTIMES") {
		t.Errorf("expected DTWIZ_ALL_RUNTIMES in output, got %q", got)
	}
	if !strings.Contains(got, "enabled (cli)") {
		t.Errorf("expected source 'cli' in output, got %q", got)
	}
}

func TestFeatureFlags_FlagEnabledViaEnv_SectionAppears(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "true")

	got := captureOutput(t, func() {
		printFeatureFlags()
	})

	if !strings.Contains(got, "Feature Flags") {
		t.Errorf("expected Feature Flags header, got %q", got)
	}
	if !strings.Contains(got, "DTWIZ_ALL_RUNTIMES") {
		t.Errorf("expected DTWIZ_ALL_RUNTIMES in output, got %q", got)
	}
	if !strings.Contains(got, "enabled (env)") {
		t.Errorf("expected source 'env' in output, got %q", got)
	}
}

func TestFeatureFlags_FlagDisabledAfterCLIOverrideCleared_SectionOmitted(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.AllRuntimes, false)

	got := captureOutput(t, func() {
		printFeatureFlags()
	})

	if strings.Contains(got, "Feature Flags") {
		t.Errorf("expected no Feature Flags section when flag is explicitly disabled, got %q", got)
	}
}
