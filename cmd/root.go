package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
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
	},
}

func printBanner() {
	purple := color.New(color.FgMagenta, color.Bold)
	purple.Printf("  ____   _______   __        __ ___  ____\n")
	purple.Printf(" |  _ \\ |__   __| \\ \\      / /|_ _||_  /\n")
	purple.Printf(" | | | |   | |     \\ \\ /\\ / /  | |  / / \n")
	purple.Printf(" | |_| |   | |      \\ V  V /   | | / /_ \n")
	purple.Printf(" |____/    |_|       \\_/\\_/   |___|/____| %s\n", version.Version)
	fmt.Printf("\n HASTA LA VISTA - BLIND SPOTS!\n\n")
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
