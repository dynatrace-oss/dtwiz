package cmd

import (
	"github.com/dietermayrhofer/dtwiz/pkg/installer"
	"github.com/spf13/cobra"
)

var installDryRun bool

var installCmd = &cobra.Command{
	Use:   "install <method>",
	Short: "Install a Dynatrace ingestion method",
	Args:  cobra.MinimumNArgs(1),
}

var installOneAgentCmd = &cobra.Command{
	Use:   "oneagent",
	Short: "Install Dynatrace OneAgent on this host",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		quiet, _ := cmd.Flags().GetBool("quiet")
		hostGroup, _ := cmd.Flags().GetString("host-group")
		return installer.InstallOneAgent(envURL, token, installDryRun, quiet, hostGroup)
	},
}

var installKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "Deploy Dynatrace Operator on Kubernetes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.InstallKubernetes(envURL, token, accessToken(), "", installDryRun)
	},
}

var installDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Install Dynatrace OneAgent for Docker",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.InstallDocker(envURL, token, installDryRun)
	},
}

var installOtelCmd = &cobra.Command{
	Use:   "otel",
	Short: "Install OTel Collector and instrument your application",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.InstallOtelCollector(envURL, token, accessToken(), platformToken(), installDryRun)
	},
}

var installOtelCollectorCmd = &cobra.Command{
	Use:   "otel-collector",
	Short: "Install the Dynatrace OpenTelemetry Collector only",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.InstallOtelCollectorOnly(envURL, token, accessToken(), platformToken(), installDryRun)
	},
}

var otelPythonServiceName string
var installOtelPythonCmd = &cobra.Command{
	Use:   "otel-python",
	Short: "Set up OpenTelemetry Python auto-instrumentation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.InstallOtelPython(envURL, token, platformToken(), otelPythonServiceName, installDryRun)
	},
}

var otelJavaServiceName string
var installOtelJavaCmd = &cobra.Command{
	Use:   "otel-java",
	Short: "Set up OpenTelemetry Java auto-instrumentation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.InstallOtelJava(envURL, token, otelJavaServiceName, installDryRun)
	},
}

var installAWSCmd = &cobra.Command{
	Use:   "aws",
	Short: "Set up Dynatrace AWS CloudFormation integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, token, err := getDtEnvironment()
		if err != nil {
			return err
		}
		return installer.InstallAWS(envURL, token, platformToken(), installDryRun)
	},
}

var installAzureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Set up Dynatrace Azure Monitor integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.InstallAzure()
	},
}

var installGCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "Set up Dynatrace Google Cloud Platform integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.InstallGCP()
	},
}

func init() {
	installCmd.PersistentFlags().BoolVar(&installDryRun, "dry-run", false, "show what would be done without executing")

	installOtelPythonCmd.Flags().StringVar(&otelPythonServiceName, "service-name", "", "OTEL_SERVICE_NAME for the instrumented application (default: my-service)")
	installOtelJavaCmd.Flags().StringVar(&otelJavaServiceName, "service-name", "", "OTEL_SERVICE_NAME for the instrumented application (default: my-service)")

	installOneAgentCmd.Flags().Bool("quiet", false, "Run a silent/unattended installation with no output")
	installOneAgentCmd.Flags().String("host-group", "", "Assign the host to a host group (--set-host-group)")
	installCmd.AddCommand(installOneAgentCmd)
	installCmd.AddCommand(installKubernetesCmd)
	installCmd.AddCommand(installDockerCmd)
	installCmd.AddCommand(installOtelCmd)
	installCmd.AddCommand(installOtelCollectorCmd)
	installCmd.AddCommand(installOtelPythonCmd)
	installCmd.AddCommand(installOtelJavaCmd)
	installCmd.AddCommand(installAWSCmd)
	installCmd.AddCommand(installAzureCmd)
	installCmd.AddCommand(installGCPCmd)
}
