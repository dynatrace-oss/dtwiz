package azure

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

func connectionExistsWithClient(dtc dtclient) (bool, error) {
	conns, err := dtc.findAllConnections(integrationName)
	return len(conns) > 0, err
}

// MonitoringConfigExists reports whether an Azure monitoring configuration named integrationName exists.
func MonitoringConfigExists(envURL, platformToken string) (bool, error) {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return false, err
	}
	ids, err := dtc.findAllMonitoringConfigs(integrationName)
	return len(ids) > 0, err
}

// azureGatherClientIDs collects client IDs to delete. Connection-bound IDs are trusted directly;
// display-name matches are verified via federated credential (Entra names aren't unique).
func azureGatherClientIDs(runner cmdRunner, conns []connRef, name, envURL string) []string {
	set := make(map[string]bool)
	for _, c := range conns {
		if c.clientID != "" {
			set[c.clientID] = true // trusted: bound to a dtwiz connection
		}
	}

	ids, err := azureListAppIDsByName(runner, name)
	if err != nil {
		logger.Debug("az app list failed during cleanup, continuing", "err", err)
	} else {
		issuer, err := azureIssuerURL(envURL)
		if err != nil {
			display.ColorWarning.Printf("  Warning: skipping App Registration display-name cleanup: %v\n", err)
			ids = nil
		}
		for _, id := range ids {
			if set[id] {
				continue // already trusted via a connection
			}
			ok, verr := azureAppHasDtwizFedCred(runner, id, issuer)
			if verr != nil {
				display.ColorWarning.Printf("  Warning: skipping App Registration %s named %q: could not verify it was created by dtwiz (%v)\n", id, name, verr)
				continue
			}
			if !ok {
				display.ColorWarning.Printf("  Warning: skipping App Registration %s named %q: it lacks the dtwiz federated credential, so it was not created by dtwiz\n", id, name)
				continue
			}
			set[id] = true
		}
	}

	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func UninstallAzure(envURL, platformToken string, dryRun bool) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return uninstallAzureWithRunner(envURL, dryRun, realRunner, dtc)
}

func uninstallAzureWithRunner(envURL string, dryRun bool, runner cmdRunner, dtc dtclient) error {
	var monConfigIDs []string
	var conns []connRef
	err := installer.RunConcurrently(
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationName); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationName); return },
	)
	if err != nil {
		return err
	}
	clientIDs := azureGatherClientIDs(runner, conns, integrationName, envURL)

	if len(monConfigIDs) == 0 && len(conns) == 0 && len(clientIDs) == 0 {
		fmt.Println("  No Azure Monitor integration resources found; nothing to uninstall.")
		return nil
	}

	azureUninstallPrintPreview(envURL, monConfigIDs, conns, clientIDs, integrationName, integrationName)

	if proceed, err := installer.ShouldProceed(dryRun, "Uninstall"); !proceed {
		return err
	}

	totalSteps := uninstallStepCount(monConfigIDs, conns, clientIDs)
	if err := runUninstallSteps(0, totalSteps, monConfigIDs, conns, clientIDs, runner, dtc); err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration removed.")
	fmt.Println()
	return nil
}

func uninstallStepCount(monConfigIDs []string, conns []connRef, clientIDs []string) int {
	// 2 steps per app (role assignment delete + app registration delete).
	return len(monConfigIDs) + len(clientIDs)*2 + len(conns)
}

// runUninstallSteps executes deletion steps; offset shifts step numbers when called mid-sequence (e.g. reinstall).
func runUninstallSteps(offset, total int, monConfigIDs []string, conns []connRef, clientIDs []string, runner cmdRunner, dtc dtclient) error {
	step := offset
	var errs []error

	for _, id := range monConfigIDs {
		step++
		fmt.Printf("  Step %d/%d: Delete Azure monitoring configuration...\n", step, total)
		if err := dtc.deleteMonitoring(id); err != nil {
			display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
			errs = append(errs, fmt.Errorf("step %d: %w", step, err))
			continue
		}
		display.ColorOK.Println("  ✓ Monitoring configuration deleted")
	}

	for _, clientID := range clientIDs {
		step++
		fmt.Printf("  Step %d/%d: Delete Monitoring Reader role assignment...\n", step, total)
		if err := azureDeleteRoleAssignment(runner, clientID); err != nil {
			display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
			errs = append(errs, fmt.Errorf("step %d: %w", step, err))
		} else {
			display.ColorOK.Println("  ✓ Role assignment deleted")
		}

		step++
		fmt.Printf("  Step %d/%d: Delete Azure App Registration...\n", step, total)
		if err := azureDeleteApp(runner, clientID); err != nil {
			display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
			errs = append(errs, fmt.Errorf("step %d: %w", step, err))
		} else {
			display.ColorOK.Println("  ✓ App Registration deleted (Service Principal + federated credential removed)")
		}
	}

	for _, c := range conns {
		step++
		fmt.Printf("  Step %d/%d: Delete Dynatrace Azure connection...\n", step, total)
		if err := dtc.deleteConnection(c.objectID); err != nil {
			display.ColorWarning.Printf("  Warning: step %d failed: %v\n", step, err)
			errs = append(errs, fmt.Errorf("step %d: %w", step, err))
		} else {
			display.ColorOK.Println("  ✓ Connection deleted")
		}
	}

	return errors.Join(errs...)
}

// azureUninstallBuildSteps builds step descriptions; also used in the combined update preview.
func azureUninstallBuildSteps(monConfigIDs []string, conns []connRef, clientIDs []string, configName, connName string) []string {
	var steps []string
	for range monConfigIDs {
		steps = append(steps, fmt.Sprintf("DT Extensions API: delete monitoring configuration '%s'", configName))
	}
	for _, clientID := range clientIDs {
		steps = append(steps, fmt.Sprintf("az role assignment delete --assignee %s --role \"Monitoring Reader\"", clientID))
		steps = append(steps, fmt.Sprintf("az ad app delete --id %s  (removes Service Principal + federated credential)", clientID))
	}
	for _, c := range conns {
		steps = append(steps, fmt.Sprintf("DT Settings API: delete Azure connection '%s' (id: %s)", connName, c.objectID))
	}
	return steps
}

func azureUninstallPrintPreview(envURL string, monConfigIDs []string, conns []connRef, clientIDs []string, configName, connName string) {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Azure Monitor Integration: Uninstall")
	fmt.Println()
	fmt.Printf("  Environment: %s\n", envURL)
	if len(conns) > 0 {
		for _, c := range conns {
			fmt.Printf("  Connection:  %s (id: %s)\n", connName, c.objectID)
		}
	} else {
		fmt.Printf("  Connection:  %s: not found, skipping\n", connName)
	}
	for _, clientID := range clientIDs {
		fmt.Printf("  App Registration: %s\n", clientID)
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

	for i, s := range azureUninstallBuildSteps(monConfigIDs, conns, clientIDs, configName, connName) {
		fmt.Printf("  Step %d: %s\n", i+1, s)
	}

	display.PrintSectionDivider()
	fmt.Println()
}
