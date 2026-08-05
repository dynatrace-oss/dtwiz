//go:build !windows

package analyzer

import "os"

// systemdRunDir is the directory whose presence indicates that systemd is the
// running init system (see sd_booted(3)). Overridable in tests.
var systemdRunDir = "/run/systemd/system"

// systemdAvailable reports whether systemd is the running init system.
// In containers (devcontainer images, GitHub Codespaces, plain docker runs of
// those images), systemctl is often a compatibility shim that exits 0 for any
// invocation, so its exit code must not be trusted unless systemd is actually
// running.
func systemdAvailable() bool {
	fi, err := os.Stat(systemdRunDir)
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
