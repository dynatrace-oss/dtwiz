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
	return updateGCPWithRunner(envURL, platformToken, dryRun, startTime, realRunner, time.Sleep, dtc)
}

func updateGCPWithRunner(
	envURL, platformToken string,
	dryRun bool,
	startTime time.Time,
	runner cmdRunner,
	sleeper func(time.Duration),
	dtc dtclient,
) error {
	name := integrationNameForEnv(envURL)

	var monConfigIDs []string
	var conns []connRef
	var projectID string
	err := installer.RunConcurrentlyWithSpinner("Looking up existing Google Cloud integration",
		func() (err error) { monConfigIDs, err = dtc.findAllMonitoringConfigs(integrationPrefix); return },
		func() (err error) { conns, err = dtc.findAllConnections(integrationPrefix); return },
		func() (err error) { projectID, _, err = gcpAccountInfo(runner); return },
	)
	if err != nil {
		return err
	}

	conn, err := selectUpdatableConnection(conns, name)
	if err != nil {
		return err
	}

	cfg := gcpConfig{
		ConnectionName:      name,
		ConfigurationName:   name,
		EnvURL:              envURL,
		PlatformToken:       platformToken,
		ProjectID:           projectID,
		ServiceAccountName:  name,
		ServiceAccountEmail: conn.serviceAccountEmail,
		ConnectionID:        conn.objectID,
	}

	gcpUpdatePrintPreview(cfg, monConfigIDs, dryRun)

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

// selectUpdatableConnection requires exactly one connection with a bound service account; partial or duplicate connections are rejected.
func selectUpdatableConnection(conns []connRef, name string) (connRef, error) {
	usable, _ := splitConnectionsByCompleteness(conns)
	switch {
	case len(usable) == 0:
		return connRef{}, fmt.Errorf("no complete GCP connection named %q found: run `dtwiz install gcp` to set one up (or `dtwiz uninstall gcp` then install to repair a partial one)", name)
	case len(usable) > 1:
		return connRef{}, fmt.Errorf("found %d GCP connections named %q: run `dtwiz uninstall gcp` then `dtwiz install gcp` for a clean single integration", len(usable), name)
	default:
		return usable[0], nil
	}
}

// reconcileMonitoring updates or creates the monitoring configuration. Each update is a single atomic call; failure leaves the prior config intact.
func reconcileMonitoring(cfg gcpConfig, monConfigIDs []string, dtc dtclient) error {
	if len(monConfigIDs) == 0 {
		return display.RunStep("Created monitoring configuration", func(*display.Step) error {
			if err := dtc.createMonitoring(cfg.ConfigurationName, cfg.ConnectionID, cfg.ServiceAccountEmail, cfg.ProjectID); err != nil {
				return fmt.Errorf("create monitoring configuration: %w", err)
			}
			return nil
		})
	}

	for _, id := range monConfigIDs {
		logger.Debug("reconciling monitoring config", "configID", id)
		if err := display.RunStep("Updated monitoring configuration", func(*display.Step) error {
			if err := dtc.updateMonitoring(id, cfg.ConfigurationName, cfg.ConnectionID, cfg.ServiceAccountEmail, cfg.ProjectID); err != nil {
				return fmt.Errorf("update monitoring configuration %s: %w", id, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func gcpUpdatePrintPreview(cfg gcpConfig, monConfigIDs []string, showPlan bool) {
	fmt.Println()
	display.Header("dtwiz · Google Cloud update")
	display.PrintAlignedStatusLines(display.ColorDefault,
		"Project", cfg.ProjectID,
		"Connection", cfg.ConnectionName+" (unchanged)",
		"Updates", "GCP monitoring configuration "+cfg.ConfigurationName,
	)
	display.Detail("environment: %s", cfg.EnvURL)
	display.Detail("service account: %s", cfg.ServiceAccountEmail)
	display.Detail("authentication (connection, service account, role grants) is left unchanged")

	if showPlan || logger.Enabled() {
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
	}
	fmt.Println()
}
