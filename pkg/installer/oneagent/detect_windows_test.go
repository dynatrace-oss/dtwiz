//go:build windows

package oneagent

import "testing"

// withInstallDir is a no-op on Windows: oneAgentInstallDir is defined in the
// Unix build only, so there is nothing to override.
func withInstallDir(t *testing.T, _ string) {
	t.Helper()
}
