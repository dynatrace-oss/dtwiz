//go:build windows

package analyzer

import "strings"

// detectOneAgent checks for a running Dynatrace OneAgent on Windows.
func detectOneAgent() bool {
	// Check whether the OneAgent Windows service is running.
	ok, out := runCmd("powershell", "-NoProfile", "-Command",
		"Get-Service -Name 'Dynatrace OneAgent' -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Status")
	if ok && strings.EqualFold(strings.TrimSpace(out), "Running") {
		return true
	}

	// Check for oneagentctl in PATH (works when the service has a
	// non-standard name or was installed in a custom location).
	ok, _ = runCmd("oneagentctl", "--version")
	return ok
}
