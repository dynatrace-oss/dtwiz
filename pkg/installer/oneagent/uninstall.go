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
	Quiet  bool
}

// UninstallOneAgentV2 removes Dynatrace OneAgent from the current host.
func UninstallOneAgentV2(opts UninstallOptions) error {
	logger.Debug("starting oneagent uninstall", "dry_run", opts.DryRun)

	if !oneAgentInstalled() {
		return fmt.Errorf("OneAgent is not installed — nothing to uninstall")
	}
	logger.Debug("existing OneAgent detected")

	display.InstallHeader("dtwiz · OneAgent uninstall")
	printPlan(opts.DryRun)

	if opts.DryRun {
		display.PrintStatusLine("dry-run", "no changes made", display.ColorMuted)
		return nil
	}

	if !opts.Quiet {
		ok, err := installer.ConfirmProceed("  Apply?")
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		if !ok {
			display.PrintStatusLine("result", "uninstall cancelled", display.ColorMuted)
			return installer.ErrInstallCancelled
		}
	}

	// sudo may prompt for a password, which a spinner would overwrite.
	if !opts.Quiet {
		if err := primeUninstallSudo(); err != nil {
			return err
		}
	}

	return display.RunStep("Uninstalled OneAgent", func(*display.Step) error {
		return runUninstallFn()
	})
}
