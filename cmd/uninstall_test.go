package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestUninstallCloudCmds_RunE_ValidatePlatformToken(t *testing.T) {
	for _, c := range []*cobra.Command{uninstallAWSCmd, uninstallAzureCmd, uninstallGCPCmd} {
		t.Run(c.Use, func(t *testing.T) { assertRunEValidatesPlatformToken(t, c) })
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
