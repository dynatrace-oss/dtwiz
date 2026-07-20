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
	conns, err := dtc.findAllConnections(integrationPrefix)
	return len(conns) > 0, err
}

// MonitoringConfigExists reports whether an Azure monitoring configuration exists.
func MonitoringConfigExists(envURL, platformToken string) (bool, error) {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return false, err
	}
	ids, err := dtc.findAllMonitoringConfigs(integrationPrefix)
	return len(ids) > 0, err
}

// azureGatherClientIDs collects client IDs to delete. Connection-bound IDs are trusted directly;
// display-name matches are verified via federated credential (Entra names aren't unique).
func azureGatherClientIDs(runner cmdRunner, conns []connRef, names []string, envURL string) []string {
	set := make(map[string]bool)
	for _, c := range conns {
		if c.clientID != "" {
			set[c.clientID] = true // trusted: bound to a dtwiz connection
		}
	}

	issuer, issuerErr := azureIssuerURL(envURL)
	if issuerErr != nil {
		display.ColorWarning.Printf("  Warning: skipping App Registration display-name cleanup: %v\n", issuerErr)
	} else {
		for _, name := range names {
			ids, err := azureListAppIDsByName(runner, name)
			if err != nil {
				logger.Debug("az app list failed during cleanup, continuing", "name", name, "err", err)
				continue
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
	if err := installer.RunConcurrently(
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationPrefix); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationPrefix); return },
	); err != nil {
		return err
	}

	currentName := integrationNameForEnv(envURL)

	// Current: IDs from DT connections + any orphaned app under the current env-scoped name.
	// Failures when deleting these are hard errors.
	currentClientIDs := azureGatherClientIDs(runner, conns, []string{currentName}, envURL)

	// Legacy: orphaned apps found only under the old fixed name, excluding current IDs.
	// Failures when deleting these are warnings only — the resource may be owned by
	// someone else and the user may lack permission to delete it.
	currentSet := make(map[string]bool, len(currentClientIDs))
	for _, id := range currentClientIDs {
		currentSet[id] = true
	}
	var legacyClientIDs []string
	for _, id := range azureGatherClientIDs(runner, nil, []string{integrationPrefix}, envURL) {
		if !currentSet[id] {
			legacyClientIDs = append(legacyClientIDs, id)
		}
	}

	allClientIDs := append(currentClientIDs, legacyClientIDs...)
	if len(monConfigIDs) == 0 && len(conns) == 0 && len(allClientIDs) == 0 {
		fmt.Println("  No Azure Monitor integration resources found; nothing to uninstall.")
		return nil
	}

	azureUninstallPrintPreview(envURL, monConfigIDs, conns, allClientIDs, currentName, currentName)

	if proceed, err := installer.ShouldProceed(dryRun, "Uninstall"); !proceed {
		return err
	}

	total := uninstallStepCount(monConfigIDs, conns, currentClientIDs, legacyClientIDs)
	if err := runUninstallSteps(0, total, monConfigIDs, conns, currentClientIDs, legacyClientIDs, runner, dtc); err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration removed.")
	fmt.Println()
	return nil
}

func uninstallStepCount(monConfigIDs []string, conns []connRef, currentClientIDs, legacyClientIDs []string) int {
	// 2 steps per app (role assignment delete + app registration delete).
	return len(monConfigIDs) + (len(currentClientIDs)+len(legacyClientIDs))*2 + len(conns)
}

// runUninstallSteps executes deletion steps; offset shifts step numbers when called mid-sequence (e.g. reinstall).
// currentClientIDs failures are hard errors; legacyClientIDs failures are warnings only.
func runUninstallSteps(offset, total int, monConfigIDs []string, conns []connRef, currentClientIDs, legacyClientIDs []string, runner cmdRunner, dtc dtclient) error {
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

	for _, clientID := range currentClientIDs {
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

	for _, clientID := range legacyClientIDs {
		step++
		fmt.Printf("  Step %d/%d: Delete Monitoring Reader role assignment (legacy)...\n", step, total)
		if err := azureDeleteRoleAssignment(runner, clientID); err != nil {
			display.ColorWarning.Printf("  Warning: step %d skipped: legacy resource cleanup failed (continuing): %v\n", step, err)
		} else {
			display.ColorOK.Println("  ✓ Role assignment deleted")
		}

		step++
		fmt.Printf("  Step %d/%d: Delete Azure App Registration (legacy)...\n", step, total)
		if err := azureDeleteApp(runner, clientID); err != nil {
			display.ColorWarning.Printf("  Warning: step %d skipped: legacy resource cleanup failed (continuing): %v\n", step, err)
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
