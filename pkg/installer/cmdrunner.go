package installer

import (
	"fmt"
	"os/exec"
	"strings"
)

// CmdRunner runs a command and captures its stdout. It receives the executable
// name, argument slice, and optional environment variables (nil = inherit).
// Shared by the hyperscaler installers (azure, gcp) so `gcloud`/`az` invocations
// can be stubbed out in tests.
type CmdRunner func(name string, args []string, env []string) (stdout string, err error)

// ExecLookPath is a variable alias for exec.LookPath, allowing tests to stub it.
var ExecLookPath = exec.LookPath

// RealRunner is the production CmdRunner implementation.
func RealRunner(name string, args []string, env []string) (string, error) {
	cmd := exec.Command(name, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
	}
	return string(out), err
}

// IsNotFoundErr reports whether err looks like a "resource not found yet" error
// from a cloud CLI (gcloud/az), as opposed to a permanent failure. Used to decide
// whether a not-yet-propagated resource is worth retrying.
func IsNotFoundErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "not_found")
}
