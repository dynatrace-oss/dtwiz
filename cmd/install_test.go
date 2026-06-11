package cmd

import (
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
