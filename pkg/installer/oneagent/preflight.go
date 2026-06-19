package oneagent

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// oneAgentInstalledFn is the function used to detect an existing installation.
// Overridable in tests.
var oneAgentInstalledFn = oneAgentInstalled

// isElevatedFn reports whether the process has the privileges required to run
// the installer. Overridable in tests.
var isElevatedFn = isElevated

type preflightResult struct {
	IsUpdate bool
}

// runPreflightChecks validates system readiness before any network operation.
// Checks run in order: existing-install detection, update-confirmation prompt,
// sudo availability (Linux non-root only), and elevation check (Windows only).
func runPreflightChecks(env Environment, opts InstallOptions) (preflightResult, error) {
	var result preflightResult

	result.IsUpdate = oneAgentInstalledFn()
	logger.Debug("preflight: oneagent detection", "is_update", result.IsUpdate)

	if result.IsUpdate && !opts.DryRun && !opts.Quiet && !opts.ConnectivityCheckOnly {
		ok, err := installer.ConfirmProceed("  OneAgent is already installed. Update?")
		if err != nil || !ok {
			return preflightResult{}, installer.ErrInstallCancelled
		}
	}

	if env.OS == "linux" && needsSudoFn() {
		if _, err := sudoPathFn(); err != nil {
			return preflightResult{}, fmt.Errorf("sudo not found: install sudo or run dtwiz as root")
		}
		logger.Debug("preflight: sudo available")
	}

	if env.OS == "windows" && !opts.DryRun && !opts.ConnectivityCheckOnly {
		if isElevatedFn() {
			logger.Debug("preflight: running as Administrator")
		} else {
			if opts.Quiet {
				return preflightResult{}, fmt.Errorf("installer requires Administrator privileges: run from an elevated terminal or omit --quiet to allow UAC prompt")
			}
			display.PrintWarning("notice", fmt.Errorf("OneAgent installer will request Administrator privileges via UAC"))
			logger.Debug("preflight: not elevated, UAC prompt will be requested")
		}
	}

	return result, nil
}
