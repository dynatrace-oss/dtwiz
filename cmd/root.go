package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var environmentFlag string
var accessTokenFlag string
var platformTokenFlag string

var rootCmd = &cobra.Command{
	Use:   "dtwiz",
	Short: "Dynatrace Ingest CLI — analyze systems and deploy observability",
	Long: `dtwiz analyzes your system and deploys the best Dynatrace ingestion method.

Set your Dynatrace credentials via environment variables:

  export DT_ENVIRONMENT=https://<your-tenant-domain>
  export DT_ACCESS_TOKEN=dt0c01.****
  export DT_PLATFORM_TOKEN=dt0s16.****

Then use dtwiz commands to analyze and instrument your system.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&environmentFlag, "environment", "", "Dynatrace environment URL (also read from DT_ENVIRONMENT)")
	rootCmd.PersistentFlags().StringVar(&accessTokenFlag, "access-token", "", "Dynatrace API access token (also read from DT_ACCESS_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&platformTokenFlag, "platform-token", "", "Dynatrace platform token (dt0s16.*) for AWS installer (also read from DT_PLATFORM_TOKEN)")

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(recommendCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}
