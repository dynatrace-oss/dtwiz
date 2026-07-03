package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/azure"
	k8s "github.com/dynatrace-oss/dtwiz/pkg/installer/kubernetes"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/oneagent"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var uninstallDryRun bool
var uninstallAutoConfirm bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <method>",
	Short: "Uninstall a Dynatrace ingestion method",
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// uninstallCmd.PersistentPreRun overrides root's, so reproduce its behaviour here.
		cmd.Root().SilenceUsage = true
		logger.Init(debugFlag, verbosityFlag)
		logger.Verbose("logging: verbose")
		logger.Debug("logging: debug")
		featureflags.ApplyCLIOverrides(cmd.Flags())
		installer.AutoConfirm = uninstallAutoConfirm
	},
}

var uninstallKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "Remove Dynatrace Operator and DynaKube resources from Kubernetes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		k8sInfo := analyzer.DetectKubernetesIdentity()
		return k8s.UninstallKubernetes(k8sInfo.Context, k8sInfo.Distribution, uninstallDryRun)
	},
}

var uninstallOneAgentCmd = &cobra.Command{
	Use:   "oneagent",
	Short: "Uninstall Dynatrace OneAgent from this host",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := oneagent.UninstallOneAgentV2(oneagent.UninstallOptions{DryRun: uninstallDryRun})
		if errors.Is(err, installer.ErrInstallCancelled) {
			return nil
		}
		return err
	},
}

var uninstallAWSCmd = &cobra.Command{
	Use:   "aws",
	Short: "Remove the Dynatrace AWS CloudFormation stack and monitoring configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		c, err := setupClientFromCreds(envURL, classicTok, platformTok)
		if err != nil {
			return err
		}
		return installer.UninstallAWS(c.Platform, envURL, uninstallDryRun)
	},
}

var uninstallAWSLambdaCmd = &cobra.Command{
	Use:   "aws-lambda",
	Short: "Remove Dynatrace Lambda Layer from all functions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := installer.UninstallAWSLambda(uninstallDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		return nil // success path
	},
}

var uninstallOtelCmd = &cobra.Command{
	Use:   "otel",
	Short: "Kill running OTel Collector processes and remove installation files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := installer.UninstallOtelCollector(uninstallDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		return nil
	},
}

var uninstallAzureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Remove the Dynatrace Azure Monitor integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, _, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := azure.UninstallAzure(envURL, platformTok, uninstallDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		return nil
	},
}

var uninstallSelfCmd = &cobra.Command{
	Use:   "self",
	Short: "Remove the dtwiz binary and its PATH entry",
	Long: `Remove the dtwiz binary and the PATH entry added by the install script.

On Linux/macOS the binary is deleted and the shell profile is updated.
On Windows, ready-to-paste PowerShell commands are printed instead.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.UninstallSelf()
	},
}

func init() {
	uninstallCmd.PersistentFlags().BoolVar(&uninstallDryRun, "dry-run", false, "show what would be done without making changes")
	uninstallCmd.PersistentFlags().BoolVarP(&uninstallAutoConfirm, "yes", "y", false, "skip confirmation prompts")
	uninstallCmd.AddCommand(uninstallKubernetesCmd)
	uninstallCmd.AddCommand(uninstallOneAgentCmd)
	uninstallCmd.AddCommand(uninstallAWSCmd)
	uninstallCmd.AddCommand(uninstallAWSLambdaCmd)
	uninstallCmd.AddCommand(uninstallAzureCmd)
	uninstallCmd.AddCommand(uninstallOtelCmd)
	uninstallCmd.AddCommand(uninstallSelfCmd)
}
