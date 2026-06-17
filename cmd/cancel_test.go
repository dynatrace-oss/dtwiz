package cmd

import (
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
)

// TestInstallCmd_CancelledSubcommands verifies each install subcommand that
// carries a ErrInstallCancelled guard is actually registered.
//
// Note: these tests verify command registration (structural wiring), not that
// the guard is present in RunE. Testing the real RunE paths requires injectable
// installer dependencies; that is tracked as a separate refactor.
func TestInstallCmd_CancelledSubcommands(t *testing.T) {
	guarded := []string{
		"kubernetes",
		"otel",
		"otel-collector",
		"otel-python",
		"otel-node",
		"otel-java",
		"aws",
		"aws-lambda",
		"demo",
	}
	registered := map[string]bool{}
	for _, c := range installCmd.Commands() {
		registered[c.Use] = true
	}
	for _, name := range guarded {
		if !registered[name] {
			t.Errorf("expected install subcommand %q to be registered", name)
		}
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

// TestUninstallCmd_AWSLambdaCancelledSubcommand verifies the uninstall
// aws-lambda subcommand is registered.
func TestUninstallCmd_AWSLambdaCancelledSubcommand(t *testing.T) {
	found := false
	for _, c := range uninstallCmd.Commands() {
		if c.Use == "aws-lambda" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected uninstall aws-lambda subcommand to be registered")
	}
}
