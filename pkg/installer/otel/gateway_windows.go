//go:build windows

package otel

import "fmt"

// isKubernetesPod is always false on Windows: Kubernetes worker nodes running
// Windows containers are supervised via the container runtime, which is
// already detected generically (collectorInstance.containerRuntime), not via
// this cgroup-based check.
func isKubernetesPod(_ int) bool {
	return false
}

// detectSystemdUnit is always ("", false) on Windows, which has no systemd.
// Windows service supervision is not detected in this version; a
// service-supervised collector falls back to the manual-restart path.
func detectSystemdUnit(_ int) (unit string, ok bool) {
	return "", false
}

// captureLaunchContext is always incomplete on Windows: there is no simple,
// unprivileged way to read another process's full environment block (unlike
// /proc/<pid>/environ on Linux), so a faithful relaunch cannot be guaranteed.
// Bare/manual collectors on Windows fall back to the manual-restart path.
func captureLaunchContext(_ int) (*launchContext, bool) {
	return nil, false
}

// restartViaSystemctl is never called on Windows (detectSystemdUnit always
// returns false), but is defined to satisfy gateway.go's cross-platform
// restartForeignCollector dispatch.
func restartViaSystemctl(_ string) error {
	return fmt.Errorf("not supported on Windows in this version")
}

// relaunchWithContext is never called on Windows (captureLaunchContext always
// returns incomplete), but is defined to satisfy gateway.go's cross-platform
// restartForeignCollector dispatch.
func relaunchWithContext(_ *launchContext) error {
	return fmt.Errorf("not supported on Windows in this version")
}
