package cmd

import (
	"fmt"
	"os"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

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
  export DT_ACCESS_TOKEN=dt0c01.****
  export DT_PLATFORM_TOKEN=dt0s16.****

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
	purple.Printf(" |____/    |_|       \\_/\\_/   |___|/____| %s\n", Version)
	fmt.Printf("\n HASTA LA VISTA - BLIND SPOTS!\n\n")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	rootCmd.PersistentFlags().StringVar(&accessTokenFlag, "access-token", "", "Dynatrace API access token (also read from DT_ACCESS_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&platformTokenFlag, "platform-token", "", "Dynatrace platform token (dt0s16.*) for AWS installer (also read from DT_PLATFORM_TOKEN)")

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
