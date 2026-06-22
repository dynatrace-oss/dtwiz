//go:build windows

package oneagent

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

func printPlan() {
	fmt.Println("  Method:     WMI product lookup + msiexec /x (quiet)")
}

func runUninstall() error {
	psScript := `$app = Get-WmiObject win32_product -filter "Name like 'Dynatrace OneAgent'"; if ($app -eq $null) { Write-Error 'Dynatrace OneAgent not found'; exit 1 }; $p = Start-Process msiexec -ArgumentList "/x $($app.IdentifyingNumber) /quiet /l*vx uninstall.log" -Verb RunAs -Wait -PassThru; exit $p.ExitCode`

	logger.Debug("running WMI uninstall", "method", "msiexec")
	display.PrintStatusLine("uninstall", "looking up Dynatrace OneAgent via WMI...", display.ColorMessage)

	if err := installer.RunCommand("powershell", "-NoProfile", "-Command", psScript); err != nil {
		return fmt.Errorf("OneAgent uninstall failed: %w", err)
	}
	logger.Verbose("WMI uninstall completed")

	display.PrintStatusLine("log", "uninstall.log written to current directory", display.ColorMuted)
	return nil
}
