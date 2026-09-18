package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
