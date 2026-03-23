//go:build !windows

package installer

import "os"

// needsSudo returns true when the current process is not running as root,
// indicating that the installer needs to be prefixed with sudo.
func needsSudo() bool {
	return os.Getuid() != 0
}
