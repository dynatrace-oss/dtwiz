package oneagent

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// needsSudoFn is the function used to check if sudo is needed.
// Overridable in tests.
var needsSudoFn = installer.NeedsSudo

// sudoPathFn resolves the path to the sudo binary.
// Overridable in tests to avoid exec.LookPath on platforms without sudo.
var sudoPathFn = func() (string, error) { return exec.LookPath("sudo") }

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
			sudoPath, err := sudoPathFn()
			if err != nil {
				return nil, fmt.Errorf("sudo not found: %w", err)
			}
			logger.Debug("using sudo", "path", sudoPath)
			argv = append([]string{sudoPath}, argv...)
		}
	case "windows":
		argv = []string{installerPath, "--quiet"}
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
	if len(argv) == 0 {
		return 1, fmt.Errorf("install command is empty")
	}
	start := time.Now()
	logger.Debug("executing installer", "argv", argv)
	if !quiet {
		display.PrintStatusLine("execute", "Executing installer...", display.ColorMessage)
	}
	code, err := runInstaller(argv, quiet)
	logger.Verbose("installer exited", "exit_code", code, "duration", time.Since(start))
	if err != nil {
		return code, fmt.Errorf("installer %w", err)
	}
	if !quiet {
		display.PrintStatusLine("result", "Installer executed successfully", display.ColorOK)
	}
	return 0, nil
}
