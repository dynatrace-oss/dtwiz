package gcp

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// DynatraceOnlyFlag is the `dtwiz uninstall gcp` flag that selects uninstallDynatraceOnly.
// Defined here so the guidance box and the command name the same flag.
const DynatraceOnlyFlag = "dynatrace-only"

// uninstallDynatraceOnly removes the Dynatrace side of the integration (monitoring
// configurations and connections) without touching gcloud. It is the escape hatch for
// a project that was deleted or is no longer accessible, where the full uninstall can't
// pass the gcloud preflight. GCP resources are left in place and listed in the preview.
func uninstallDynatraceOnly(envURL string, dryRun bool, dtc dtclient) error {
	var monConfigIDs []string
	var conns []connRef
	err := installer.RunConcurrently(
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationPrefix); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationPrefix); return },
	)
	if err != nil {
		return err
	}

	if len(monConfigIDs) == 0 && len(conns) == 0 {
		fmt.Println("  No Dynatrace Google Cloud integration resources found; nothing to uninstall.")
		return nil
	}

	gcpDynatraceOnlyPrintPreview(envURL, monConfigIDs, conns, integrationPrefix+"*", integrationPrefix+"*")

	if proceed, err := installer.ShouldProceed(dryRun, "Uninstall"); !proceed {
		return err
	}

	// No project: the service-account steps are skipped entirely.
	total := uninstallStepCount(monConfigIDs, conns, nil, nil)
	if err := runUninstallSteps(total, "", monConfigIDs, conns, nil, nil, nil, dtc); err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace side of the Google Cloud integration removed.")
	fmt.Println()
	return nil
}

// boundServiceAccounts returns the distinct service-account emails bound to conns, in order.
func boundServiceAccounts(conns []connRef) []string {
	seen := make(map[string]bool)
	var emails []string
	for _, c := range conns {
		if c.serviceAccountEmail != "" && !seen[c.serviceAccountEmail] {
			seen[c.serviceAccountEmail] = true
			emails = append(emails, c.serviceAccountEmail)
		}
	}
	return emails
}

func gcpDynatraceOnlyPrintPreview(envURL string, monConfigIDs []string, conns []connRef, configName, connName string) {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Google Cloud Integration: Uninstall (Dynatrace only)")
	fmt.Println()
	fmt.Printf("  Environment: %s\n", envURL)
	for _, c := range conns {
		fmt.Printf("  Connection:  %s (id: %s)\n", connName, c.objectID)
	}
	for _, id := range monConfigIDs {
		fmt.Printf("  Monitoring config: %s (id: %s)\n", configName, id)
	}
	fmt.Println()
	display.ColorWarning.Println("  GCP resources are left in place. If the project still exists, remove them there:")
	if emails := boundServiceAccounts(conns); len(emails) > 0 {
		for _, email := range emails {
			fmt.Printf("    • Service account %s and its Viewer binding\n", email)
		}
	} else {
		fmt.Println("    • The dtwiz service account and its Viewer binding, if any")
	}
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Steps to be executed:")
	display.PrintSectionDivider()

	for i, s := range gcpUninstallBuildSteps("", monConfigIDs, conns, nil, configName, connName) {
		fmt.Printf("  Step %d: %s\n", i+1, s)
	}

	display.PrintSectionDivider()
	fmt.Println()
}
