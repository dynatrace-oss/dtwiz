package cmd

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/dietermayrhofer/dtwiz/pkg/analyzer"
	"github.com/dietermayrhofer/dtwiz/pkg/installer"
	"github.com/dietermayrhofer/dtwiz/pkg/recommender"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var setupDryRun bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup — analyze, recommend, and install the best ingestion method",
	Long: `Runs a full interactive workflow:
  1. Analyzes the current system
  2. Generates ranked recommendations
  3. Prompts you to pick a method
  4. Runs the selected installer`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		setupHeader := color.New(color.FgMagenta, color.Bold)
		setupMuted := color.New()
		setupPrompt := color.New(color.FgMagenta)
		setupBadge := color.New(color.FgMagenta, color.Bold)

		setupHeader.Println("  Analyzing system...")
		setupMuted.Println("  " + strings.Repeat("─", 42))
		info, err := analyzer.AnalyzeSystem()
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
		fmt.Println(info.Summary())

		fmt.Println()
		setupHeader.Println("  Recommendations — select an installation method:")
		setupMuted.Println("  " + strings.Repeat("─", 42))
		recs := recommender.GenerateRecommendations(info)

		// Collect actionable (non-done, non-not-supported) recommendations.
		var actionable []recommender.Recommendation
		for _, r := range recs {
			if !r.Done && r.Method != recommender.MethodNotSupported {
				actionable = append(actionable, r)
			}
		}

		if len(actionable) == 0 {
			return nil
		}

		for i, r := range actionable {
		fmt.Printf("  %s  %s\n", setupBadge.Sprintf("[%d]", i+1), r.Title)
		}
		fmt.Printf("  %s  %s\n", setupMuted.Sprint("[0]"), setupMuted.Sprint("Cancel"))
		fmt.Println()
		setupPrompt.Print("  Enter number: ")

		reader := bufio.NewReader(cmd.InOrStdin())
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)

		if input == "" || input == "0" {
			setupMuted.Println("  Setup cancelled.")
			return nil
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(actionable) {
			return fmt.Errorf("invalid selection: %q", input)
		}

		selected := actionable[choice-1]
		fmt.Println()
		setupHeader.Printf("  Installing: %s\n", selected.Title)
		setupMuted.Println("  " + strings.Repeat("─", 42))

		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}

		switch selected.Method {
		case recommender.MethodOneAgent:
			return installer.InstallOneAgent(envURL, token, setupDryRun, false, "")
		case recommender.MethodKubernetes:
			return installer.InstallKubernetes(envURL, token, accessToken(), "" /* name */, setupDryRun)
		case recommender.MethodDocker:
			return installer.InstallDocker(envURL, token, setupDryRun)
		case recommender.MethodOtelCollector:
			return installer.InstallOtelCollector(envURL, token, accessToken(), platformToken(), setupDryRun)
		case recommender.MethodOtelUpdate:
			cfgPath := selected.ConfigPath
			if cfgPath == "" {
				cfgPath = "config.yaml" // fall back to CWD default
			}
			return installer.UpdateOtelConfig(cfgPath, envURL, token, platformToken(), setupDryRun)
		case recommender.MethodAWS:
			return installer.InstallAWS(envURL, token, platformToken(), setupDryRun)
		default:
			return fmt.Errorf("unsupported method: %s", selected.Method)
		}
	},
}

func init() {
	setupCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "show what would be done without executing")
}
