//go:build !windows

package oneagent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// stateDir holds OneAgent configuration that the uninstall script preserves.
const stateDir = "/var/lib/dynatrace/oneagent"

// uninstallScriptPath derives the uninstall script location from
// oneAgentInstallDir (declared in detect_unix.go) so detection and uninstall
// always agree on the install root — including when tests redirect it.
func uninstallScriptPath() string {
	return filepath.Join(oneAgentInstallDir, "agent", "uninstall.sh")
}

func printPlan() {
	fmt.Printf("  Script:     %s\n", uninstallScriptPath())
	if needsSudoFn() {
		fmt.Println("  Privileges: sudo required (current user is not root)")
	}
}

func runUninstall() error {
	script := uninstallScriptPath()
	logger.Debug("checking uninstall script", "path", script)
	if _, err := os.Stat(script); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("OneAgent uninstall script not found at %s", script)
		}
		return fmt.Errorf("stat OneAgent uninstall script at %s: %w", script, err)
	}

	needsSudo := needsSudoFn()
	logger.Debug("privilege check", "needs_sudo", needsSudo)

	args := []string{script}
	if needsSudo {
		sudoPath, err := sudoPathFn()
		if err != nil {
			return fmt.Errorf("sudo not found: %w", err)
		}
		logger.Debug("using sudo", "path", sudoPath)
		args = append([]string{sudoPath}, args...)
	}
	logger.Debug("running uninstall script", "argv", args)

	display.PrintStatusLine("uninstall", "running uninstall script...", display.ColorMessage)
	if err := installer.RunCommand(args[0], args[1:]...); err != nil {
		return fmt.Errorf("OneAgent uninstall failed: %w", err)
	}
	logger.Verbose("uninstall script completed")

	if err := cleanupInstallDir(oneAgentInstallDir, needsSudo); err != nil {
		logger.Warn("could not remove residual directory", "path", oneAgentInstallDir, "error", err)
		fmt.Printf("  Warning: could not remove %s: %v\n", oneAgentInstallDir, err)
	}

	if _, statErr := os.Stat(stateDir); !os.IsNotExist(statErr) {
		logger.Debug("state directory preserved", "path", stateDir)
		display.PrintStatusLine("note", "configuration preserved at "+stateDir, display.ColorMuted)
	}

	return nil
}

// cleanupInstallDir removes the stub directory left behind by the uninstall
// script. Missing paths and non-directories are silently skipped.
func cleanupInstallDir(path string, needsSudo bool) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		logger.Debug("residual directory already absent", "path", path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		logger.Debug("residual path is not a directory, skipping", "path", path)
		return nil
	}

	if needsSudo {
		sudoPath, err := sudoPathFn()
		if err != nil {
			return fmt.Errorf("sudo not found: %w", err)
		}
		logger.Debug("removing residual install directory", "path", path, "needs_sudo", true)
		if err := installer.RunCommand(sudoPath, "rm", "-rf", path); err != nil {
			return fmt.Errorf("sudo rm -rf %s: %w", path, err)
		}
		logger.Verbose("removed residual install directory", "path", path)
		return nil
	}

	logger.Debug("removing residual install directory", "path", path, "needs_sudo", false)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("rm -rf %s: %w", path, err)
	}
	logger.Verbose("removed residual install directory", "path", path)
	return nil
}
