//go:build windows

package oneagent

import (
	"os/exec"
	"strings"
)

// oneAgentServicePresentFn reports whether the 'Dynatrace OneAgent' Windows
// service exists (running or stopped). Overridable in tests.
var oneAgentServicePresentFn = func() bool {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"if (Get-Service -Name 'Dynatrace OneAgent' -ErrorAction SilentlyContinue) { 'present' }",
	).Output()
	return err == nil && strings.Contains(string(out), "present")
}

// oneAgentInstalled reports whether OneAgent appears to be installed on this
// host. It is best-effort and deliberately checks for *installation*, not a
// running service, so a stopped-but-installed agent is still detected.
//
// The install-directory check was removed: the MSI uninstaller leaves the
// directory behind (log files, config remnants), causing false positives
// immediately after uninstallation. Service registration and the oneagentctl
// binary are removed promptly by MSI and are reliable indicators.
func oneAgentInstalled() bool {
	// Service registration — present whether the service is Running or Stopped.
	if oneAgentServicePresentFn() {
		return true
	}
	// Fallback for custom install paths: oneagentctl reachable on PATH.
	_, err := exec.LookPath("oneagentctl")
	return err == nil
}
