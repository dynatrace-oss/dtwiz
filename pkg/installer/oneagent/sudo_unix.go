//go:build !windows

package oneagent

import "os"

func needsSudo() bool {
	return os.Getuid() != 0
}
