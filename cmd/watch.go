package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
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
			fromClause = StartTime.UTC().Format(installer.IngestTimeFormat)
		}

		installer.WatchIngestWithEvent(envURL, pTok, fromClause, func(r installer.WatchSessionResult) {
			params := buildEventParams(cmd, selfmonitoring.StepCompleted)
			params.CmdID = ""
			params.Type = watchSignalCSV(r.FirstDataMs)
			fireSelfMonitoringEvent(params)
		})
	},
}

// watchSignalOrder is the fixed positional order used by watchSignalCSV (alphabetical).
// Position in this slice determines position in the encoded t= value.
var watchSignalOrder = []string{"cld", "exc", "hst", "k8s", "log", "rel", "req", "svc"}

// watchSignalCSV encodes time-to-first-data in a positional format.
// Format: "0,0,12,5,0,9,0,3" — one value per signal in watchSignalOrder.
// 0 = signal not seen; ≥1 = whole seconds to first data (minimum 1, even if sub-second).
// Returns "" when no signals were seen (t= field is then omitted from the header).
func watchSignalCSV(firstDataMs map[string]int64) string {
	if len(firstDataMs) == 0 {
		return ""
	}
	parts := make([]string, len(watchSignalOrder))
	for i, sig := range watchSignalOrder {
		if ms, ok := firstDataMs[sig]; ok {
			if secs := ms / 1000; secs > 0 {
				parts[i] = strconv.FormatInt(secs, 10)
			} else {
				parts[i] = "1" // present but sub-second — distinguish from absent (0)
			}
		} else {
			parts[i] = "0"
		}
	}
	return strings.Join(parts, ",")
}

func init() {
	watchCmd.Flags().StringVar(&watchFromFlag, "from", "", `start time for queries — RFC3339 (e.g. "2026-04-21T14:30:05Z") or DQL relative (e.g. "now()-1h"); defaults to dtwiz start time`)
	rootCmd.AddCommand(watchCmd)
}
