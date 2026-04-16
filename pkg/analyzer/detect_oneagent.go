//go:build !windows

package analyzer

// detectOneAgent checks for a running Dynatrace OneAgent on Unix systems.
func detectOneAgent() bool {
	// Check whether the oneagent service is active.
	ok, _ := runCmd("systemctl", "is-active", "--quiet", "oneagent")
	if ok {
		return true
	}
	// Check for oneagentctl in PATH.
	ok, _ = runCmd("oneagentctl", "--version")
	return ok
}
