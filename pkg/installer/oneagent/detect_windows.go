//go:build windows

package oneagent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// oneAgentInstalled reports whether OneAgent appears to be installed on this
// host. It is best-effort and deliberately checks for *installation*, not a
// running service, so a stopped-but-installed agent is still detected.
func oneAgentInstalled() bool {
	// Default install directory under Program Files — instant, no subprocess.
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		if fi, err := os.Stat(filepath.Join(pf, "dynatrace", "oneagent")); err == nil && fi.IsDir() {
			return true
		}
	}
	// Service registration — present whether the service is Running or Stopped.
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"if (Get-Service -Name 'Dynatrace OneAgent' -ErrorAction SilentlyContinue) { 'present' }",
	).Output()
	if err == nil && strings.Contains(string(out), "present") {
		return true
	}
	// Fallback for custom install paths: oneagentctl reachable on PATH.
	_, err = exec.LookPath("oneagentctl")
	return err == nil
}
