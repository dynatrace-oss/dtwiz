package cmd

import (
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/spf13/cobra"
)

var uninstallDryRun bool
var uninstallAutoConfirm bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <method>",
	Short: "Uninstall a Dynatrace ingestion method",
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		installer.AutoConfirm = uninstallAutoConfirm
	},
}

var uninstallKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "Remove Dynatrace Operator and DynaKube resources from Kubernetes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.UninstallKubernetes()
	},
}

var uninstallOneAgentCmd = &cobra.Command{
	Use:   "oneagent",
	Short: "Uninstall Dynatrace OneAgent from this host",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.UninstallOneAgent(uninstallDryRun)
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
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		return installer.UninstallAWS(envURL, accessTok, uninstallDryRun)
	},
}

var uninstallAWSLambdaCmd = &cobra.Command{
	Use:   "aws-lambda",
	Short: "Remove Dynatrace Lambda Layer from all functions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.UninstallAWSLambda(uninstallDryRun)
	},
}

var uninstallOtelCmd = &cobra.Command{
	Use:   "otel",
	Short: "Kill running OTel Collector processes and remove installation files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.UninstallOtelCollector(uninstallDryRun)
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
	uninstallCmd.AddCommand(uninstallOtelCmd)
	uninstallCmd.AddCommand(uninstallSelfCmd)
}
