//go:build !windows

package oneagent

import "github.com/dynatrace-oss/dtwiz/pkg/installer"

func runInstaller(argv []string, quiet bool) (int, error) {
	return installer.RunCommandWithExitCode(argv, quiet)
}
