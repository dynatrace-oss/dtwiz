package cmd

import (
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
)

// TestUninstallOneAgentCmd_CLIFlagEnablesOneAgentPoC ensures that passing
// --oneagent-poc as a CLI flag (not just the env var) activates the V2 path
// for `dtwiz uninstall oneagent`. This regresses if uninstallCmd.PersistentPreRun
// stops calling featureflags.ApplyCLIOverrides.
func TestUninstallOneAgentCmd_CLIFlagEnablesOneAgentPoC(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.OneAgentPoC)

	// InheritedFlags forces cobra to merge rootCmd persistent flags (where
	// featureflags.RegisterFlags registers --oneagent-poc) into uninstallCmd.Flags().
	// Without this call, uninstallCmd.Flags() only contains the command's own flags.
	uninstallCmd.InheritedFlags()

	f := uninstallCmd.Flags().Lookup("oneagent-poc")
	if f == nil {
		t.Fatal("--oneagent-poc flag not found in uninstallCmd.Flags() after merge")
	}
	prevChanged := f.Changed
	if err := f.Value.Set("true"); err != nil {
		t.Fatalf("failed to set --oneagent-poc: %v", err)
	}
	f.Changed = true
	t.Cleanup(func() {
		_ = f.Value.Set("false")
		f.Changed = prevChanged
	})

	// cobra calls PersistentPreRun with the leaf subcommand as cmd; we pass
	// uninstallCmd itself which also has the merged flags.
	uninstallCmd.PersistentPreRun(uninstallCmd, nil)

	if !featureflags.IsEnabled(featureflags.OneAgentPoC) {
		t.Error("--oneagent-poc CLI flag did not enable OneAgentPoC after PersistentPreRun; " +
			"uninstallCmd.PersistentPreRun must call featureflags.ApplyCLIOverrides")
	}
}
