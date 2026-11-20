//go:build windows

package oneagent

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// runUninstallFn is the function called by UninstallOneAgentV2 to execute the
// platform-specific uninstall logic. Overridable in tests.
var runUninstallFn = runUninstall

// runCommandFn executes the uninstall subprocess. Overridable in tests to
// capture argv without spawning a real process.
var runCommandFn = installer.RunCommandQuiet

func printPlan(showPlan bool) {
	display.PrintAlignedStatusLines(display.ColorDefault, "Removes", "Dynatrace OneAgent from this host")
	if showPlan {
		display.Println("Method:     WMI product lookup + msiexec /x (quiet)")
	} else {
		display.Detail("method: WMI product lookup + msiexec /x (quiet)")
	}
}

// primeUninstallSudo is a no-op on Windows (elevation is requested by msiexec).
func primeUninstallSudo() error { return nil }

func runUninstall() error {
	psScript := `$app = Get-WmiObject win32_product -filter "Name like 'Dynatrace OneAgent'"; if ($app -eq $null) { Write-Error 'Dynatrace OneAgent not found'; exit 1 }; $log = Join-Path (Get-Location).Path 'uninstall.log'; $q = [char]34; $p = Start-Process msiexec -ArgumentList @("/x $($app.IdentifyingNumber)", '/quiet', "/l*vx $q$log$q") -Verb RunAs -WorkingDirectory (Get-Location).Path -Wait -PassThru; exit $p.ExitCode`

	logger.Debug("running WMI uninstall", "method", "msiexec")
	display.Detail("looking up Dynatrace OneAgent via WMI")

	if err := runCommandFn("powershell", "-NoProfile", "-Command", psScript); err != nil {
		return fmt.Errorf("OneAgent uninstall failed: %w", err)
	}
	logger.Verbose("WMI uninstall completed")

	display.Detail("uninstall.log written to current directory")
	return nil
}
