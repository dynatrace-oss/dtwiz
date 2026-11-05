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
		display.StepWarn("Skipped App Registration display-name cleanup", issuerErr.Error())
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
					display.StepWarn(fmt.Sprintf("Skipped App Registration %s named %q", id, name), fmt.Sprintf("could not verify it was created by dtwiz (%v)", verr))
					continue
				}
				if !ok {
					display.StepWarn(fmt.Sprintf("Skipped App Registration %s named %q", id, name), "it lacks the dtwiz federated credential, so it was not created by dtwiz")
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
	if err := installer.RunConcurrentlyWithSpinner("Looking up Azure integration resources",
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

	var allClientIDs = append(currentClientIDs, legacyClientIDs...)
	if len(monConfigIDs) == 0 && len(conns) == 0 && len(allClientIDs) == 0 {
		fmt.Println("  No Azure Monitor integration resources found; nothing to uninstall.")
		return nil
	}

	azureUninstallPrintPreview(envURL, monConfigIDs, conns, allClientIDs, integrationPrefix+"*", integrationPrefix+"*", dryRun)

	if proceed, err := installer.ShouldProceed(dryRun, "Uninstall"); !proceed {
		return err
	}

	total := uninstallStepCount(monConfigIDs, conns, currentClientIDs, legacyClientIDs)
	if err := runUninstallSteps(0, total, monConfigIDs, conns, currentClientIDs, legacyClientIDs, runner, dtc); err != nil {
		return err
	}

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

	// run shows one step. hard failures are collected as errors; soft failures
	// (legacy resources, which may belong to someone else) only warn.
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
			display.StepWarn("Skipped "+label, "legacy resource cleanup failed (continuing): "+err.Error())
			return
		}
		display.StepDone(label, "")
	}

	for _, id := range monConfigIDs {
		run("Deleted monitoring configuration", true, func() error { return dtc.deleteMonitoring(id) })
	}

	for _, clientID := range currentClientIDs {
		run("Deleted Monitoring Reader role assignment", true, func() error { return azureDeleteRoleAssignment(runner, clientID) })
		run("Deleted App Registration", true, func() error { return azureDeleteApp(runner, clientID) })
	}

	for _, clientID := range legacyClientIDs {
		run("Deleted Monitoring Reader role assignment (legacy)", false, func() error { return azureDeleteRoleAssignment(runner, clientID) })
		run("Deleted App Registration (legacy)", false, func() error { return azureDeleteApp(runner, clientID) })
	}

	for _, c := range conns {
		run("Deleted Dynatrace connection", true, func() error { return dtc.deleteConnection(c.objectID) })
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

func azureUninstallPrintPreview(envURL string, monConfigIDs []string, conns []connRef, clientIDs []string, configName, connName string, showPlan bool) {
	fmt.Println()
	display.Header("dtwiz · Azure uninstall")
	rows := []string{}
	for range conns {
		rows = append(rows, "Removes", "Dynatrace Azure connection")
	}
	if len(clientIDs) > 0 {
		rows = append(rows, "Removes", fmt.Sprintf("%d App Registration(s) and role assignment(s)", len(clientIDs)))
	}
	for range monConfigIDs {
		rows = append(rows, "Removes", "Azure monitoring configuration")
	}
	display.PrintAlignedStatusLines(display.ColorDefault, rows...)
	display.Detail("environment: %s", envURL)
	for _, c := range conns {
		display.Detail("connection: %s (id: %s)", connName, c.objectID)
	}
	for _, clientID := range clientIDs {
		display.Detail("app registration: %s", clientID)
	}
	for _, id := range monConfigIDs {
		display.Detail("monitoring config: %s (id: %s)", configName, id)
	}

	if showPlan || logger.Enabled() {
		fmt.Println()
		display.PrintSectionDivider()
		display.ColorMessage.Println("  Steps to be executed:")
		display.PrintSectionDivider()
		for i, s := range azureUninstallBuildSteps(monConfigIDs, conns, clientIDs, configName, connName) {
			fmt.Printf("  Step %d: %s\n", i+1, s)
		}
		display.PrintSectionDivider()
	}
	fmt.Println()
}
