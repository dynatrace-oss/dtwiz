package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/recommender"
	"github.com/spf13/cobra"
)

var recommendJSON bool

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Recommend the best Dynatrace ingestion methods for this system",
	Long:  `Analyzes the system and generates ranked recommendations with priorities, prerequisites, and steps.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := analyzer.AnalyzeSystem()
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		recs := recommender.GenerateRecommendations(info)

		if recommendJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(recs)
		}

		fmt.Println(recommender.FormatRecommendations(recs))
		return nil
	},
}

func init() {
	recommendCmd.Flags().BoolVar(&recommendJSON, "json", false, "output recommendations as JSON")
}
