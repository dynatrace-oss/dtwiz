package gcp

import (
	"errors"
	"fmt"
	"sort"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

func ConnectionExists(envURL, platformToken string) (bool, error) {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return false, err
	}
	return connectionExistsWithClient(dtc)
}

// connectionExistsWithClient reports whether a fully-configured connection exists — an
// incomplete one (left by an interrupted install) is not considered "configured" so
// `dtwiz setup` routes back to install, which resumes it, rather than to update, which
// would reject it. Uses prefix matching so it finds connections under either the old
// fixed name or the current env-scoped name.
func connectionExistsWithClient(dtc dtclient) (bool, error) {
	conns, err := dtc.findAllConnections(integrationPrefix)
	if err != nil {
		return false, err
	}
	complete, _ := splitConnectionsByCompleteness(conns)
	return len(complete) > 0, nil
}

// gcpGatherServiceAccounts splits SA emails into current and legacy sets.
// Current: emails bound to discovered connections plus the deterministic email for the
// env-scoped name. Legacy: the deterministic email for the old fixed integrationPrefix
// name, only when not already in the current set.
// Legacy cleanup failures are warn-only so an unowned old SA cannot block cleanup of
// the current integration.
func gcpGatherServiceAccounts(conns []connRef, currentName, projectID string) (current, legacy []string) {
	currentSet := make(map[string]bool)
	for _, c := range conns {
		if c.serviceAccountEmail != "" && !currentSet[c.serviceAccountEmail] {
			currentSet[c.serviceAccountEmail] = true
			current = append(current, c.serviceAccountEmail)
		}
	}
	if projectID != "" {
		if email := gcpServiceAccountEmail(currentName, projectID); !currentSet[email] {
			currentSet[email] = true
			current = append(current, email)
		}
		if legacyEmail := gcpServiceAccountEmail(integrationPrefix, projectID); !currentSet[legacyEmail] {
			legacy = append(legacy, legacyEmail)
		}
	}
	sort.Strings(current)
	sort.Strings(legacy)
	return
}

// MonitoringConfigExists reports whether any GCP monitoring configuration with the
// dtwiz prefix exists. Uses prefix matching so it finds configs under either the old
// fixed name or the current env-scoped name.
func MonitoringConfigExists(envURL, platformToken string) (bool, error) {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return false, err
	}
	ids, err := dtc.findAllMonitoringConfigs(integrationPrefix)
	return len(ids) > 0, err
}

func UninstallGCP(envURL, platformToken string, dryRun bool) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return uninstallGCPWithRunner(envURL, dryRun, realRunner, dtc)
}

func uninstallGCPWithRunner(envURL string, dryRun bool, runner cmdRunner, dtc dtclient) error {
	var monConfigIDs []string
	var conns []connRef
	err := installer.RunConcurrentlyWithSpinner("Looking up Google Cloud integration resources",
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationPrefix); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationPrefix); return },
	)
	if err != nil {
		return err
	}

	// Project ID is best-effort: cleanup proceeds even without an active gcloud project.
	projectID, _, err := gcpAccountInfo(runner)
	if err != nil {
		logger.Debug("gcloud project unavailable during cleanup, continuing", "err", err)
	}

	currentName := integrationNameForEnv(envURL)
	currentSAEmails, legacySAEmails := gcpGatherServiceAccounts(conns, currentName, projectID)
	var allSAEmails = append(currentSAEmails, legacySAEmails...)

	if len(monConfigIDs) == 0 && len(conns) == 0 {
		fmt.Println("  No Google Cloud integration resources found; nothing to uninstall.")
		return nil
	}

	gcpUninstallPrintPreview(envURL, projectID, monConfigIDs, conns, allSAEmails, integrationPrefix+"*", integrationPrefix+"*", dryRun)

	if proceed, err := installer.ShouldProceed(dryRun, "Uninstall"); !proceed {
		return err
	}

	totalSteps := uninstallStepCount(monConfigIDs, conns, currentSAEmails, legacySAEmails, projectID)
	if err := runUninstallSteps(totalSteps, projectID, monConfigIDs, conns, currentSAEmails, legacySAEmails, runner, dtc); err != nil {
		return err
	}

	return nil
}

func uninstallStepCount(monConfigIDs []string, conns []connRef, currentSAEmails, legacySAEmails []string, projectID string) int {
	saSteps := 0
	if projectID != "" {
		saSteps = (len(currentSAEmails) + len(legacySAEmails)) * 2
	}
	return len(monConfigIDs) + saSteps + len(conns)
}

// runUninstallSteps executes deletion steps in order, printing progress as "Step N/total".
// Current SA cleanup failures are fatal; legacy SA cleanup failures are warn-only so that
// an SA created under the old fixed name by a different user cannot block removal of the
// current integration.
func runUninstallSteps(total int, projectID string, monConfigIDs []string, conns []connRef, currentSAEmails, legacySAEmails []string, runner cmdRunner, dtc dtclient) error {
	step := 0
	var errs []error

	// run shows one step. Hard failures are collected as errors; soft failures
	// (legacy resources, possibly created by someone else) only warn.
	run := func(label string, hard bool, fn func() error) {
		step++
		logger.Debug("uninstall step", "step", step, "total", total, "label", label)
		if hard {
			if err := display.RunStep(label, func(*display.Step) error { return fn() }); err != nil {
				errs = append(errs, fmt.Errorf("step %d: %w", step, err))
			}
			return
		}
		if err := fn(); err != nil {
			display.StepWarn("Skipped "+label, "legacy cleanup failed (continuing): "+err.Error())
			return
		}
		display.StepDone(label, "")
	}

	for _, id := range monConfigIDs {
		run("Deleted monitoring configuration", true, func() error { return dtc.deleteMonitoring(id) })
	}

	// Service-account cleanup only runs with an active gcloud project to target.
	if projectID != "" {
		for _, email := range currentSAEmails {
			run("Removed project IAM binding", true, func() error {
				return gcpRemoveProjectBinding(runner, projectID, serviceAccountMember(email), viewerRole)
			})
			run("Deleted service account", true, func() error { return gcpDeleteServiceAccount(runner, email, projectID) })
		}

		for _, email := range legacySAEmails {
			run("Removed legacy project IAM binding", false, func() error {
				return gcpRemoveProjectBinding(runner, projectID, serviceAccountMember(email), viewerRole)
			})
			run("Deleted legacy service account", false, func() error { return gcpDeleteServiceAccount(runner, email, projectID) })
		}
	}

	for _, c := range conns {
		run("Deleted Dynatrace connection", true, func() error { return dtc.deleteConnection(c.objectID) })
	}

	return errors.Join(errs...)
}

// gcpUninstallBuildSteps builds step descriptions for the uninstall preview.
func gcpUninstallBuildSteps(projectID string, monConfigIDs []string, conns []connRef, saEmails []string, configName, connName string) []string {
	var steps []string
	for range monConfigIDs {
		steps = append(steps, fmt.Sprintf("DT Extensions API: delete monitoring configuration '%s'", configName))
	}
	if projectID != "" {
		for _, email := range saEmails {
			steps = append(steps, fmt.Sprintf("gcloud projects remove-iam-policy-binding %s --member=%s --role=%s",
				projectID, serviceAccountMember(email), viewerRole))
			steps = append(steps, fmt.Sprintf("gcloud iam service-accounts delete %s --project=%s  (removes impersonation binding)",
				email, projectID))
		}
	}
	for _, c := range conns {
		steps = append(steps, fmt.Sprintf("DT Settings API: delete GCP connection '%s' (id: %s)", connName, c.objectID))
	}
	return steps
}

func gcpUninstallPrintPreview(envURL, projectID string, monConfigIDs []string, conns []connRef, saEmails []string, configName, connName string, showPlan bool) {
	fmt.Println()
	display.Header("dtwiz · Google Cloud uninstall")
	rows := []string{}
	if projectID != "" {
		rows = append(rows, "Project", projectID)
	}
	for range conns {
		rows = append(rows, "Removes", "Dynatrace GCP connection")
	}
	if projectID != "" && len(saEmails) > 0 {
		rows = append(rows, "Removes", fmt.Sprintf("%d service account(s) and IAM binding(s)", len(saEmails)))
	}
	for range monConfigIDs {
		rows = append(rows, "Removes", "GCP monitoring configuration")
	}
	display.PrintAlignedStatusLines(display.ColorDefault, rows...)
	display.Detail("environment: %s", envURL)
	for _, c := range conns {
		display.Detail("connection: %s (id: %s)", connName, c.objectID)
	}
	if projectID != "" {
		for _, email := range saEmails {
			display.Detail("service account: %s", email)
		}
	}
	for _, id := range monConfigIDs {
		display.Detail("monitoring config: %s (id: %s)", configName, id)
	}

	if showPlan || logger.Enabled() {
		fmt.Println()
		display.PrintSectionDivider()
		display.ColorMessage.Println("  Steps to be executed:")
		display.PrintSectionDivider()
		for i, s := range gcpUninstallBuildSteps(projectID, monConfigIDs, conns, saEmails, configName, connName) {
			fmt.Printf("  Step %d: %s\n", i+1, s)
		}
		display.PrintSectionDivider()
	}
	fmt.Println()
}
