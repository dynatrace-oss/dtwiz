//go:build !windows

package analyzer

import "os"

// detectOneAgent checks for a Dynatrace OneAgent installation on Unix systems.
func detectOneAgent() bool {
	// Check the default Linux install path.
	if _, err := os.Stat("/opt/dynatrace/oneagent"); err == nil {
		return true
	}
	// Check for oneagentctl in PATH.
	ok, _ := runCmd("oneagentctl", "--version")
	return ok
}
