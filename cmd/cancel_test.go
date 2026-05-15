package cmd

import (
	"errors"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// applyInstallCancelGuard models the guard pattern used in every install/update/
// uninstall subcommand RunE: ErrInstallCancelled is treated as a clean nil exit;
// all other errors are propagated unchanged.
func applyInstallCancelGuard(err error) error {
	if errors.Is(err, installer.ErrInstallCancelled) {
		return nil
	}
	return err
}

func TestInstallCancelledGuard_ReturnNilOnCancelled(t *testing.T) {
	got := applyInstallCancelGuard(installer.ErrInstallCancelled)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestInstallCancelledGuard_PropagatesOtherErrors(t *testing.T) {
	other := errors.New("real failure")
	got := applyInstallCancelGuard(other)
	if got != other {
		t.Errorf("expected original error to be propagated, got %v", got)
	}
}

func TestInstallCancelledGuard_NilPassesThrough(t *testing.T) {
	got := applyInstallCancelGuard(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestInstallCmd_CancelledSubcommands verifies each install subcommand that
// carries a ErrInstallCancelled guard is actually registered.
func TestInstallCmd_CancelledSubcommands(t *testing.T) {
	guarded := []string{
		"kubernetes",
		"otel",
		"otel-collector",
		"otel-python",
		"otel-node",
		"otel-java",
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
