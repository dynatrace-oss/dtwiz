package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var updateDryRun bool
var updateAutoConfirm bool

var updateCmd = &cobra.Command{
	Use:   "update <method>",
	Short: "Update an existing ingestion method configuration",
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// updateCmd.PersistentPreRun overrides root's, so reproduce its behaviour here.
		logger.Init(debugFlag, verbosityFlag)
		logger.Verbose("logging: verbose")
		logger.Debug("logging: debug")
		featureflags.ApplyCLIOverrides(cmd.Flags())
		installer.AutoConfirm = updateAutoConfirm
	},
}

var updateOtelConfigPath string
var updateOtelCmd = &cobra.Command{
	Use:    "otel",
	Short:  "Patch an existing OTel Collector config with the Dynatrace exporter",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !featureflags.IsEnabled(featureflags.Experimental) {
			return fmt.Errorf("otel update is an experimental feature; enable it with --experimental or DTWIZ_EXPERIMENTAL=true")
		}
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		var updateErr error
		if updateOtelConfigPath != "" {
			updateErr = installer.UpdateOtelConfig(updateOtelConfigPath, envURL, classicTok, platformTok, updateDryRun)
		} else {
			updateErr = installer.UpdateOtelConfigInteractive(envURL, classicTok, platformTok, updateDryRun)
		}
		if errors.Is(updateErr, installer.ErrInstallCancelled) {
			return nil
		}
		if updateErr != nil {
			return updateErr
		}
		return nil
	},
}

func init() {
	updateCmd.PersistentFlags().BoolVar(&updateDryRun, "dry-run", false, "show what would be done without executing")
	updateCmd.PersistentFlags().BoolVarP(&updateAutoConfirm, "yes", "y", false, "skip confirmation prompts")
	updateOtelCmd.Flags().StringVar(&updateOtelConfigPath, "config", "", "path to the existing OTel Collector config file to patch (prompts with running collector list when omitted)")
	updateCmd.AddCommand(updateOtelCmd)

	// PersistentPreRun doesn't run for --help, so resolve the experimental flag here
	// to conditionally show the otel subcommand in help output.
	defaultUpdateHelp := updateCmd.HelpFunc()
	updateCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		featureflags.ApplyCLIOverrides(cmd.Flags())
		updateOtelCmd.Hidden = !featureflags.IsEnabled(featureflags.Experimental)
		defaultUpdateHelp(cmd, args)
	})
}
