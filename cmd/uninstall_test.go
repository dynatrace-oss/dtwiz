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
