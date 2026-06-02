package oneagent

import (
	"fmt"
	"os"
	"runtime"

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
	MonitoringMode string
}

// Environment identifies the target OS and CPU architecture used to select the
// correct OneAgent installer. OS values: "linux", "windows". Arch values:
// "x86", "arm".
type Environment struct {
	OS   string
	Arch string
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MonitoringMode: "fullstack"}
}

func ResolveAgentConfig(opts InstallOptions) AgentConfig {
	cfg := DefaultAgentConfig()
	if opts.MonitoringMode != "" {
		cfg.MonitoringMode = opts.MonitoringMode
	}
	logger.Debug("resolved agent config",
		"monitoring-mode", cfg.MonitoringMode,
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
	display.PrintStatusLine("oneagent", fmt.Sprintf("PoC flow (monitoring-mode=%s)", cfg.MonitoringMode), display.ColorWarning)
	logger.Debug("install options",
		"dry_run", opts.DryRun,
		"no_verify_signature", opts.NoVerifySignature,
		"monitoring_mode", cfg.MonitoringMode,
	)

	if opts.DryRun {
		printDryRun(c.Classic.BaseURL(), env, cfg, opts)
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

	display.PrintStatusLine("oneagent", "install execution not yet implemented", display.ColorWarning)
	logger.Debug("install execution not yet implemented", "installer_path", installerPath)
	return nil
}

func printDryRun(baseURL string, env Environment, cfg AgentConfig, opts InstallOptions) {
	fmt.Println("[dry-run] Would install Dynatrace OneAgent")
	if url, err := InstallerDownloadURL(baseURL, env); err == nil {
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
