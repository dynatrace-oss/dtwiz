//go:build !windows

package installer

// RefreshWindowsPath is a no-op outside Windows.
func RefreshWindowsPath() error {
	return nil
}
