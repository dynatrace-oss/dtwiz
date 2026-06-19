//go:build windows

package analyzer

import "strings"

// detectOneAgent checks for a running Dynatrace OneAgent on Windows.
//
// Only the service-running check is used. The oneagentctl binary fallback was
// removed: after MSI uninstallation, binaries may persist on disk (pending
// deletion until reboot) while the service is removed immediately, making the
// binary check an unreliable indicator of a running agent.
func detectOneAgent() bool {
	ok, out := runCmd("powershell", "-NoProfile", "-Command",
		"Get-Service -Name 'Dynatrace OneAgent' -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Status")
	return ok && strings.EqualFold(strings.TrimSpace(out), "Running")
}
