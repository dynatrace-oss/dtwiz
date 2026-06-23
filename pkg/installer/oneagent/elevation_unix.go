//go:build !windows

package oneagent

// isElevated always returns true on non-Windows: privilege checks are handled
// separately via sudo detection.
func isElevated() bool {
	return true
}
