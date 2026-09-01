package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	awspkg "github.com/dynatrace-oss/dtwiz/pkg/installer/aws"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/azure"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/gcp"
	k8s "github.com/dynatrace-oss/dtwiz/pkg/installer/kubernetes"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/oneagent"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
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
		fmt.Printf(" Learn about needed permissions? %s\n\n", display.Hyperlink("Visit docs", "https://github.com/dynatrace-oss/dtwiz/blob/main/README.md"))

		display.Header("Analyzing system...")

		// Start demo-running check concurrently with system analysis — they are independent.
		demoRunningCh := make(chan bool, 1)
		go func() { demoRunningCh <- otel.IsDemoRunning() }()

		info, err := analyzeSystem()
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		demoRunning := <-demoRunningCh

		display.Header("Recommendations — What do you want to monitor?")
		fmt.Println("  Monitor Logs, Metrics, Traces of:")
		fmt.Println()
		recs := recommender.GenerateRecommendations(info)
		actionable := recommender.ActionableItems(recs, featureflags.IsEnabled(featureflags.Experimental))

		if len(actionable) == 0 {
			return nil
		}

		fmt.Print(recommender.FormatSetupMenu(recs, demoRunning, featureflags.IsEnabled(featureflags.Experimental)))
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

		if input == "d" {
			fmt.Println()

			envURL, accessTok, platformTok, err := getDtEnvironment()
			if err != nil {
				return err
			}
			classicTok, err := validateCredentials(envURL, accessTok, platformTok)
			if err != nil {
				return err
			}

			display.Header("Installing: Demo app (schnitzel)")
			if err := otel.InstallDemo(envURL, classicTok, platformTok, setupDryRun); err != nil {
				if errors.Is(err, installer.ErrInstallCancelled) {
					return nil
				}
				return err
			}
			if !setupDryRun {
				installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
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
		if selected.Method == recommender.MethodAzureUpdate || selected.Method == recommender.MethodGCPUpdate {
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
				MonitoringMode: string(oneagent.InstallModeFullStack),
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
			installErr = otel.InstallOtelCollector(envURL, classicTok, platformTok, setupDryRun)
		case recommender.MethodOtelUpdate:
			installErr = otel.UpdateOtelConfigInteractive(envURL, classicTok, platformTok, setupDryRun)
		case recommender.MethodAWS:
			installErr = awspkg.InstallAWS(envURL, platformTok, setupDryRun, StartTime.UTC().Format(installer.IngestTimeFormat))
		case recommender.MethodAzure:
			installErr = azure.InstallAzure(envURL, platformTok, setupDryRun, StartTime)
		case recommender.MethodAzureUpdate:
			installErr = azure.UpdateAzure(envURL, platformTok, setupDryRun, StartTime)
		case recommender.MethodGCP:
			installErr = gcp.InstallGCP(envURL, platformTok, setupDryRun, StartTime)
		case recommender.MethodGCPUpdate:
			installErr = gcp.UpdateGCP(envURL, platformTok, setupDryRun, StartTime)
		default:
			return fmt.Errorf("unsupported method: %s", selected.Method)
		}
		if installErr != nil {
			if errors.Is(installErr, installer.ErrInstallCancelled) || errors.Is(installErr, otel.ErrUpToDate) {
				return nil
			}
			return installErr
		}
		if setupDryRun {
			return nil
		}
		// AWS scopes its watch to the account (WatchIngestAWS); Azure and GCP run their
		// own generic watch from inside the installer. Only the methods below use the
		// generic post-install watch.
		switch selected.Method {
		case recommender.MethodOneAgent,
			recommender.MethodKubernetes,
			recommender.MethodDocker,
			recommender.MethodOtelCollector,
			recommender.MethodOtelUpdate:
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
		}
		return nil
	},
}

func init() {
	setupCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "show what would be done without executing")
}
