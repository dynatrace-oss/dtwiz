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

	// ── Lookup ─────────────────────────────────────────────────────────────────
	configID, err := dtc.findMonitoringConfig(configurationName)
	if err != nil {
		return err
	}
	connObjectID, clientID, err := dtc.findConnection(connectionName)
	if err != nil {
		return err
	}

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

	totalSteps := 0
	if configID != "" {
		totalSteps++
	}
	if clientID != "" {
		totalSteps += 3
	}
	if connObjectID != "" {
		totalSteps++
	}
	step := 0

	// ── Delete monitoring configuration ───────────────────────────────────────
	if configID != "" {
		step++
		fmt.Printf("  Step %d/%d: Delete Azure monitoring configuration...\n", step, totalSteps)
		if err := dtc.deleteMonitoring(configID); err != nil {
			return fmt.Errorf("step %d: %w", step, err)
		}
		display.ColorOK.Println("  ✓ Monitoring configuration deleted")
	}

	// ── Delete Azure resources ─────────────────────────────────────────────────
	if clientID != "" {
		step++
		if _, err = azureRunStep(step, totalSteps, runner, "az",
			[]string{"ad", "app", "federated-credential", "delete",
				"--id", clientID,
				"--federated-credential-id", fedCredName},
			nil, "Delete federated credential"); err != nil {
			return err
		}
		display.ColorOK.Println("  ✓ Federated credential deleted")

		step++
		if _, err = azureRunStep(step, totalSteps, runner, "az",
			[]string{"role", "assignment", "delete",
				"--assignee", clientID,
				"--role", "Monitoring Reader"},
			nil, "Delete Monitoring Reader role assignment"); err != nil {
			return err
		}
		display.ColorOK.Println("  ✓ Role assignment deleted")

		step++
		if _, err = azureRunStep(step, totalSteps, runner, "az",
			[]string{"ad", "sp", "delete", "--id", clientID},
			nil, "Delete Azure Service Principal"); err != nil {
			return err
		}
		display.ColorOK.Println("  ✓ Service Principal deleted")
	}

	// ── Delete DT connection ───────────────────────────────────────────────────
	if connObjectID != "" {
		step++
		fmt.Printf("  Step %d/%d: Delete Dynatrace Azure connection...\n", step, totalSteps)
		if err := dtc.deleteConnection(connObjectID); err != nil {
			return fmt.Errorf("step %d: %w", step, err)
		}
		display.ColorOK.Println("  ✓ Connection deleted")
	}

	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration removed.")
	fmt.Println()
	return nil
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

	step := 0
	if configID != "" {
		step++
		fmt.Printf("  Step %d: DT Extensions API: delete monitoring configuration '%s'\n", step, configName)
	}
	if clientID != "" {
		step++
		fmt.Printf("  Step %d: az ad app federated-credential delete --id %s --federated-credential-id %s\n", step, clientID, fedCredName)
		step++
		fmt.Printf("  Step %d: az role assignment delete --assignee %s --role \"Monitoring Reader\"\n", step, clientID)
		step++
		fmt.Printf("  Step %d: az ad sp delete --id %s\n", step, clientID)
	}
	if connObjectID != "" {
		step++
		fmt.Printf("  Step %d: DT Settings API: delete Azure connection '%s' (id: %s)\n", step, connName, connObjectID)
	}

	display.PrintSectionDivider()
	fmt.Println()
}
