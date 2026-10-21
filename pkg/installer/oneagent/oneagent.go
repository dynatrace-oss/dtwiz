package oneagent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	if !opts.Quiet {
		display.Header(fmt.Sprintf("dtwiz · OneAgent on %s %s", env.OS, env.Arch))
	}

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
		var report ConnectivityReport
		_ = runStep(opts, "Checked connectivity", func(s *display.Step) error {
			report = CheckAllEndpoints(endpoints, connectivityProbeTimeout)
			s.Detail("%d endpoints", len(report.Results))
			return nil
		})
		if !report.AllPassed {
			printConnectivityWarning(report)
		}
	} else {
		logger.Debug("skipping connectivity probe", "reason", "--skip-connectivity-check")
	}

	var installerPath string
	err = runStep(opts, "Downloaded installer", func(s *display.Step) error {
		var derr error
		installerPath, derr = DownloadInstallerWithProgress(c.Classic, env, func(read, total int64) {
			if total > 0 {
				s.Detail("%s / %s", display.HumanBytes(read), display.HumanBytes(total))
			} else {
				s.Detail("%s", display.HumanBytes(read))
			}
		})
		if derr == nil {
			if fi, serr := os.Stat(installerPath); serr == nil {
				s.Detail("%s", display.HumanBytes(fi.Size()))
			}
		}
		return derr
	})
	if err != nil {
		return err
	}
	defer os.Remove(installerPath)

	if env.OS == "linux" && !opts.NoVerifySignature {
		err = runStep(opts, "Verified signature", func(*display.Step) error {
			return VerifyInstallerSignature(env, installerPath, opts.NoVerifySignature)
		})
		if err != nil {
			return err
		}
	}

	argv, err := BuildInstallCommand(env, cfg, opts, installerPath)
	if err != nil {
		return err
	}
	display.Detail("command: %s", strings.Join(argv, " "))

	// sudo may prompt for a password, which a spinner would overwrite: prime
	// its credential cache on the terminal first.
	if err := primeSudo(argv, opts); err != nil {
		return err
	}

	err = runStep(opts, "Installed OneAgent", func(*display.Step) error {
		_, execErr := ExecuteInstallCommand(argv, opts.Quiet)
		return execErr
	})
	installer.RecordFeature(installer.FeatureHostMonitoring, err == nil)
	return err
}

// runStep runs fn as a spinner step, or plainly when --quiet suppresses output.
func runStep(opts InstallOptions, label string, fn func(*display.Step) error) error {
	if opts.Quiet {
		return fn(&display.Step{})
	}
	return display.RunStep(label, fn)
}

// primeSudo runs "sudo -v" on the terminal when the install command is sudo-wrapped.
func primeSudo(argv []string, opts InstallOptions) error {
	if opts.Quiet || len(argv) == 0 || filepath.Base(argv[0]) != "sudo" {
		return nil
	}
	cmd := exec.Command(argv[0], "-v")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo authentication failed: %w", err)
	}
	return nil
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
