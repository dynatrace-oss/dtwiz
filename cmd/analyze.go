package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/azure"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/gcp"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var analyzeJSON bool

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze the current system for observability configuration",
	Long:  `Detect platform, container runtime, orchestration, existing agents, cloud providers, and running services.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := analyzeSystem()
		if err != nil {
			err = fmt.Errorf("analysis failed: %w", err)
			fireCompletedEvent(cmd, err)
			return err
		}

		if analyzeJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			err = enc.Encode(info)
			fireCompletedEvent(cmd, err)
			return err
		}

		fmt.Println(info.Summary())
		fireCompletedEvent(cmd, nil)
		return nil
	},
}

func init() {
	analyzeCmd.Flags().BoolVar(&analyzeJSON, "json", false, "output analysis as JSON")
}

// analyzeSystem runs AnalyzeSystem and enriches the result with cloud connection
// status. All cmd callers should use this instead of calling AnalyzeSystem directly.
func analyzeSystem() (*analyzer.SystemInfo, error) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		return nil, err
	}
	if envURL, _, platformTok, credErr := getDtEnvironment(); credErr == nil {
		var checks []func() error
		if info.Azure != nil && info.Azure.Available {
			checks = append(checks, func() error {
				exists, err := azure.ConnectionExists(envURL, platformTok)
				info.AzureConfigured = exists
				return err
			})
		}
		if info.GCP != nil && info.GCP.Available {
			checks = append(checks, func() error {
				exists, err := gcp.ConnectionExists(envURL, platformTok)
				info.GCPConfigured = exists
				return err
			})
		}
		if len(checks) > 0 {
			if err := installer.RunConcurrently(checks...); err != nil {
				logger.Debug("connection existence check failed, assuming not configured", "err", err)
			}
		}
	}
	return info, nil
}
