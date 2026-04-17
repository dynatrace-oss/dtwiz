package cmd

import (
	"testing"
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
