package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	awspkg "github.com/dynatrace-oss/dtwiz/pkg/installer/aws"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/azure"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/gcp"
	k8s "github.com/dynatrace-oss/dtwiz/pkg/installer/kubernetes"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/oneagent"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var installDryRun bool
var installAutoConfirm bool

var (
	flagMonitoringMode        string
	flagNoVerifySignature     bool
	flagSkipConnectivityCheck bool
	flagConnectivityCheckOnly bool
)

var installCmd = &cobra.Command{
	Use:   "install <method>",
	Short: "Install a Dynatrace ingestion method",
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// installCmd.PersistentPreRun overrides root's, so reproduce its behaviour here.
		cmd.Root().SilenceUsage = true
		logger.Init(debugFlag, verbosityFlag)
		logger.Verbose("logging: verbose")
		logger.Debug("logging: debug")
		featureflags.ApplyCLIOverrides(cmd.Flags())
		installer.AutoConfirm = installAutoConfirm
		fireSelfMonitoringStart()
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		c, err := setupClientFromCreds(envURL, classicTok, platformTok)
		if err != nil {
			return err
		}
		quiet, _ := cmd.Flags().GetBool("quiet")
		hostGroup, _ := cmd.Flags().GetString("host-group")

		opts := oneagent.InstallOptions{
			DryRun:                installDryRun,
			MonitoringMode:        flagMonitoringMode,
			HostGroup:             hostGroup,
			NoVerifySignature:     flagNoVerifySignature,
			SkipConnectivityCheck: flagSkipConnectivityCheck,
			ConnectivityCheckOnly: flagConnectivityCheckOnly,
			Quiet:                 quiet,
		}

		if err := oneagent.InstallOneAgentV2(c, opts); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		k8sInfo := analyzer.DetectKubernetes()
		clusterName := ""
		distro := k8sInfo.Distribution
		if k8sInfo.Available {
			clusterName = k8sInfo.Cluster
		}
		if err := k8s.InstallKubernetes(envURL, classicTok, clusterName, distro, installDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
		}
		return nil
	},
}

var installDockerCmd = &cobra.Command{
	Use:    "docker",
	Short:  "Install Dynatrace OneAgent for Docker",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !featureflags.IsEnabled(featureflags.Experimental) {
			return fmt.Errorf("docker installation is an experimental feature; enable it with --experimental or DTWIZ_EXPERIMENTAL=true")
		}
		envURL, accessTok, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		if err := installer.InstallDocker(envURL, classicTok, installDryRun); err != nil {
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		manualLang, err := otel.InstallOtelCollectorWithProject(envURL, classicTok, platformTok, otelProject, installDryRun)
		if err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngestOtel(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat), manualLang)
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		if err := otel.InstallOtelCollectorOnly(envURL, classicTok, platformTok, installDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		if err := otel.InstallOtelPython(envURL, classicTok, platformTok, otelPythonServiceName, otelProject, installDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
		}
		return nil
	},
}

var otelNodeServiceName string
var installOtelNodeCmd = &cobra.Command{
	Use:   "otel-node",
	Short: "Set up OpenTelemetry Node.js auto-instrumentation",
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
		if err := otel.InstallOtelNode(envURL, classicTok, platformTok, otelNodeServiceName, otelProject, installDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		if err := otel.InstallOtelJava(envURL, classicTok, otelJavaServiceName, otelProject, installDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
		}
		return nil
	},
}

var installAWSCmd = &cobra.Command{
	Use:   "aws",
	Short: "Set up Dynatrace AWS CloudFormation integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, _, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if _, err := validateCredentials(envURL, "", platformTok); err != nil {
			return err
		}
		if err := awspkg.InstallAWS(envURL, platformTok, installDryRun, StartTime.UTC().Format(installer.IngestTimeFormat)); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		if err := installer.InstallAWSLambda(envURL, classicTok, installDryRun, true); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
		}
		return nil
	},
}

var installAzureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Set up Dynatrace Azure Monitor integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, _, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if _, err := validateCredentials(envURL, "", platformTok); err != nil {
			return err
		}
		if err := azure.InstallAzure(envURL, platformTok, installDryRun, StartTime); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		return nil
	},
}

var installGCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "Set up Dynatrace Google Cloud Platform integration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		envURL, _, platformTok, err := getDtEnvironment()
		if err != nil {
			return err
		}
		if err := gcp.InstallGCP(envURL, platformTok, installDryRun, StartTime); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		return nil
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
		classicTok, err := validateCredentials(envURL, accessTok, platformTok)
		if err != nil {
			return err
		}
		if err := otel.InstallDemo(envURL, classicTok, platformTok, installDryRun); err != nil {
			if errors.Is(err, installer.ErrInstallCancelled) {
				return nil
			}
			return err
		}
		if !installDryRun {
			installer.WatchIngest(envURL, platformTok, StartTime.UTC().Format(installer.IngestTimeFormat))
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
	installOtelNodeCmd.Flags().StringVar(&otelProject, "project", "", "path to the Node.js project to instrument (skips interactive scan)")
	installOtelNodeCmd.Flags().StringVar(&otelNodeServiceName, "service-name", "", "OTEL_SERVICE_NAME for the instrumented application (default: my-service)")
	installOtelJavaCmd.Flags().StringVar(&otelProject, "project", "", "path to the Java project to instrument (skips interactive scan)")
	installOtelJavaCmd.Flags().StringVar(&otelJavaServiceName, "service-name", "", "OTEL_SERVICE_NAME for the instrumented application (default: my-service)")

	installOneAgentCmd.Flags().Bool("quiet", false, "Suppress prompts and output; requires an elevated terminal on Windows")
	installOneAgentCmd.Flags().String("host-group", "", "Assign the host to a host group (--set-host-group)")
	installOneAgentCmd.Flags().StringVar(&flagMonitoringMode, "monitoring-mode", string(oneagent.InstallModeFullStack), "OneAgent monitoring mode to pass to the installer")
	installOneAgentCmd.Flags().BoolVar(&flagNoVerifySignature, "no-verify-signature", false, "Skip installer signature verification (Linux only)")
	installOneAgentCmd.Flags().BoolVar(&flagSkipConnectivityCheck, "skip-connectivity-check", false, "Skip endpoint connectivity probe before installation")
	installOneAgentCmd.Flags().BoolVar(&flagConnectivityCheckOnly, "connectivity-check-only", false, "Run connectivity probe and exit without installing")
	installCmd.AddCommand(installOneAgentCmd)
	installCmd.AddCommand(installKubernetesCmd)
	installCmd.AddCommand(installDockerCmd)
	installCmd.AddCommand(installOtelCmd)
	installCmd.AddCommand(installOtelCollectorCmd)
	installCmd.AddCommand(installOtelPythonCmd)
	installCmd.AddCommand(installOtelNodeCmd)
	installCmd.AddCommand(installOtelJavaCmd)
	installCmd.AddCommand(installAWSCmd)
	installCmd.AddCommand(installAWSLambdaCmd)
	installCmd.AddCommand(installAzureCmd)
	installCmd.AddCommand(installGCPCmd)
	installCmd.AddCommand(installDemoCmd)

	// PersistentPreRun doesn't run for --help, so resolve the experimental flag here
	// to conditionally show the docker subcommand in help output.
	defaultInstallHelp := installCmd.HelpFunc()
	installCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		featureflags.ApplyCLIOverrides(cmd.Flags())
		installDockerCmd.Hidden = !featureflags.IsEnabled(featureflags.Experimental)
		defaultInstallHelp(cmd, args)
	})
}
