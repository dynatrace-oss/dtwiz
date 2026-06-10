//go:build !windows

package oneagent

import (
	"os"
	"os/exec"
)

// oneAgentInstallDir is OneAgent's default install location on Unix. Its
// presence is the most reliable "installed" signal: it exists regardless of
// init system (systemd, OpenRC, none) and regardless of whether the agent is
// currently running, and it is removed on a clean uninstall.
// Declared as a var so tests can redirect it to a temporary directory.
var oneAgentInstallDir = "/opt/dynatrace/oneagent"

// oneAgentInstalled reports whether OneAgent appears to be installed on this
// host. It is best-effort — a false negative simply proceeds to a normal
// install — and it deliberately checks for *installation*, not a running
// service, so a stopped-but-installed agent is still detected.
func oneAgentInstalled() bool {
	// Default install directory — instant, no subprocess, covers the common case.
	if fi, err := os.Stat(oneAgentInstallDir); err == nil && fi.IsDir() {
		return true
	}
	// Fallback for custom install paths: oneagentctl reachable on PATH.
	_, err := exec.LookPath("oneagentctl")
	return err == nil
}
