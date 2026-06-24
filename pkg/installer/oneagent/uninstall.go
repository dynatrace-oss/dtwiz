package oneagent

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// UninstallOptions configures the OneAgent V2 uninstall behaviour.
type UninstallOptions struct {
	DryRun bool
}

// UninstallOneAgentV2 removes Dynatrace OneAgent from the current host.
func UninstallOneAgentV2(opts UninstallOptions) error {
	logger.Debug("starting oneagent uninstall", "dry_run", opts.DryRun)

	if !oneAgentInstalled() {
		return fmt.Errorf("OneAgent is not installed — nothing to uninstall")
	}
	logger.Debug("existing OneAgent detected")

	printPlan()

	if opts.DryRun {
		display.PrintStatusLine("dry-run", "no changes made", display.ColorMuted)
		return nil
	}

	ok, err := installer.ConfirmProceed("  Proceed with OneAgent uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		display.PrintStatusLine("result", "uninstall cancelled", display.ColorMuted)
		return installer.ErrInstallCancelled
	}

	if err := runUninstallFn(); err != nil {
		return err
	}

	display.PrintStatusLine("result", "OneAgent uninstalled successfully", display.ColorOK)
	return nil
}
