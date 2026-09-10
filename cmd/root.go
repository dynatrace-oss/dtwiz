package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

// StartTime is the time when dtwiz was started.
var StartTime time.Time

var debugFlag bool
var verbosityFlag int
var environmentFlag string
var accessTokenFlag string
var platformTokenFlag string

var rootCmd = &cobra.Command{
	Use:   "dtwiz",
	Short: "Dynatrace Ingest CLI — analyze systems and deploy observability",
	Long: `dtwiz analyzes your system and deploys the best Dynatrace ingestion method.

Set your Dynatrace credentials via environment variables:

  export DT_ENVIRONMENT=https://<your-tenant-domain>
  export DT_PLATFORM_TOKEN=dt0s16.****

For legacy environments you can opt into a Classic API access token by passing
--access-token explicitly (it is intentionally not read from the environment).

Then use dtwiz commands to analyze and instrument your system.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(debugFlag, verbosityFlag)
		logger.Verbose("logging: verbose")
		logger.Debug("logging: debug")

		featureflags.ApplyCLIOverrides(cmd.Flags())
		fireSelfMonitoringEvent(cmd, selfmonitoring.StepInvoked)
	},
}

func fireSelfMonitoringEvent(cmd *cobra.Command, stepID string) {
	if !featureflags.IsEnabled(featureflags.SelfMonitoringPoC) {
		return
	}
	params := buildEventParams(cmd, stepID)
	go func() {
		envURL, _, platformTok, err := getDtEnvironment()
		if err != nil {
			logger.Debug(fmt.Sprintf("selfmonitoring: could not resolve credentials: %v", err))
			return
		}
		if err := selfmonitoring.SendEvent(installer.APIURL(envURL), platformTok, params); err != nil {
			logger.Debug(fmt.Sprintf("selfmonitoring: %v", err))
		}
	}()
}

func buildEventParams(cmd *cobra.Command, stepID string) selfmonitoring.EventParams {
	cmdID, subID := deriveCommandIDs(cmd)
	return selfmonitoring.EventParams{
		CmdID:  cmdID,
		SubID:  subID,
		StepID: stepID,
		Mode:   resolveMode(),
	}
}

var normCmdMap = map[string]string{
	"install":   "ins",
	"uninstall": "uni",
	"update":    "upd",
	"analyze":   "ana",
	"recommend": "rec",
	"status":    "sta",
	"watch":     "wch",
	"setup":     "set",
	"version":   "ver",
}

var normSubMap = map[string]string{
	"otel":           "otel",
	"otel-collector": "otlc",
	"otel-python":    "otlp",
	"otel-node":      "otln",
	"otel-java":      "otlj",
	"kubernetes":     "k8s",
	"oneagent":       "oa",
	"gcp":            "gcp",
	"azure":          "az",
	"aws":            "aws",
	"aws-lambda":     "awsl",
	"docker":         "dock",
	"demo":           "demo",
	"self":           "self",
}

func deriveCommandIDs(cmd *cobra.Command) (cmdID, subID string) {
	parent := cmd.Parent()
	if parent == nil || parent.Name() == "dtwiz" {
		return normCmd(cmd.Name()), ""
	}
	return normCmd(parent.Name()), normSub(cmd.Name())
}

func normCmd(name string) string {
	if short, ok := normCmdMap[name]; ok {
		return short
	}
	return name
}

func normSub(name string) string {
	if short, ok := normSubMap[name]; ok {
		return short
	}
	return name
}

func resolveMode() string {
	if debugFlag {
		return "deb"
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return "tty"
	}
	return "ntt"
}

func printBanner() {
	purple := color.New(color.FgMagenta, color.Bold)
	purple.Printf("  ____   _______  __        __ ___  _____\n")
	purple.Printf(" |  _ \\ |__   __| \\ \\      / /|_ _||__  /\n")
	purple.Printf(" | | | |   | |     \\ \\ /\\ / /  | |   / / \n")
	purple.Printf(" | |_| |   | |      \\ V  V /   | |  / /_ \n")
	purple.Printf(" |____/    |_|       \\_/\\_/   |___|/____| %s\n", version.Version)
	fmt.Printf("\n HASTA LA VISTA - BLIND SPOTS!\n\n")
}

// fireSelfMonitoringWatchComplete sends the watch completion event synchronously
// with a 500ms cap so it does not delay the command exit noticeably.
func fireSelfMonitoringWatchComplete(cmd *cobra.Command, result installer.WatchSessionResult) {
	if !featureflags.IsEnabled(featureflags.SelfMonitoringPoC) {
		logger.Debug("selfmonitoring: skipped (self-monitoring-poc not enabled)")
		return
	}
	params := buildEventParams(cmd, selfmonitoring.StepCompleted)
	params.ExtraProps = watchResultToProps(result)

	done := make(chan struct{})
	go func() {
		defer close(done)
		envURL, _, platformTok, err := getDtEnvironment()
		if err != nil {
			logger.Debug(fmt.Sprintf("selfmonitoring: could not resolve credentials: %v", err))
			return
		}
		if err := selfmonitoring.SendEvent(installer.APIURL(envURL), platformTok, params); err != nil {
			logger.Debug(fmt.Sprintf("selfmonitoring: watch event failed: %v", err))
		} else {
			logger.Debug("selfmonitoring: watch event sent")
		}
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		logger.Debug("selfmonitoring: watch event flush timed out (500ms), event may be lost")
	}
}

func watchResultToProps(result installer.WatchSessionResult) map[string]string {
	props := map[string]string{
		"watch.dur":  strconv.FormatInt(result.Duration.Milliseconds(), 10),
		"watch.exit": result.ExitReason,
	}
	signals := make([]string, 0, len(result.FirstDataMs))
	for sig, ms := range result.FirstDataMs {
		signals = append(signals, sig)
		props["watch.t_"+sig] = strconv.FormatInt(ms, 10)
	}
	sort.Strings(signals)
	props["watch.sig"] = strings.Join(signals, ",")
	return props
}

// setupClientFromCreds creates a Dynatrace API client from already-resolved credentials.
func setupClientFromCreds(envURL, classicTok, platformTok string) (*client.Client, error) {
	level := verbosityFlag
	if debugFlag {
		level = 2
	}
	return client.New(installer.APIURL(envURL), installer.AppsURL(envURL), classicTok, platformTok, level)
}

// setupClient creates a Dynatrace API client by resolving and validating credentials.
func setupClient() (*client.Client, error) {
	envURL, accessTok, platformTok, err := getDtEnvironment()
	if err != nil {
		return nil, err
	}
	classicTok, err := validateCredentials(envURL, accessTok, platformTok)
	if err != nil {
		return nil, err
	}
	return setupClientFromCreds(envURL, classicTok, platformTok)
}

// Execute runs the root command.
func Execute(t time.Time) {
	StartTime = t
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Show banner when no subcommand is given or --help is used on the root command.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == rootCmd {
			printBanner()
		}
		defaultHelp(cmd, args)
	})
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	}

	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().CountVarP(&verbosityFlag, "verbose", "v", "verbose output")
	rootCmd.PersistentFlags().StringVar(&environmentFlag, "environment", "", "Dynatrace environment URL (also read from DT_ENVIRONMENT)")
	rootCmd.PersistentFlags().StringVar(&platformTokenFlag, "platform-token", "", "Dynatrace platform token (also read from DT_PLATFORM_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&accessTokenFlag, "access-token", "", "Dynatrace API access token for legacy environments (opt-in; must be passed explicitly — not read from the environment variables)")

	featureflags.RegisterFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(recommendCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}
