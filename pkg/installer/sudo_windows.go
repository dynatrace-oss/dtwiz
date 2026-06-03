//go:build windows

package installer

// NeedsSudo always returns false on Windows — the installer exe handles
// privilege elevation itself via its embedded manifest.
func NeedsSudo() bool {
	return false
}
