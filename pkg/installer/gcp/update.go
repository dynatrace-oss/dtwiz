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
	type monitorRes struct {
		ids []string
		err error
	}
	type connRes struct {
		conns []connRef
		err   error
	}
	type accountRes struct {
		projectID string
		account   string
		err       error
	}
	monitorCh := make(chan monitorRes, 1)
	connCh := make(chan connRes, 1)
	accountCh := make(chan accountRes, 1)
	go func() {
		ids, err := dtc.findAllMonitoringConfigs(integrationName)
		monitorCh <- monitorRes{ids: ids, err: err}
	}()
	go func() {
		conns, err := dtc.findAllConnections(integrationName)
		connCh <- connRes{conns: conns, err: err}
	}()
	go func() {
		projectID, account, err := gcpAccountInfo(runner)
		accountCh <- accountRes{projectID: projectID, account: account, err: err}
	}()
	mr := <-monitorCh
	cr := <-connCh
	ar := <-accountCh
	if mr.err != nil {
		return mr.err
	}
	if cr.err != nil {
		return cr.err
	}
	if ar.err != nil {
		return ar.err
	}
	monConfigIDs, conns := mr.ids, cr.conns
	projectID := ar.projectID

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
		Account:             ar.account,
		ServiceAccountName:  serviceAccountName,
		ServiceAccountEmail: conn.serviceAccountEmail,
		ConnectionID:        conn.objectID,
	}

	gcpUpdatePrintPreview(cfg, monConfigIDs)

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	ok, err := installer.ConfirmProceed("  Apply?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Update cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	if err := reconcileMonitoring(cfg, monConfigIDs, dtc); err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Google Cloud integration updated!")
	fmt.Println()

	gcpWatchIngest(cfg, startTime)
	return nil
}

// selectUpdatableConnection requires exactly one connection with a bound service account; partial or duplicate connections are rejected.
func selectUpdatableConnection(conns []connRef) (connRef, error) {
	var usable []connRef
	for _, c := range conns {
		if c.serviceAccountEmail != "" {
			usable = append(usable, c)
		}
	}
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
