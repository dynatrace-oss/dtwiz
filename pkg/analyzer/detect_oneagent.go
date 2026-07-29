//go:build !windows

package analyzer

import "os"

// systemdAvailable reports whether systemd is the running init system.
// /run/systemd/system exists as a directory only when systemd is booted
// (see sd_booted(3)). In containers such as GitHub Codespaces, systemctl
// is a compatibility shim that exits 0 for any invocation, so its exit
// code must not be trusted unless systemd is actually running.
func systemdAvailable() bool {
	fi, err := os.Stat("/run/systemd/system")
	return err == nil && fi.IsDir()
}

// detectOneAgent checks for a running Dynatrace OneAgent on Unix systems.
func detectOneAgent() bool {
	// Check whether the oneagent service is active.
	if systemdAvailable() {
		ok, _ := runCmd("systemctl", "is-active", "--quiet", "oneagent")
		if ok {
			return true
		}
	}
	// Check for oneagentctl in PATH.
	ok, _ := runCmd("oneagentctl", "--version")
	return ok
}
