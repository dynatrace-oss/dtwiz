package installer

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fatih/color"
)

const (
	// Linux: standard uninstall script path.
	linuxUninstallScript = "/opt/dynatrace/oneagent/agent/uninstall.sh"
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
	// Verify the uninstall script exists.
	if _, err := os.Stat(linuxUninstallScript); os.IsNotExist(err) {
		return fmt.Errorf("OneAgent uninstall script not found at %s — is OneAgent installed?", linuxUninstallScript)
	}

	header := color.New(color.FgMagenta, color.Bold)
	muted := color.New()

	header.Println("  OneAgent Uninstall (Linux)")
	muted.Println("  " + "────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  Uninstall script:  %s\n", linuxUninstallScript)

	if needsSudo() {
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

	// Run the uninstall script, prepending sudo if needed.
	args := []string{linuxUninstallScript}
	if needsSudo() {
		args = append([]string{"sudo"}, args...)
	}

	fmt.Println("  Running OneAgent uninstall script...")
	if err := RunCommand(args[0], args[1:]...); err != nil {
		return fmt.Errorf("OneAgent uninstall failed: %w", err)
	}

	color.New(color.FgGreen, color.Bold).Println("\n  OneAgent uninstalled successfully.")
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
