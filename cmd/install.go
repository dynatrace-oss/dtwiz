package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

var installDryRun bool
var installAutoConfirm bool

var installCmd = &cobra.Command{
	Use:   "install <method>",
	Short: "Install a Dynatrace ingestion method",
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		installer.AutoConfirm = installAutoConfirm
	},
}

var installOneAgentCmd = &cobra.Command{
	Use:   "oneagent",
	Short: "Install Dynatrace OneAgent on this host",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		quiet, _ := cmd.Flags().GetBool("quiet")
		hostGroup, _ := cmd.Flags().GetString("host-group")
		if err := installer.InstallOneAgent(envURL, accessTok, installDryRun, quiet, hostGroup); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

var installKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "Deploy Dynatrace Operator on Kubernetes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallKubernetes(envURL, accessTok, accessTok, "", installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

var installDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Install Dynatrace OneAgent for Docker",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallDocker(envURL, accessTok, installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

var installOtelCmd = &cobra.Command{
	Use:   "otel",
	Short: "Install OTel Collector and instrument your application",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallOtelCollectorWithProject(envURL, accessTok, accessTok, platformTok, otelProject, installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

var installOtelCollectorCmd = &cobra.Command{
	Use:   "otel-collector",
	Short: "Install the Dynatrace OpenTelemetry Collector only",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallOtelCollectorOnly(envURL, accessTok, accessTok, platformTok, installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

var otelProject string
var otelPythonServiceName string
var installOtelPythonCmd = &cobra.Command{
	Use:   "otel-python",
	Short: "Set up OpenTelemetry Python auto-instrumentation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallOtelPython(envURL, accessTok, platformTok, otelPythonServiceName, otelProject, installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

var otelJavaServiceName string
var installOtelJavaCmd = &cobra.Command{
	Use:   "otel-java",
	Short: "Set up OpenTelemetry Java auto-instrumentation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallOtelJava(envURL, accessTok, otelJavaServiceName, installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

var installAWSCmd = &cobra.Command{
	Use:   "aws",
	Short: "Set up Dynatrace AWS CloudFormation integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallAWS(envURL, accessTok, platformTok, installDryRun, StartTime.UTC().Format("2006-01-02T15:04:05Z")); err != nil {
			return err
		}
		return nil
	},
}

var installAWSLambdaCmd = &cobra.Command{
	Use:   "aws-lambda",
	Short: "Install Dynatrace Lambda Layer on all functions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallAWSLambda(envURL, accessTok, platformTok, installDryRun, true); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
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

var installDemoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Install the schnitzel demo app and set up Dynatrace OTel monitoring",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := validateCredentials(envURL, accessTok, platformTok); err != nil {
			return err
		}
		if err := installer.InstallDemo(envURL, accessTok, platformTok, installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format("2006-01-02T15:04:05Z"))
		}
		return nil
	},
}

func init() {
	installCmd.PersistentFlags().BoolVar(&installDryRun, "dry-run", false, "show what would be done without executing")
	installCmd.PersistentFlags().BoolVarP(&installAutoConfirm, "yes", "y", false, "skip confirmation prompts")

	installOtelCmd.Flags().StringVar(&otelProject, "project", "", "path to the project to instrument (skips interactive scan)")
	installOtelPythonCmd.Flags().StringVar(&otelProject, "project", "", "path to the Python project to instrument (skips interactive scan)")
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
	installCmd.AddCommand(installAWSLambdaCmd)
	installCmd.AddCommand(installAzureCmd)
	installCmd.AddCommand(installGCPCmd)
	installCmd.AddCommand(installDemoCmd)
}
