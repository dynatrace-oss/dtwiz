package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

var updateDryRun bool
var updateAutoConfirm bool

var updateCmd = &cobra.Command{
	Use:   "update <method>",
	Short: "Update an existing ingestion method configuration",
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		installer.AutoConfirm = updateAutoConfirm
	},
}

var updateOtelConfigPath string
var updateOtelCmd = &cobra.Command{
	Use:   "otel",
	Short: "Patch an existing OTel Collector config with the Dynatrace exporter",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		if err := installer.UpdateOtelConfig(updateOtelConfigPath, envURL, classicTok, updateDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		return nil
	},
}

func init() {
	updateCmd.PersistentFlags().BoolVar(&updateDryRun, "dry-run", false, "show what would be done without executing")
	updateCmd.PersistentFlags().BoolVarP(&updateAutoConfirm, "yes", "y", false, "skip confirmation prompts")
	updateOtelCmd.Flags().StringVar(&updateOtelConfigPath, "config", "config.yaml", "path to the existing OTel Collector config file to patch")
	updateCmd.AddCommand(updateOtelCmd)
}
