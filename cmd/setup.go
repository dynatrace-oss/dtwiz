package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/azure"
	k8s "github.com/dynatrace-oss/dtwiz/pkg/installer/kubernetes"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/oneagent"
	"github.com/dynatrace-oss/dtwiz/pkg/recommender"
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

		if env := environmentHint(); env != "" {
			display.ColorDefault.Printf(" Environment: %s\n\n", env)
		} else {
			display.ColorDefault.Println(" Environment: (not configured)")
			fmt.Println()
		}

		fmt.Printf(" Looking for more deployment options beyond these popular ones? %s\n\n", display.Hyperlink("Visit docs", "https://docs.dynatrace.com/docs/ingest-from"))

		display.Header("Analyzing system...")

		info, err := analyzer.AnalyzeSystem()
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
		fmt.Println(info.Summary())

		fmt.Println()
		display.Header("Recommendations — What do you want to monitor?")
		recs := recommender.GenerateRecommendations(info)

		// Collect actionable (non-done, non-not-supported, non-coming-soon) recommendations.
		var actionable []recommender.Recommendation
		for _, r := range recs {
			if !r.Done && r.Method != recommender.MethodNotSupported && !r.ComingSoon {
				if !featureflags.IsEnabled(featureflags.Experimental) &&
					(r.Method == recommender.MethodDocker || r.Method == recommender.MethodOtelUpdate) {
					continue
				}
				actionable = append(actionable, r)
			}
		}

		if len(actionable) == 0 {
			return nil
		}

		// Pre-check Azure status so we can badge the Azure entry in the list.
		azureConfigured := false
		if envURL, _, platformTok, credErr := getDtEnvironment(); credErr == nil {
			if exists, _ := azure.ConnectionExists(envURL, platformTok); exists {
				azureConfigured = true
			}
		}

		for i, r := range actionable {
			title := r.Title
			if r.Method == recommender.MethodAzure {
				if azureConfigured {
					title += "  [update]"
				} else {
					title += "  [install]"
				}
			}
			fmt.Printf("  %s  %s\n", display.ColorHeader.Sprintf("[%d]", i+1), title)
		}
		// Show coming-soon items (informational only, not selectable).
		for _, r := range recs {
			if r.ComingSoon {
				fmt.Printf("  %s  %s\n", display.ColorDefault.Sprint(" · "), display.ColorDefault.Sprint(r.Title))
			}
		}
		if featureflags.IsEnabled(featureflags.Experimental) {
			fmt.Println()
			fmt.Printf("  %s  %s\n", display.ColorDefault.Sprint("[d]"), display.ColorDefault.Sprint("Install demo app (schnitzel)"))
		}
		fmt.Println()
		fmt.Printf("  %s  %s\n", display.ColorDefault.Sprint("[u]"), display.ColorDefault.Sprint("Show uninstall commands"))
		fmt.Printf("  %s  %s\n", display.ColorDefault.Sprint("[0]"), display.ColorDefault.Sprint("Cancel"))
		fmt.Println()
		display.ColorMessage.Print("  Enter selection: ")

		reader := bufio.NewReader(cmd.InOrStdin())
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)

		if input == "" || input == "0" {
			display.ColorDefault.Println("  Setup cancelled.")
			return nil
		}

		if input == "u" {
			fmt.Println()
			return uninstallCmd.Help()
		}

		if input == "d" && featureflags.IsEnabled(featureflags.Experimental) {
			fmt.Println()
			display.Header("Installing: Demo app (schnitzel)")

			envURL, accessTok, platformTok, err := getDtEnvironment()
			if err != nil {
				return err
			}
			classicTok, err := validateCredentials(envURL, accessTok, platformTok)
			if err != nil {
				return err
			}
			if err := installer.InstallDemo(envURL, classicTok, platformTok, setupDryRun); err != nil {
				if errors.Is(err, installer.ErrInstallCancelled) {
					return nil
				}
				return err
			}
			if !setupDryRun {
				installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
			}
			return nil
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(actionable) {
			return fmt.Errorf("invalid selection: %q", input)
		}

		selected := actionable[choice-1]
		fmt.Println()

		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}

		headerVerb := "Installing"
		if selected.Method == recommender.MethodAzure && azureConfigured {
			headerVerb = "Updating"
		}
		display.Header(fmt.Sprintf("%s: %s", headerVerb, selected.Title))

		c, err := setupClientFromCreds(envURL, classicTok, platformTok)
		if err != nil {
			return err
		}

		var installErr error
		switch selected.Method {
		case recommender.MethodOneAgent:
			installErr = oneagent.InstallOneAgentV2(c, oneagent.InstallOptions{
				DryRun:         setupDryRun,
				MonitoringMode: string(installer.InstallModeFullStack),
			})
		case recommender.MethodKubernetes:
			k8sClusterName := ""
			k8sDistro := ""
			if info.Kubernetes.Available {
				k8sClusterName = info.Kubernetes.Cluster
				k8sDistro = info.Kubernetes.Distribution
			}
			installErr = k8s.InstallKubernetes(envURL, classicTok, k8sClusterName, k8sDistro, setupDryRun)
		case recommender.MethodDocker:
			installErr = installer.InstallDocker(envURL, classicTok, setupDryRun)
		case recommender.MethodOtelCollector:
			installErr = installer.InstallOtelCollector(envURL, classicTok, platformTok, setupDryRun)
		case recommender.MethodOtelUpdate:
			installErr = installer.UpdateOtelConfigInteractive(envURL, classicTok, platformTok, setupDryRun)
		case recommender.MethodAWS:
			installErr = installer.InstallAWS(c.Platform, envURL, platformTok, setupDryRun, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		case recommender.MethodAzure:
			if azureConfigured {
				installErr = azure.UpdateAzure(envURL, platformTok, setupDryRun, StartTime)
			} else {
				installErr = azure.InstallAzure(envURL, platformTok, setupDryRun, StartTime)
			}
		default:
			return fmt.Errorf("unsupported method: %s", selected.Method)
		}
		if installErr != nil {
			if errors.Is(installErr, installer.ErrInstallCancelled) {
				return nil
			}
			return installErr
		}
		// AWS scopes its watch to the account (WatchIngestAWS) and Azure runs the
		// generic watch from inside the installer; both start their own watch, so
		// the generic post-install watch here is only used for the other methods.
		if !setupDryRun && selected.Method != recommender.MethodAWS && selected.Method != recommender.MethodAzure {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

func init() {
	setupCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "show what would be done without executing")
}
