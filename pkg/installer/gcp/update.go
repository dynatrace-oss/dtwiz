package gcp

import (
	"fmt"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// UpdateGCP reconciles only the monitoring configuration; the auth chain (connection, service account, role grants) is never touched.
// Reached from `dtwiz setup` when a connection already exists; there is no `dtwiz update gcp` subcommand.
func UpdateGCP(envURL, platformToken string, dryRun bool, startTime time.Time) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return updateGCPWithRunner(envURL, platformToken, dryRun, startTime, realRunner, dtc)
}

func updateGCPWithRunner(
	envURL, platformToken string,
	dryRun bool,
	startTime time.Time,
	runner cmdRunner,
	dtc dtclient,
) error {
	var monConfigIDs []string
	var conns []connRef
	var projectID string
	err := installer.RunConcurrently(
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationName); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationName); return },
		func() (err error) { projectID, _, err = gcpAccountInfo(runner); return },
	)
	if err != nil {
		return err
	}

	conn, err := selectUpdatableConnection(conns)
	if err != nil {
		return err
	}

	cfg := gcpConfig{
		ConnectionName:      integrationName,
		ConfigurationName:   integrationName,
		EnvURL:              envURL,
		PlatformToken:       platformToken,
		ProjectID:           projectID,
		ServiceAccountName:  serviceAccountName,
		ServiceAccountEmail: conn.serviceAccountEmail,
		ConnectionID:        conn.objectID,
	}

	gcpUpdatePrintPreview(cfg, monConfigIDs)

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
	display.ColorMessage.Println("  Google Cloud integration updated!")
	fmt.Println()

	installer.WatchIngestCloudFromTime(cfg.EnvURL, cfg.PlatformToken, startTime)
	return nil
}

// selectUpdatableConnection requires exactly one connection with a bound service account; partial or duplicate connections are rejected.
func selectUpdatableConnection(conns []connRef) (connRef, error) {
	usable, _ := splitConnectionsByCompleteness(conns)
	switch {
	case len(usable) == 0:
		return connRef{}, fmt.Errorf("no complete GCP connection named %q found: run `dtwiz install gcp` to set one up (or `dtwiz uninstall gcp` then install to repair a partial one)", integrationName)
	case len(usable) > 1:
		return connRef{}, fmt.Errorf("found %d GCP connections named %q: run `dtwiz uninstall gcp` then `dtwiz install gcp` for a clean single integration", len(usable), integrationName)
	default:
		return usable[0], nil
	}
}

// reconcileMonitoring updates or creates the monitoring configuration. Each update is a single atomic call; failure leaves the prior config intact.
func reconcileMonitoring(cfg gcpConfig, monConfigIDs []string, dtc dtclient) error {
	if len(monConfigIDs) == 0 {
		fmt.Println("  Step 1/1: Create GCP monitoring configuration...")
		if err := dtc.createMonitoring(cfg.ConfigurationName, cfg.ConnectionID, cfg.ServiceAccountEmail, cfg.ProjectID); err != nil {
			return fmt.Errorf("create monitoring configuration: %w", err)
		}
		display.ColorOK.Println("  ✓ Monitoring configuration created")
		return nil
	}

	total := len(monConfigIDs)
	for i, id := range monConfigIDs {
		fmt.Printf("  Step %d/%d: Update GCP monitoring configuration...\n", i+1, total)
		logger.Debug("reconciling monitoring config", "configID", id)
		if err := dtc.updateMonitoring(id, cfg.ConfigurationName, cfg.ConnectionID, cfg.ServiceAccountEmail, cfg.ProjectID); err != nil {
			return fmt.Errorf("update monitoring configuration %s: %w", id, err)
		}
		display.ColorOK.Println("  ✓ Monitoring configuration updated")
	}
	return nil
}

func gcpUpdatePrintPreview(cfg gcpConfig, monConfigIDs []string) {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Google Cloud Integration: Update")
	fmt.Println()
	fmt.Printf("  Environment:        %s\n", cfg.EnvURL)
	fmt.Printf("  Project:            %s\n", cfg.ProjectID)
	fmt.Printf("  Service account:    %s\n", cfg.ServiceAccountEmail)
	fmt.Printf("  Connection name:    %s (unchanged)\n", cfg.ConnectionName)
	fmt.Printf("  Configuration name: %s\n", cfg.ConfigurationName)
	fmt.Println()
	display.ColorMessage.Println("  Authentication (connection, service account, role grants) is left unchanged.")
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Steps to be executed:")
	display.PrintSectionDivider()

	if len(monConfigIDs) == 0 {
		fmt.Printf("  Step 1: DT Extensions API: create GCP monitoring configuration '%s'\n", cfg.ConfigurationName)
	} else {
		for i, id := range monConfigIDs {
			fmt.Printf("  Step %d: DT Extensions API: update GCP monitoring configuration '%s' (id: %s)\n", i+1, cfg.ConfigurationName, id)
		}
	}

	display.PrintSectionDivider()
	fmt.Println()
}
