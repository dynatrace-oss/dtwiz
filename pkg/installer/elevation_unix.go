//go:build !windows

package installer

// IsElevated reports whether the current process has root privileges.
func IsElevated() bool {
	return !NeedsSudo()
}
