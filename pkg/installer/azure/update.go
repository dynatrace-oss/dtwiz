package azure

import (
	"fmt"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// UpdateAzure reconciles only the monitoring configuration; the auth chain (connection, SP, federated credential, role) is never touched.
// Reached from `dtwiz update azure` directly, or from `dtwiz setup` when a connection already exists.
func UpdateAzure(envURL, platformToken string, dryRun bool, startTime time.Time) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	if err := requireSupportedAzVersion(); err != nil {
		return err
	}
	return updateAzureWithRunner(envURL, platformToken, dryRun, startTime, realRunner, dtc)
}

func updateAzureWithRunner(
	envURL, platformToken string,
	dryRun bool,
	startTime time.Time,
	runner cmdRunner,
	dtc dtclient,
) error {
	subscriptionID, tenantID, err := azureAccountInfo(runner)
	if err != nil {
		return err
	}

	name := integrationNameForEnv(envURL)

	var monConfigIDs []string
	var conns []connRef
	if err := installer.RunConcurrently(
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationPrefix); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationPrefix); return },
	); err != nil {
		return err
	}

	conn, err := selectUpdatableConnection(conns, name)
	if err != nil {
		return err
	}

	cfg := azureConfig{
		ConnectionName:    name,
		ConfigurationName: name,
		EnvURL:            envURL,
		PlatformToken:     platformToken,
		TenantID:          tenantID,
		SubscriptionID:    subscriptionID,
		ConnectionID:      conn.objectID,
		ClientID:          conn.clientID,
	}

	azureUpdatePrintPreview(cfg, monConfigIDs)

	if proceed, err := installer.ShouldProceed(dryRun, "Update"); !proceed {
		return err
	}

	if err := dtc.installExtension(); err != nil {
		return fmt.Errorf("installing extension %s: %w", extensionName, err)
	}

	if err := reconcileMonitoring(cfg, monConfigIDs, dtc); err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration updated!")
	fmt.Println()

	installer.WatchIngestCloudFromTime(cfg.EnvURL, cfg.PlatformToken, startTime)
	return nil
}

// selectUpdatableConnection requires exactly one connection with a bound client ID; partial or duplicate connections are rejected.
func selectUpdatableConnection(conns []connRef, name string) (connRef, error) {
	var usable []connRef
	for _, c := range conns {
		if c.clientID != "" {
			usable = append(usable, c)
		}
	}
	switch {
	case len(usable) == 0:
		return connRef{}, fmt.Errorf("no complete Azure connection named %q found: run `dtwiz install azure` to set one up (or `dtwiz uninstall azure` then install to repair a partial one)", name)
	case len(usable) > 1:
		return connRef{}, fmt.Errorf("found %d Azure connections named %q: run `dtwiz uninstall azure` then `dtwiz install azure` for a clean single integration", len(usable), name)
	default:
		return usable[0], nil
	}
}

// reconcileMonitoring updates or creates the monitoring configuration. Each update is a single atomic call; failure leaves the prior config intact.
func reconcileMonitoring(cfg azureConfig, monConfigIDs []string, dtc dtclient) error {
	if len(monConfigIDs) == 0 {
		fmt.Println("  Step 1/1: Create Azure monitoring configuration...")
		if err := dtc.createMonitoring(cfg.ConfigurationName, cfg.ConnectionID, cfg.ClientID, cfg.SubscriptionID); err != nil {
			return fmt.Errorf("create monitoring configuration: %w", err)
		}
		display.ColorOK.Println("  ✓ Monitoring configuration created")
		return nil
	}

	total := len(monConfigIDs)
	for i, id := range monConfigIDs {
		fmt.Printf("  Step %d/%d: Update Azure monitoring configuration...\n", i+1, total)
		logger.Debug("reconciling monitoring config", "configID", id)
		if err := dtc.updateMonitoring(id, cfg.ConfigurationName, cfg.ConnectionID, cfg.ClientID, cfg.SubscriptionID); err != nil {
			return fmt.Errorf("update monitoring configuration %s: %w", id, err)
		}
		display.ColorOK.Println("  ✓ Monitoring configuration updated")
	}
	return nil
}

func azureUpdatePrintPreview(cfg azureConfig, monConfigIDs []string) {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Azure Monitor Integration: Update")
	fmt.Println()
	fmt.Printf("  Environment:        %s\n", cfg.EnvURL)
	fmt.Printf("  Tenant ID:          %s\n", cfg.TenantID)
	fmt.Printf("  Subscription:       %s\n", cfg.SubscriptionID)
	fmt.Printf("  Connection name:    %s (already exists, unchanged)\n", cfg.ConnectionName)
	fmt.Printf("  Configuration name: %s\n", cfg.ConfigurationName)
	fmt.Println()
	display.ColorMessage.Println("  Authentication (connection, Service Principal, federated credential, role) is left unchanged.")
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Steps to be executed:")
	display.PrintSectionDivider()

	if len(monConfigIDs) == 0 {
		fmt.Printf("  Step 1: DT Extensions API: create Azure monitoring configuration '%s'\n", cfg.ConfigurationName)
	} else {
		for i, id := range monConfigIDs {
			fmt.Printf("  Step %d: DT Extensions API: update Azure monitoring configuration '%s' (id: %s)\n", i+1, cfg.ConfigurationName, id)
		}
	}

	display.PrintSectionDivider()
	fmt.Println()
}
