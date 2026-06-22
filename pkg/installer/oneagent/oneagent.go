package oneagent

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type InstallOptions struct {
	DryRun                bool
	MonitoringMode        string
	HostGroup             string
	NoVerifySignature     bool
	SkipConnectivityCheck bool
	ConnectivityCheckOnly bool
	PrintEndpoints        bool
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

const connectivityProbeTimeout = 5 * time.Second

func InstallOneAgentV2(c *client.Client, opts InstallOptions) error {
	env := detectRuntimeEnvironment()
	logger.Debug("detected environment", "os", env.OS, "arch", env.Arch)

	if err := validateEnvironment(env); err != nil {
		return err
	}
	display.PrintStatusLine("OS", fmt.Sprintf("✓ %s / %s", env.OS, env.Arch), display.ColorOK)

	cfg := ResolveAgentConfig(opts)
	cfg.ServerURL = c.Classic.BaseURL()
	logger.Debug("install options",
		"dry_run", opts.DryRun,
		"no_verify_signature", opts.NoVerifySignature,
		"monitoring_mode", cfg.MonitoringMode,
		"app_log_content_access", cfg.AppLogContentAccess,
		"server_url", cfg.ServerURL,
	)

	preflight, err := runPreflightChecks(env, opts)
	if err != nil {
		return err
	}

	if opts.DryRun {
		printDryRun(env, cfg, opts, preflight.IsUpdate)
		return nil
	}

	logTenantID(c.Classic.BaseURL())

	endpoints, err := ResolveEndpoints(c.Classic)
	if err != nil {
		return err
	}

	if opts.PrintEndpoints {
		for _, e := range endpoints {
			fmt.Println(e.String())
		}
		return nil
	}

	if opts.ConnectivityCheckOnly {
		report := CheckAllEndpoints(endpoints, connectivityProbeTimeout)
		printConnectivityReport(report)
		return nil
	}

	if !opts.SkipConnectivityCheck {
		report := CheckAllEndpoints(endpoints, connectivityProbeTimeout)
		if !report.AllPassed {
			printConnectivityWarning(report)
		}
	} else {
		logger.Debug("skipping connectivity probe", "reason", "--skip-connectivity-check")
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

// classifyEnvironment maps goos/goarch strings to canonical OS and arch tokens.
// It never returns an error — unrecognized values map to "other", and
// validateEnvironment encodes the support decision.
func classifyEnvironment(goos, goarch string) Environment {
	var arch string
	switch goarch {
	case "amd64", "386":
		arch = "x86"
	case "arm64", "arm":
		arch = "arm"
	default:
		arch = "other"
	}

	var os string
	switch goos {
	case "linux":
		os = "linux"
	case "windows":
		os = "windows"
	case "darwin":
		os = "darwin"
	default:
		os = "other"
	}

	return Environment{OS: os, Arch: arch}
}

func detectRuntimeEnvironment() Environment {
	return classifyEnvironment(runtime.GOOS, runtime.GOARCH)
}

func validateEnvironment(env Environment) error {
	switch env.OS {
	case "darwin":
		return fmt.Errorf("OneAgent direct install is not supported on macOS; use Docker or Linux")
	case "other":
		return fmt.Errorf("OneAgent direct install is not supported on %s; use Linux or Windows", runtime.GOOS)
	}
	if env.Arch == "other" {
		return fmt.Errorf("OneAgent direct install is not supported on %s architecture; use an x86 or ARM host", runtime.GOARCH)
	}
	return nil
}
