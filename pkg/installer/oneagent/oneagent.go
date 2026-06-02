package oneagent

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// needsSudoFn is the function used to check if sudo is needed.
// Overridable in tests.
var needsSudoFn = needsSudo

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MonitoringMode: "fullstack", AppLogContentAccess: true}
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

// BuildInstallCommand constructs the OS-specific installer argv from the
// resolved AgentConfig and InstallOptions.
//
// Linux: [sudo?] /bin/sh <installer> --set-server=... --set-monitoring-mode=... --set-app-log-content-access=... [--set-host-group=...]
// Windows: <installer> [--quiet] --set-monitoring-mode=... --set-app-log-content-access=... [--set-host-group=...]
func BuildInstallCommand(env Environment, cfg AgentConfig, opts InstallOptions, installerPath string) ([]string, error) {
	var argv []string
	switch env.OS {
	case "linux":
		argv = []string{
			"/bin/sh", installerPath,
			fmt.Sprintf("--set-server=%s", cfg.ServerURL),
			fmt.Sprintf("--set-monitoring-mode=%s", cfg.MonitoringMode),
			fmt.Sprintf("--set-app-log-content-access=%t", cfg.AppLogContentAccess),
		}
		if opts.HostGroup != "" {
			argv = append(argv, fmt.Sprintf("--set-host-group=%s", opts.HostGroup))
		}
		if needsSudoFn() {
			argv = append([]string{"sudo"}, argv...)
		}
	case "windows":
		argv = []string{installerPath}
		if opts.Quiet {
			argv = append(argv, "--quiet")
		}
		argv = append(argv,
			fmt.Sprintf("--set-monitoring-mode=%s", cfg.MonitoringMode),
			fmt.Sprintf("--set-app-log-content-access=%t", cfg.AppLogContentAccess),
		)
		if opts.HostGroup != "" {
			argv = append(argv, fmt.Sprintf("--set-host-group=%s", opts.HostGroup))
		}
	default:
		return nil, fmt.Errorf("unsupported OS for install command: %q", env.OS)
	}
	logger.Debug("built install command", "argv", argv)
	return argv, nil
}

// ExecuteInstallCommand launches the installer subprocess described by argv.
// stdout/stderr are streamed to the user when quiet is false, captured when
// true. The subprocess exit code is always returned alongside any error; a
// non-zero exit is always an error.
func ExecuteInstallCommand(argv []string, quiet bool) (int, error) {
	start := time.Now()
	logger.Debug("executing installer", "argv", argv)

	if !quiet {
		display.PrintStatusLine("execute", "Executing installer...", display.ColorMessage)
	}

	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec // argv is built from controlled inputs
	var captured bytes.Buffer
	if quiet {
		cmd.Stdout = &captured
		cmd.Stderr = &captured
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	runErr := cmd.Run()

	code := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			return 1, fmt.Errorf("executing installer: %w", runErr)
		}
	}

	logger.Verbose("installer exited", "exit_code", code, "duration", time.Since(start))

	if code != 0 {
		msg := fmt.Sprintf("installer exited with code %d", code)
		if out := strings.TrimSpace(captured.String()); out != "" {
			msg += ": " + out
		}
		return code, fmt.Errorf("%s", msg) //nolint:goerr113
	}

	if !quiet {
		display.PrintStatusLine("result", "Installer executed successfully", display.ColorOK)
	}
	return 0, nil
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
	)

	if opts.DryRun {
		printDryRun(env, cfg, opts)
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

func printDryRun(env Environment, cfg AgentConfig, opts InstallOptions) {
	fmt.Println("[dry-run] Would install Dynatrace OneAgent")
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
