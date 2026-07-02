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
// would reject it.
func connectionExistsWithClient(dtc dtclient) (bool, error) {
	conns, err := dtc.findAllConnections(integrationName)
	if err != nil {
		return false, err
	}
	complete, _ := splitConnectionsByCompleteness(conns)
	return len(complete) > 0, nil
}

// gcpGatherServiceAccounts collects the SA emails to clean up: those bound to a
// dtwiz connection plus the deterministic email dtwiz would have created.
func gcpGatherServiceAccounts(conns []connRef, projectID string) []string {
	set := make(map[string]bool)
	for _, c := range conns {
		if c.serviceAccountEmail != "" {
			set[c.serviceAccountEmail] = true
		}
	}
	if projectID != "" {
		set[gcpServiceAccountEmail(serviceAccountName, projectID)] = true
	}
	out := make([]string, 0, len(set))
	for email := range set {
		out = append(out, email)
	}
	sort.Strings(out)
	return out
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
	err := installer.RunConcurrently(
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationName); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationName); return },
	)
	if err != nil {
		return err
	}

	// Project ID is best-effort: cleanup proceeds even without an active gcloud project.
	projectID, _, err := gcpAccountInfo(runner)
	if err != nil {
		logger.Debug("gcloud project unavailable during cleanup, continuing", "err", err)
	}
	saEmails := gcpGatherServiceAccounts(conns, projectID)

	if len(monConfigIDs) == 0 && len(conns) == 0 {
		fmt.Println("  No Google Cloud integration resources found; nothing to uninstall.")
		return nil
	}

	gcpUninstallPrintPreview(envURL, projectID, monConfigIDs, conns, saEmails, integrationName, integrationName)

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	ok, err := installer.ConfirmProceed("  Apply?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Uninstall cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	totalSteps := uninstallStepCount(monConfigIDs, conns, saEmails, projectID)
	if err := runUninstallSteps(totalSteps, projectID, monConfigIDs, conns, saEmails, runner, dtc); err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Google Cloud integration removed.")
	fmt.Println()
	return nil
}

func uninstallStepCount(monConfigIDs []string, conns []connRef, saEmails []string, projectID string) int {
	saSteps := 0
	if projectID != "" {
		// per SA: remove project viewer binding + delete service account.
		saSteps = len(saEmails) * 2
	}
	return len(monConfigIDs) + saSteps + len(conns)
}

// runUninstallSteps executes deletion steps in order, printing progress as "Step N/total".
func runUninstallSteps(total int, projectID string, monConfigIDs []string, conns []connRef, saEmails []string, runner cmdRunner, dtc dtclient) error {
	step := 0
	var errs []error

	for _, id := range monConfigIDs {
		step++
		fmt.Printf("  Step %d/%d: Delete GCP monitoring configuration...\n", step, total)
		if err := dtc.deleteMonitoring(id); err != nil {
			display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
			errs = append(errs, fmt.Errorf("step %d: %w", step, err))
			continue
		}
		display.ColorOK.Println("  ✓ Monitoring configuration deleted")
	}

	// Service-account cleanup only runs with an active gcloud project to target.
	if projectID != "" {
		for _, email := range saEmails {
			step++
			fmt.Printf("  Step %d/%d: Remove project IAM binding...\n", step, total)
			if err := gcpRemoveProjectBinding(runner, projectID, serviceAccountMember(email), viewerRole); err != nil {
				display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
				errs = append(errs, fmt.Errorf("step %d: %w", step, err))
			} else {
				display.ColorOK.Println("  ✓ Project IAM binding removed")
			}

			step++
			fmt.Printf("  Step %d/%d: Delete Google Cloud service account...\n", step, total)
			if err := gcpDeleteServiceAccount(runner, email, projectID); err != nil {
				display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
				errs = append(errs, fmt.Errorf("step %d: %w", step, err))
			} else {
				display.ColorOK.Println("  ✓ Service account deleted (impersonation binding removed)")
			}
		}
	}

	for _, c := range conns {
		step++
		fmt.Printf("  Step %d/%d: Delete Dynatrace GCP connection...\n", step, total)
		if err := dtc.deleteConnection(c.objectID); err != nil {
			display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
			errs = append(errs, fmt.Errorf("step %d: %w", step, err))
		} else {
			display.ColorOK.Println("  ✓ Connection deleted")
		}
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

func gcpUninstallPrintPreview(envURL, projectID string, monConfigIDs []string, conns []connRef, saEmails []string, configName, connName string) {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Google Cloud Integration: Uninstall")
	fmt.Println()
	fmt.Printf("  Environment: %s\n", envURL)
	if projectID != "" {
		fmt.Printf("  Project:     %s\n", projectID)
	}
	if len(conns) > 0 {
		for _, c := range conns {
			fmt.Printf("  Connection:  %s (id: %s)\n", connName, c.objectID)
		}
	} else {
		fmt.Printf("  Connection:  %s: not found, skipping\n", connName)
	}
	if projectID != "" {
		for _, email := range saEmails {
			fmt.Printf("  Service account: %s\n", email)
		}
	}
	if len(monConfigIDs) > 0 {
		for _, id := range monConfigIDs {
			fmt.Printf("  Monitoring config: %s (id: %s)\n", configName, id)
		}
	} else {
		fmt.Printf("  Monitoring config: %s: not found, skipping\n", configName)
	}
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Steps to be executed:")
	display.PrintSectionDivider()

	for i, s := range gcpUninstallBuildSteps(projectID, monConfigIDs, conns, saEmails, configName, connName) {
		fmt.Printf("  Step %d: %s\n", i+1, s)
	}

	display.PrintSectionDivider()
	fmt.Println()
}
