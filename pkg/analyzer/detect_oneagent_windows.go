//go:build windows

package analyzer

import "os"

// detectOneAgent checks for a Dynatrace OneAgent installation on Windows.
func detectOneAgent() bool {
	// Check the default Windows install paths.
	windowsPaths := []string{
		os.Getenv("ProgramFiles") + `\dynatrace\oneagent`,
		os.Getenv("ProgramFiles(x86)") + `\dynatrace\oneagent`,
		`C:\ProgramData\dynatrace\oneagent`,
	}
	for _, p := range windowsPaths {
		if p == `\dynatrace\oneagent` {
			// env var was empty
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}

	// Check for the OneAgent Windows service.
	ok, _ := runCmd("powershell", "-NoProfile", "-Command",
		"Get-Service -Name 'Dynatrace OneAgent' -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Status")
	if ok {
		return true
	}

	// Check for oneagentctl in PATH.
	ok, _ = runCmd("oneagentctl", "--version")
	return ok
}
