//go:build windows

package installer

// needsSudo always returns false on Windows — the installer exe handles
// privilege elevation itself via its embedded manifest.
func needsSudo() bool {
	return false
}
