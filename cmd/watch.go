package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

var watchFromFlag string

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

		fromClause := watchFromFlag
		if fromClause == "" {
			fromClause = StartTime.UTC().Format("2006-01-02T15:04:05Z")
		}

		installer.WatchIngest(envURL, pTok, fromClause)
	},
}

func init() {
	watchCmd.Flags().StringVar(&watchFromFlag, "from", "", `start time for queries — RFC3339 (e.g. "2026-04-21T14:30:05Z") or DQL relative (e.g. "now()-1h"); defaults to dtwiz start time`)
	rootCmd.AddCommand(watchCmd)
}
