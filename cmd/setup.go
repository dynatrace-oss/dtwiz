package cmd

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/recommender"
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
		printBanner()
		setupHeader := color.New(color.FgMagenta, color.Bold)
		setupMuted := color.New()
		setupPrompt := color.New(color.FgMagenta)
		setupBadge := color.New(color.FgMagenta, color.Bold)

		if env := environmentHint(); env != "" {
			setupMuted.Printf(" Environment: %s\n\n", env)
		} else {
			setupMuted.Println(" Environment: (not configured)")
			fmt.Println()
		}

		setupHeader.Println("  Analyzing system...")
		setupMuted.Println("  " + strings.Repeat("─", 42))
		info, err := analyzer.AnalyzeSystem()
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
		fmt.Println(info.Summary())

		fmt.Println()
		setupHeader.Println("  Recommendations — What do you want to monitor?")
		setupMuted.Println("  " + strings.Repeat("─", 42))
		recs := recommender.GenerateRecommendations(info)

		// Collect actionable (non-done, non-not-supported, non-coming-soon) recommendations.
		var actionable []recommender.Recommendation
		for _, r := range recs {
			if !r.Done && r.Method != recommender.MethodNotSupported && !r.ComingSoon {
				actionable = append(actionable, r)
			}
		}

		if len(actionable) == 0 {
			return nil
		}

		for i, r := range actionable {
			fmt.Printf("  %s  %s\n", setupBadge.Sprintf("[%d]", i+1), r.Title)
		}
		// Show coming-soon items (informational only, not selectable).
		for _, r := range recs {
			if r.ComingSoon {
				fmt.Printf("  %s  %s\n", setupMuted.Sprint(" · "), setupMuted.Sprint(r.Title))
			}
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

		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}

		var installErr error
		switch selected.Method {
		case recommender.MethodOneAgent:
			installErr = installer.InstallOneAgent(envURL, accessTok, setupDryRun, false, "")
		case recommender.MethodKubernetes:
			installErr = installer.InstallKubernetes(envURL, accessTok, accessTok, "" /* name */, setupDryRun)
		case recommender.MethodDocker:
			installErr = installer.InstallDocker(envURL, accessTok, setupDryRun)
		case recommender.MethodOtelCollector:
			installErr = installer.InstallOtelCollector(envURL, accessTok, accessTok, platformTok, setupDryRun)
		case recommender.MethodOtelUpdate:
			cfgPath := selected.ConfigPath
			if cfgPath == "" {
				cfgPath = "config.yaml" // fall back to CWD default
			}
			installErr = installer.UpdateOtelConfig(cfgPath, envURL, accessTok, platformTok, setupDryRun)
		case recommender.MethodAWS:
			installErr = installer.InstallAWS(envURL, accessTok, platformTok, setupDryRun, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		default:
			return fmt.Errorf("unsupported method: %s", selected.Method)
		}
		if installErr != nil {
			return installErr
		}
		// AWS watch is started inside InstallAWS (runs in parallel with deploy).
		if !setupDryRun && selected.Method != recommender.MethodAWS {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

func init() {
	setupCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "show what would be done without executing")
}
