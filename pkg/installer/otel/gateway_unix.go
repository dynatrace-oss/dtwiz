//go:build !windows

package otel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// systemdActiveTimeout bounds how long to wait for a restarted unit to report
// itself active. "systemctl restart" can return success before the service
// has actually finished starting (or before it crashes shortly after), so a
// zero exit status alone is not sufficient evidence of a healthy restart.
const systemdActiveTimeout = 15 * time.Second

// relaunchGracePeriod mirrors startOtelCollector's grace window: long enough
// to catch a process that exits immediately on an obvious misconfiguration,
// short enough not to noticeably delay the update flow.
const relaunchGracePeriod = 3 * time.Second

// isKubernetesPod reports whether pid appears to be running inside a
// Kubernetes pod, detected via its cgroup path. Kubernetes always runs on
// Linux nodes, so this is a no-op (always false) on macOS.
func isKubernetesPod(pid int) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "kubepods")
}

// detectSystemdUnit reports the systemd unit supervising pid, if any,
// distinguished from a generic login-session/user cgroup scope (which
// indicates a bare/manual process, not a dedicated unit). Linux only —
// always returns ("", false) on macOS, which has no systemd.
func detectSystemdUnit(pid int) (unit string, ok bool) {
	if runtime.GOOS != "linux" {
		return "", false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		// cgroup v2 lines look like "0::/system.slice/otelcol-contrib.service";
		// cgroup v1 lines are "<num>:<controller>:/system.slice/otelcol-contrib.service".
		parts := strings.Split(strings.TrimSpace(line), "/")
		for _, p := range parts {
			if strings.HasSuffix(p, ".service") {
				return p, true
			}
		}
	}
	return "", false
}

// restartViaSystemctl restarts a systemd-supervised collector through
// systemd's own tooling rather than killing the process directly, so systemd
// tracks the new PID correctly and honours the unit's Restart= policy. It
// then polls "systemctl is-active" until the unit reports active or
// systemdActiveTimeout elapses — a restart that exits 0 but leaves the unit
// failed or activating is reported as an error, not a false success.
func restartViaSystemctl(unit string) error {
	out, err := exec.Command("systemctl", "restart", unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s failed: %w\n%s", unit, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("restartViaSystemctl: restart issued", "unit", unit)

	deadline := time.Now().Add(systemdActiveTimeout)
	var lastState string
	for {
		stateOut, _ := exec.Command("systemctl", "is-active", unit).Output()
		lastState = strings.TrimSpace(string(stateOut))
		if lastState == "active" {
			logger.Debug("restartViaSystemctl: unit active", "unit", unit)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("unit %s did not become active within %s (last state: %s)", unit, systemdActiveTimeout, lastState)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// captureLaunchContext captures a bare/manual process's full original
// invocation (argv, environment, working directory) on Linux via /proc, so
// it can be faithfully relaunched. Any failure to read argv, environment, or
// cwd yields an incomplete result (complete=false); per design Decision 4, an
// incomplete capture must never be used to attempt an automatic restart.
//
// macOS has no reliable, unprivileged way to read another process's
// environment block, so this always returns an incomplete result there —
// bare/manual collectors on macOS fall back to the manual-restart path.
func captureLaunchContext(pid int) (*launchContext, bool) {
	if runtime.GOOS != "linux" {
		return nil, false
	}

	cmdlineRaw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(cmdlineRaw) == 0 {
		return nil, false
	}
	argv := splitNulSeparated(cmdlineRaw)
	if len(argv) == 0 {
		return nil, false
	}

	environRaw, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		// Commonly permission-denied for a process owned by another user.
		return nil, false
	}
	env := splitNulSeparated(environRaw)

	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return nil, false
	}

	return &launchContext{argv: argv, env: env, cwd: cwd, complete: true}, true
}

// splitNulSeparated splits a NUL-separated byte buffer (as found in
// /proc/<pid>/cmdline and /proc/<pid>/environ) into strings, dropping any
// trailing empty element left by a terminating NUL.
func splitNulSeparated(data []byte) []string {
	parts := strings.Split(string(data), "\x00")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// relaunchWithContext starts a bare/manual collector process using its
// faithfully captured original argv, environment, and working directory —
// never a synthesized "binary --config path" command line.
func relaunchWithContext(lc *launchContext) error {
	logDir := filepath.Dir(lc.argv[0])
	if info, err := os.Stat(logDir); err != nil || !info.IsDir() {
		logDir = os.TempDir()
	}
	logPath := filepath.Join(logDir, "dtwiz-collector-restart.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating restart log file: %w", err)
	}

	cmd := exec.Command(lc.argv[0], lc.argv[1:]...)
	cmd.Dir = lc.cwd
	cmd.Env = lc.env
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("relaunching collector: %w", err)
	}

	pid := cmd.Process.Pid
	crashed := make(chan error, 1)
	go func() { defer logFile.Close(); crashed <- cmd.Wait() }()

	// Give it a moment to fail fast on obvious misconfigurations, mirroring
	// startOtelCollector's own grace-period check — a restart that "starts"
	// but exits immediately must not be reported as a success.
	select {
	case err := <-crashed:
		if err != nil {
			return fmt.Errorf("relaunched collector (PID %d) exited immediately: %w", pid, err)
		}
		return fmt.Errorf("relaunched collector (PID %d) exited immediately with no error (check %s for details)", pid, logPath)
	case <-time.After(relaunchGracePeriod):
		logger.Debug("relaunchWithContext: still running after grace period", "pid", pid)
	}

	fmt.Printf("  Collector relaunched (PID %d).\n", pid)
	return nil
}
