package oneagent

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type InstallOptions struct {
	DryRun                bool
	MonitoringMode        string
	HostGroup             string
	NoVerifySignature     bool
	SkipConnectivityCheck bool
	ConnectivityCheckOnly bool
	Quiet                 bool
}

type AgentConfig struct {
	MonitoringMode      string
	AppLogContentAccess bool
	ServerURL           string
}

// Environment identifies the target OS and CPU architecture used to select the
// correct OneAgent installer. OS values: "linux", "windows". Arch values:
// "x86", "arm".
type Environment struct {
	OS   string
	Arch string
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MonitoringMode: "fullstack", AppLogContentAccess: true}
}

func ResolveAgentConfig(opts InstallOptions) AgentConfig {
	cfg := DefaultAgentConfig()
	if opts.MonitoringMode != "" {
		cfg.MonitoringMode = opts.MonitoringMode
	}
	logger.Debug("resolved agent config",
		"monitoring_mode", cfg.MonitoringMode,
		"app_log_content_access", cfg.AppLogContentAccess,
		"override_set", cfg.MonitoringMode != "fullstack",
	)
	return cfg
}

func InstallOneAgentV2(c *client.Client, opts InstallOptions) error {
	env, err := detectRuntimeEnvironment()
	if err != nil {
		return err
	}
	logger.Debug("detected environment", "os", env.OS, "arch", env.Arch)

	cfg := ResolveAgentConfig(opts)
	cfg.ServerURL = c.Classic.BaseURL()
	logger.Debug("install options",
		"dry_run", opts.DryRun,
		"no_verify_signature", opts.NoVerifySignature,
		"monitoring_mode", cfg.MonitoringMode,
		"app_log_content_access", cfg.AppLogContentAccess,
		"server_url", cfg.ServerURL,
	)

	updating := oneAgentInstalled()
	logger.Debug("existing oneagent detected", "updating", updating)

	if opts.DryRun {
		printDryRun(env, cfg, opts, updating)
		return nil
	}

	if updating && !opts.Quiet && !opts.ConnectivityCheckOnly {
		ok, err := installer.ConfirmProceed("  OneAgent is already installed. Update?")
		if err != nil || !ok {
			display.PrintStatusLine("result", "update cancelled", display.ColorMuted)
			return installer.ErrInstallCancelled
		}
	}

	if opts.SkipConnectivityCheck {
		logger.Debug("skipping connectivity probe", "reason", "--skip-connectivity-check")
	} else {
		endpoints, err := ResolveEndpoints(c.Classic)
		if err != nil {
			return err
		}
		if opts.ConnectivityCheckOnly {
			// Print header before the probe so the user sees what's happening
			// during the dial timeout window.
			display.Header("Checking network connectivity...")
			report := CheckAllEndpoints(endpoints, defaultProbeTimeout)
			printConnectivityResults(report)
			return nil
		}
		// Normal install path: transient pending line while probes run,
		// then clear it — no lingering output unless something failed.
		display.PrintPending("connectivity", "checking endpoints...")
		report := CheckAllEndpoints(endpoints, defaultProbeTimeout)
		display.ClearPending()
		if report.FailedCount > 0 {
			printConnectivityWarning(report)
			return fmt.Errorf("connectivity check failed: %d/%d endpoints unreachable", report.FailedCount, len(report.Results))
		}
		display.PrintStatusLine("connectivity", "all endpoints reachable", display.ColorOK)
	}
	if opts.ConnectivityCheckOnly {
		return nil
	}

	installerPath, err := DownloadInstaller(c.Classic, env)
	if err != nil {
		return err
	}
	defer os.Remove(installerPath)

	if err := VerifyInstallerSignature(env, installerPath, opts.NoVerifySignature); err != nil {
		return err
	}

	argv, err := BuildInstallCommand(env, cfg, opts, installerPath)
	if err != nil {
		return err
	}

	_, err = ExecuteInstallCommand(argv, opts.Quiet)
	return err
}

func printDryRun(env Environment, cfg AgentConfig, opts InstallOptions, updating bool) {
	verb := "install"
	if updating {
		verb = "update"
	}
	fmt.Printf("[dry-run] Would %s Dynatrace OneAgent\n", verb)
	if url, err := InstallerDownloadURL(cfg.ServerURL, env); err == nil {
		fmt.Printf("  Installer:  %s\n", url)
	}
	if opts.NoVerifySignature || env.OS != "linux" {
		fmt.Printf("  Signature:  skipped\n")
	} else {
		fmt.Printf("  Signature:  would verify against %s\n", dtRootCertURL)
	}
	fmt.Printf("  Mode:       %s\n", cfg.MonitoringMode)
	if opts.HostGroup != "" {
		fmt.Printf("  Host group: %s\n", opts.HostGroup)
	}
	if argv, err := BuildInstallCommand(env, cfg, opts, "<installer>"); err == nil {
		fmt.Printf("  Command:    %s\n", strings.Join(argv, " "))
	}
	display.PrintStatusLine("dry-run", "no changes made", display.ColorMuted)
}

// detectRuntimeEnvironment returns an Environment based on the current host
// OS and architecture. Used as a stand-in until DetectEnvironment (Task 2) is
// implemented.
func detectRuntimeEnvironment() (Environment, error) {
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86"
	case "arm64":
		arch = "arm"
	default:
		return Environment{}, fmt.Errorf("unsupported architecture for OneAgent: %s", runtime.GOARCH)
	}

	switch runtime.GOOS {
	case "linux":
		return Environment{OS: "linux", Arch: arch}, nil
	case "windows":
		return Environment{OS: "windows", Arch: arch}, nil
	case "darwin":
		return Environment{}, fmt.Errorf("OneAgent direct install is not supported on macOS; use Docker or Linux")
	default:
		return Environment{}, fmt.Errorf("unsupported OS for OneAgent: %s", runtime.GOOS)
	}
}
