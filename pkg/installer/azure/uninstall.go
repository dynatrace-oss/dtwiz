package azure

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// ConnectionExists reports whether the dtwiz-managed Azure connection already
// exists in the given Dynatrace environment.
func ConnectionExists(envURL, platformToken string) (bool, error) {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return false, err
	}
	id, _, err := dtc.findConnection("dtwiz-azure")
	return id != "", err
}

// UninstallAzure removes the Dynatrace Azure Monitor integration created by InstallAzure.
// It looks up the connection and monitoring config by their fixed names, then deletes them
// together with the associated Azure Service Principal and its federated credential.
func UninstallAzure(envURL, platformToken string, dryRun bool) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return uninstallAzureWithRunner(envURL, dryRun, realRunner, dtc)
}

func uninstallAzureWithRunner(envURL string, dryRun bool, runner cmdRunner, dtc dtclient) error {
	const (
		connectionName    = "dtwiz-azure"
		configurationName = "dtwiz-azure"
	)

	// ── Lookup (parallel) ─────────────────────────────────────────────────────
	type monitorRes struct {
		id  string
		err error
	}
	type connRes struct {
		objectID string
		clientID string
		err      error
	}
	monitorCh := make(chan monitorRes, 1)
	connCh := make(chan connRes, 1)
	go func() {
		id, err := dtc.findMonitoringConfig(configurationName)
		monitorCh <- monitorRes{id: id, err: err}
	}()
	go func() {
		objectID, clientID, err := dtc.findConnection(connectionName)
		connCh <- connRes{objectID: objectID, clientID: clientID, err: err}
	}()
	mr := <-monitorCh
	cr := <-connCh
	if mr.err != nil {
		return mr.err
	}
	if cr.err != nil {
		return cr.err
	}
	configID, connObjectID, clientID := mr.id, cr.objectID, cr.clientID

	if configID == "" && connObjectID == "" {
		fmt.Println("  No Azure Monitor integration resources found — nothing to uninstall.")
		return nil
	}

	// ── Preview ────────────────────────────────────────────────────────────────
	azureUninstallPrintPreview(envURL, configID, connObjectID, clientID, configurationName, connectionName)

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	// ── Confirm ────────────────────────────────────────────────────────────────
	ok, err := installer.ConfirmProceed("  Apply?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Uninstall cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	totalSteps := uninstallStepCount(configID, connObjectID, clientID)
	if err := runUninstallSteps(0, totalSteps, configID, connObjectID, clientID, runner, dtc); err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration removed.")
	fmt.Println()
	return nil
}

// uninstallStepCount returns the number of deletion steps based on what resources exist.
func uninstallStepCount(configID, connObjectID, clientID string) int {
	n := 0
	if configID != "" {
		n++
	}
	if clientID != "" {
		n += 3
	}
	if connObjectID != "" {
		n++
	}
	return n
}

// runUninstallSteps executes the deletion steps without a preview or confirmation.
// offset shifts the displayed step numbers (0 for a standalone uninstall, N for a reinstall).
// total is the grand total of steps shown to the user.
func runUninstallSteps(offset, total int, configID, connObjectID, clientID string, runner cmdRunner, dtc dtclient) error {
	step := offset

	if configID != "" {
		step++
		fmt.Printf("  Step %d/%d: Delete Azure monitoring configuration...\n", step, total)
		if err := dtc.deleteMonitoring(configID); err != nil {
			return fmt.Errorf("step %d: %w", step, err)
		}
		display.ColorOK.Println("  ✓ Monitoring configuration deleted")
	}

	if clientID != "" {
		step++
		fmt.Printf("  Step %d/%d: Delete federated credential...\n", step, total)
		if err := azureDeleteFedCred(runner, clientID); err != nil {
			return fmt.Errorf("step %d: %w", step, err)
		}
		display.ColorOK.Println("  ✓ Federated credential deleted")

		step++
		if _, err := azureRunStep(step, total, runner, "az",
			[]string{"role", "assignment", "delete",
				"--assignee", clientID,
				"--role", "Monitoring Reader"},
			nil, "Delete Monitoring Reader role assignment"); err != nil {
			return err
		}
		display.ColorOK.Println("  ✓ Role assignment deleted")

		step++
		if _, err := azureRunStep(step, total, runner, "az",
			[]string{"ad", "sp", "delete", "--id", clientID},
			nil, "Delete Azure Service Principal"); err != nil {
			return err
		}
		display.ColorOK.Println("  ✓ Service Principal deleted")
	}

	if connObjectID != "" {
		step++
		fmt.Printf("  Step %d/%d: Delete Dynatrace Azure connection...\n", step, total)
		if err := dtc.deleteConnection(connObjectID); err != nil {
			return fmt.Errorf("step %d: %w", step, err)
		}
		display.ColorOK.Println("  ✓ Connection deleted")
	}

	return nil
}

// azureUninstallBuildSteps returns the human-readable step descriptions for the uninstall phase.
// Used in the combined update preview.
func azureUninstallBuildSteps(configID, connObjectID, clientID, configName, connName string) []string {
	var steps []string
	if configID != "" {
		steps = append(steps, fmt.Sprintf("DT Extensions API: delete monitoring configuration '%s'", configName))
	}
	if clientID != "" {
		steps = append(steps, fmt.Sprintf("az ad app federated-credential delete --id %s --federated-credential-id %s", clientID, fedCredName))
		steps = append(steps, fmt.Sprintf("az role assignment delete --assignee %s --role \"Monitoring Reader\"", clientID))
		steps = append(steps, fmt.Sprintf("az ad sp delete --id %s", clientID))
	}
	if connObjectID != "" {
		steps = append(steps, fmt.Sprintf("DT Settings API: delete Azure connection '%s' (id: %s)", connName, connObjectID))
	}
	return steps
}

func azureUninstallPrintPreview(envURL, configID, connObjectID, clientID, configName, connName string) {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Azure Monitor Integration — Uninstall")
	fmt.Println()
	fmt.Printf("  Environment: %s\n", envURL)
	if connObjectID != "" {
		fmt.Printf("  Connection:  %s (id: %s)\n", connName, connObjectID)
	} else {
		fmt.Printf("  Connection:  %s — not found, skipping\n", connName)
	}
	if clientID != "" {
		fmt.Printf("  Service Principal: %s\n", clientID)
	}
	if configID != "" {
		fmt.Printf("  Monitoring config: %s (id: %s)\n", configName, configID)
	} else {
		fmt.Printf("  Monitoring config: %s — not found, skipping\n", configName)
	}
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Steps to be executed:")
	display.PrintSectionDivider()

	for i, s := range azureUninstallBuildSteps(configID, connObjectID, clientID, configName, connName) {
		fmt.Printf("  Step %d: %s\n", i+1, s)
	}

	display.PrintSectionDivider()
	fmt.Println()
}
