package cmd

import (
	"github.com/dietermayrhofer/dtwiz/pkg/installer"
	"github.com/spf13/cobra"
)

var updateDryRun bool

var updateCmd = &cobra.Command{
	Use:   "update <method>",
	Short: "Update an existing ingestion method configuration",
	Args:  cobra.MinimumNArgs(1),
}

var updateOtelConfigPath string
var updateOtelCmd = &cobra.Command{
	Use:   "otel",
	Short: "Patch an existing OTel Collector config with the Dynatrace exporter",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.UpdateOtelConfig(updateOtelConfigPath, envURL, token, platformToken(), updateDryRun)
	},
}

func init() {
	updateCmd.PersistentFlags().BoolVar(&updateDryRun, "dry-run", false, "show what would be done without executing")
	updateOtelCmd.Flags().StringVar(&updateOtelConfigPath, "config", "config.yaml", "path to the existing OTel Collector config file to patch")
	updateCmd.AddCommand(updateOtelCmd)
}
