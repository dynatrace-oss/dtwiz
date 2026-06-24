package azure

import (
	"fmt"
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

	// ── Lookup existing resources + preflight (parallel) ──────────────────────
	type monitorRes struct {
		id  string
		err error
	}
	type connRes struct {
		objectID string
		clientID string
		err      error
	}
	type preflightRes struct {
		subscriptionID string
		tenantID       string
		err            error
	}
	monitorCh := make(chan monitorRes, 1)
	connCh := make(chan connRes, 1)
	preflightCh := make(chan preflightRes, 1)
	go func() {
		id, err := dtc.findMonitoringConfig(configurationName)
		monitorCh <- monitorRes{id: id, err: err}
	}()
	go func() {
		objectID, clientID, err := dtc.findConnection(connectionName)
		connCh <- connRes{objectID: objectID, clientID: clientID, err: err}
	}()
	go func() {
		subID, tenID, err := azurePreflightChecks(runner, envURL, platformToken)
		preflightCh <- preflightRes{subscriptionID: subID, tenantID: tenID, err: err}
	}()
	mr := <-monitorCh
	cr := <-connCh
	pr := <-preflightCh
	if mr.err != nil {
		return mr.err
	}
	if cr.err != nil {
		return cr.err
	}
	if pr.err != nil {
		return pr.err
	}
	configID, connObjectID, clientID := mr.id, cr.objectID, cr.clientID
	subscriptionID, tenantID := pr.subscriptionID, pr.tenantID

	// If the DT connection has no stored clientID (previous install failed before step 6),
	// look up the existing SP by display name so the uninstall phase can clean it up.
	if clientID == "" {
		if id, err := azureLookupSPClientIDByName(runner, connectionName); err == nil && id != "" {
			clientID = id
		}
	}

	installCfg := azureConfig{
		ConnectionName:    connectionName,
		ConfigurationName: configurationName,
		EnvURL:            envURL,
		PlatformToken:     platformToken,
		TenantID:          tenantID,
		SubscriptionID:    subscriptionID,
		Scope:             "/subscriptions/" + subscriptionID,
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
	fmt.Printf("  Subscription:       %s\n", subscriptionID)
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
