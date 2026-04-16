//go:build !windows

package installer

// adoptExeclChildren is a no-op on non-Windows platforms.
// The os.execl child-adoption is only needed on Windows.
func adoptExeclChildren(_ []*ManagedProcess, _, _ *int) {}
