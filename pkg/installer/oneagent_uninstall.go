package installer

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	linuxUninstallScript = "/opt/dynatrace/oneagent/agent/uninstall.sh"

	// Stub directory left behind by the uninstall script; we clean it up.
	linuxInstallDir = "/opt/dynatrace/oneagent"

	linuxStateDir = "/var/lib/dynatrace/oneagent"
)

// UninstallOneAgent removes Dynatrace OneAgent from the current host.
func UninstallOneAgent(dryRun bool) error {
	switch runtime.GOOS {
	case "linux":
		return uninstallOneAgentLinux(dryRun)
	case "windows":
		return uninstallOneAgentWindows(dryRun)
	case "darwin":
		return fmt.Errorf("OneAgent is not supported on macOS — nothing to uninstall")
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func uninstallOneAgentLinux(dryRun bool) error {
	logger.Debug("checking for OneAgent uninstall script", "path", linuxUninstallScript)
	if _, err := os.Stat(linuxUninstallScript); os.IsNotExist(err) {
		return fmt.Errorf("OneAgent uninstall script not found at %s — is OneAgent installed?", linuxUninstallScript)
	}

	header := color.New(color.FgMagenta, color.Bold)
	muted := color.New()

	header.Println("  OneAgent Uninstall (Linux)")
	muted.Println("  " + "────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  Uninstall script:  %s\n", linuxUninstallScript)

	needsSudo := NeedsSudo()
	logger.Debug("privilege check", "needs_sudo", needsSudo)
	if needsSudo {
		fmt.Println("  Privileges:        sudo required (current user is not root)")
	}
	fmt.Println()

	if dryRun {
		fmt.Println("[dry-run] Would run the OneAgent uninstall script. No changes made.")
		return nil
	}

	ok, err := confirmProceed("  Proceed with OneAgent uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Uninstall cancelled.")
		return nil
	}
	fmt.Println()

	args := []string{linuxUninstallScript}
	if needsSudo {
		args = append([]string{"sudo"}, args...)
	}
	logger.Debug("running uninstall script", "argv", args)

	fmt.Println("  Running OneAgent uninstall script...")
	if err := RunCommand(args[0], args[1:]...); err != nil {
		return fmt.Errorf("OneAgent uninstall failed: %w", err)
	}
	logger.Verbose("uninstall script completed successfully")

	// The Dynatrace uninstall script removes agent/ contents but leaves the
	// parent install directory as an empty stub. Clean it up.
	if err := removeResidualDir(linuxInstallDir, needsSudo); err != nil {
		logger.Warn("could not remove residual install directory", "path", linuxInstallDir, "error", err)
		fmt.Printf("  Warning: could not remove %s: %v\n", linuxInstallDir, err)
	}

	// A permission error on stat doesn't mean the directory is absent.
	if _, statErr := os.Stat(linuxStateDir); !os.IsNotExist(statErr) {
		logger.Debug("state directory preserved by uninstall script", "path", linuxStateDir)
		fmt.Printf("  Note: configuration preserved at %s\n", linuxStateDir)
	}

	color.New(color.FgGreen, color.Bold).Println("\n  OneAgent uninstalled successfully.")
	return nil
}

func removeResidualDir(path string, needsSudo bool) error {
	if needsSudo {
		// Skip stat — a permission error on stat does not mean sudo rm -rf will
		// fail, and rm -rf is a no-op when the path is absent (-f suppresses the
		// "no such file" error).
		logger.Debug("removing residual install directory", "path", path, "needs_sudo", true)
		if err := RunCommand("sudo", "rm", "-rf", path); err != nil {
			return fmt.Errorf("sudo rm -rf %s: %w", path, err)
		}
		logger.Verbose("removed residual install directory", "path", path)
		return nil
	}

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

	logger.Debug("removing residual install directory", "path", path, "needs_sudo", false)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("rm -rf %s: %w", path, err)
	}
	logger.Verbose("removed residual install directory", "path", path)
	return nil
}

func uninstallOneAgentWindows(dryRun bool) error {
	header := color.New(color.FgMagenta, color.Bold)
	muted := color.New()

	header.Println("  OneAgent Uninstall (Windows)")
	muted.Println("  " + "────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  Method: WMI product lookup + msiexec /x (quiet)")
	fmt.Println()

	if dryRun {
		fmt.Println("[dry-run] Would look up Dynatrace OneAgent via WMI and run msiexec /x to uninstall. No changes made.")
		return nil
	}

	ok, err := confirmProceed("  Proceed with OneAgent uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Uninstall cancelled.")
		return nil
	}
	fmt.Println()

	// Use PowerShell to look up the OneAgent product GUID via WMI and uninstall via msiexec.
	psScript := `$app = Get-WmiObject win32_product -filter "Name like 'Dynatrace OneAgent'"; if ($app -eq $null) { Write-Error 'Dynatrace OneAgent not found'; exit 1 }; msiexec /x $app.IdentifyingNumber /quiet /l*vx uninstall.log; exit $LASTEXITCODE`

	fmt.Println("  Looking up Dynatrace OneAgent via WMI...")
	if err := RunCommand("powershell", "-NoProfile", "-Command", psScript); err != nil {
		return fmt.Errorf("OneAgent uninstall failed: %w", err)
	}

	color.New(color.FgGreen, color.Bold).Println("\n  OneAgent uninstalled successfully.")
	fmt.Println("  Uninstall log written to uninstall.log in the current directory.")
	return nil
}
