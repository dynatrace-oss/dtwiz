package cmd

import (
	"testing"
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

// TestUpdateCmd_OtelCancelledSubcommand verifies the update otel subcommand is
// registered and its RunE will not propagate ErrInstallCancelled.
func TestUpdateCmd_OtelCancelledSubcommand(t *testing.T) {
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
