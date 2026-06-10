//go:build !windows

package installer

import "os"

// NeedsSudo returns true when the current process is not running as root,
// indicating that the installer needs to be prefixed with sudo.
func NeedsSudo() bool {
	return os.Getuid() != 0
}
