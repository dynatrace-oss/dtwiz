package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dietermayrhofer/dtwiz/pkg/analyzer"
	"github.com/spf13/cobra"
)

var analyzeJSON bool

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze the current system for observability configuration",
	Long:  `Detect platform, container runtime, orchestration, existing agents, cloud providers, and running services.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := analyzer.AnalyzeSystem()
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		if analyzeJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}

		fmt.Println(info.Summary())
		return nil
	},
}

func init() {
	analyzeCmd.Flags().BoolVar(&analyzeJSON, "json", false, "output analysis as JSON")
}
