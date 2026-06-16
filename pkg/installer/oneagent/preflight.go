package oneagent

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// oneAgentInstalledFn is the function used to detect an existing installation.
// Overridable in tests.
var oneAgentInstalledFn = oneAgentInstalled

type preflightResult struct {
	IsUpdate bool
}

// runPreflightChecks validates system readiness before any network operation.
// Checks run in order: existing-install detection, update-confirmation prompt,
// and sudo availability (Linux non-root only).
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

	return result, nil
}
