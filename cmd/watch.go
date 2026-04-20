package cmd

import (
	"fmt"
	"os"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for new data arriving in Dynatrace",
	Long:  `Polls Dynatrace every 5 seconds and displays a live summary of newly ingested data including services, cloud resources, Kubernetes entities, logs, requests, and exceptions.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		envURL := environmentHint()
		if envURL == "" {
			fmt.Fprintln(os.Stderr, "no Dynatrace environment URL configured\n\nSet one with --environment or the DT_ENVIRONMENT env var:\n  export DT_ENVIRONMENT=https://<your-env>.dynatracelabs.com/")
			os.Exit(1)
		}

		pTok := platformToken()
		if pTok == "" {
			fmt.Fprintln(os.Stderr, "no Dynatrace platform token configured\n\nSet one with --platform-token or the DT_PLATFORM_TOKEN env var:\n  export DT_PLATFORM_TOKEN=dt0s16.****")
			os.Exit(1)
		}

		installer.WatchIngest(envURL, pTok)
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
