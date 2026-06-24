package azure

import (
	"fmt"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// UpdateAzure performs a clean reinstall of the Azure Monitor integration:
// it removes the existing connection/SP/config and installs a fresh one.
func UpdateAzure(envURL, platformToken string, dryRun bool, startTime time.Time) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return updateAzureWithRunner(envURL, platformToken, dryRun, startTime, realRunner, time.Sleep, dtc)
}

func updateAzureWithRunner(
	envURL, platformToken string,
	dryRun bool,
	_ time.Time,
	runner cmdRunner,
	sleeper func(time.Duration),
	dtc dtclient,
) error {
	const (
		connectionName    = "dtwiz-azure"
		configurationName = "dtwiz-azure"
	)

	// ── Lookup existing resources ──────────────────────────────────────────────
	configID, err := dtc.findMonitoringConfig(configurationName)
	if err != nil {
		return err
	}
	connObjectID, clientID, err := dtc.findConnection(connectionName)
	if err != nil {
		return err
	}

	// ── Preflight for the install phase ───────────────────────────────────────
	subscriptionID, tenantID, mgmtGroupID, err := azurePreflightChecks(runner, envURL, platformToken)
	if err != nil {
		return err
	}
	mgScope := mgmtGroupID
	if !strings.HasPrefix(mgmtGroupID, "/") {
		mgScope = "/providers/Microsoft.Management/managementGroups/" + mgmtGroupID
	}

	installCfg := azureConfig{
		ConnectionName:    connectionName,
		ConfigurationName: configurationName,
		EnvURL:            envURL,
		PlatformToken:     platformToken,
		TenantID:          tenantID,
		SubscriptionID:    subscriptionID,
		ManagementGroupID: mgScope,
	}

	// ── Build combined step list ───────────────────────────────────────────────
	uninstallSteps := azureUninstallBuildSteps(configID, connObjectID, clientID, configurationName, connectionName)
	installSteps := azureBuildStepCommands(installCfg)
	nUninstall := len(uninstallSteps)
	totalSteps := nUninstall + len(installSteps)

	// ── Preview ────────────────────────────────────────────────────────────────
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Azure Monitor Integration — Update")
	fmt.Println()
	fmt.Printf("  Environment:        %s\n", envURL)
	fmt.Printf("  Tenant ID:          %s\n", tenantID)
	fmt.Printf("  Management group:   %s\n", mgScope)
	fmt.Printf("  Connection name:    %s\n", connectionName)
	fmt.Printf("  Configuration name: %s\n", configurationName)
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Commands to be executed:")
	display.PrintSectionDivider()

	fmt.Println()
	display.ColorMessage.Println("  Phase 1 — Remove existing integration:")
	for i, s := range uninstallSteps {
		fmt.Printf("  Step %d: %s\n", i+1, s)
	}

	fmt.Println()
	display.ColorMessage.Println("  Phase 2 — Install new integration:")
	for i, s := range installSteps {
		masked := maskToken(s, platformToken)
		fmt.Printf("  Step %d: %s\n", nUninstall+i+1, masked)
	}

	display.PrintSectionDivider()
	fmt.Println()

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
		fmt.Println("  Update cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	// ── Phase 1: uninstall ─────────────────────────────────────────────────────
	display.ColorMessage.Println("  Phase 1 — Removing existing integration...")
	fmt.Println()
	if err := runUninstallSteps(0, totalSteps, configID, connObjectID, clientID, runner, dtc); err != nil {
		return fmt.Errorf("uninstall phase: %w", err)
	}

	// ── Phase 2: install ───────────────────────────────────────────────────────
	fmt.Println()
	display.ColorMessage.Println("  Phase 2 — Installing new integration...")
	fmt.Println()
	if _, err := runInstallSteps(nUninstall, totalSteps, installCfg, runner, sleeper, dtc); err != nil {
		return fmt.Errorf("install phase: %w", err)
	}

	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration updated!")
	fmt.Println()
	return nil
}
