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
	return updateAzureWithRunner(envURL, platformToken, dryRun, startTime, realRunner, time.Sleep, dtc)
}

func updateAzureWithRunner(
	envURL, platformToken string,
	dryRun bool,
	startTime time.Time,
	runner cmdRunner,
	sleeper func(time.Duration),
	dtc dtclient,
) error {
	var subscriptionID, tenantID string
	if err := display.RunQuiet("Checking Azure account", func(*display.Step) error {
		var err error
		subscriptionID, tenantID, err = azureAccountInfo(runner)
		return err
	}); err != nil {
		return err
	}

	name := integrationNameForEnv(envURL)

	var monConfigIDs []string
	var conns []connRef
	if err := installer.RunConcurrentlyWithSpinner("Looking up existing Azure integration",
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

	azureUpdatePrintPreview(cfg, monConfigIDs, dryRun)

	if proceed, err := installer.ShouldProceed(dryRun, "Update"); !proceed {
		return err
	}

	if err := display.RunStep("Installed Dynatrace extension", func(s *display.Step) error {
		freshlyInstalled, err := dtc.installExtension()
		if err != nil {
			return fmt.Errorf("installing extension %s: %w", extensionName, err)
		}
		if freshlyInstalled {
			logger.Debug("extension freshly installed (async), waiting for it to become active")
			s.Detail("waiting for it to become active")
			if waitErr := waitForExtensionActive(dtc, sleeper); waitErr != nil {
				logger.Debug("extension did not become active in time, proceeding anyway", "error", waitErr)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if err := reconcileMonitoring(cfg, monConfigIDs, dtc); err != nil {
		return err
	}

	installer.MarkInstallDone()
	installer.WatchIngestCloudFromTimeWithEvent(cfg.EnvURL, cfg.PlatformToken, startTime, installer.OnWatchSnapshot, installer.OnWatchComplete)
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
		return display.RunStep("Created monitoring configuration", func(*display.Step) error {
			if err := dtc.createMonitoring(cfg.ConfigurationName, cfg.ConnectionID, cfg.ClientID, cfg.SubscriptionID); err != nil {
				return fmt.Errorf("create monitoring configuration: %w", err)
			}
			return nil
		})
	}

	for _, id := range monConfigIDs {
		logger.Debug("reconciling monitoring config", "configID", id)
		if err := display.RunStep("Updated monitoring configuration", func(*display.Step) error {
			if err := dtc.updateMonitoring(id, cfg.ConfigurationName, cfg.ConnectionID, cfg.ClientID, cfg.SubscriptionID); err != nil {
				return fmt.Errorf("update monitoring configuration %s: %w", id, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func azureUpdatePrintPreview(cfg azureConfig, monConfigIDs []string, showPlan bool) {
	fmt.Println()
	display.Header("dtwiz · Azure update")
	display.PrintAlignedStatusLines(display.ColorDefault,
		"Subscription", cfg.SubscriptionID,
		"Connection", cfg.ConnectionName+" (unchanged)",
		"Updates", "Azure monitoring configuration "+cfg.ConfigurationName,
	)
	display.Detail("environment: %s", cfg.EnvURL)
	display.Detail("tenant ID: %s", cfg.TenantID)
	display.Detail("authentication (connection, Service Principal, federated credential, role) is left unchanged")

	if showPlan || logger.Enabled() {
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
	}
	fmt.Println()
}
