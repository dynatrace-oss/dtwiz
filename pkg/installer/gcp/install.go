package gcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// gcpBuildStepCommands returns human-readable step descriptions for the install preview.
// ConnectionID / ServiceAccountEmail / DTServiceAccount may be placeholders before the install runs.
func gcpBuildStepCommands(cfg gcpConfig) []string {
	saEmail := cfg.ServiceAccountEmail
	if saEmail == "" {
		saEmail = gcpServiceAccountEmail(cfg.ServiceAccountName, cfg.ProjectID)
	}
	dtPrincipal := cfg.DTServiceAccount
	if dtPrincipal == "" {
		dtPrincipal = "<dynatrace-principal>"
	}

	return []string{
		fmt.Sprintf("gcloud services enable %s --project=%s",
			strings.Join(requiredAPIs, " "), cfg.ProjectID),
		fmt.Sprintf("DT Settings API: create GCP connection '%s'  [env=%s token=***]",
			cfg.ConnectionName, cfg.EnvURL),
		fmt.Sprintf("gcloud iam service-accounts create %s --display-name=%q --project=%s",
			cfg.ServiceAccountName, serviceAccountDisplayName, cfg.ProjectID),
		fmt.Sprintf("gcloud projects add-iam-policy-binding %s --member=%s --role=%s",
			cfg.ProjectID, serviceAccountMember(saEmail), viewerRole) + " --condition=None",
		fmt.Sprintf("gcloud iam service-accounts add-iam-policy-binding %s --member=%s --role=%s",
			saEmail, serviceAccountMember(dtPrincipal), tokenCreatorRole),
		fmt.Sprintf("DT Settings API: update GCP connection '%s' with serviceAccount=%s  [env=%s token=***]",
			cfg.ConnectionName, saEmail, cfg.EnvURL),
		fmt.Sprintf("DT Extensions API: create GCP monitoring configuration '%s'  [env=%s token=***]",
			cfg.ConfigurationName, cfg.EnvURL),
	}
}

func gcpPrintPreview(cfg gcpConfig) {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Google Cloud Integration")
	fmt.Println()
	fmt.Printf("  Environment:         %s\n", cfg.EnvURL)
	fmt.Printf("  Project:             %s\n", cfg.ProjectID)
	fmt.Printf("  Service account:     %s\n", gcpServiceAccountEmail(cfg.ServiceAccountName, cfg.ProjectID))
	fmt.Printf("  Dynatrace principal: %s\n", cfg.DTServiceAccount)
	fmt.Printf("  Connection name:     %s\n", cfg.ConnectionName)
	fmt.Printf("  Configuration name:  %s\n", cfg.ConfigurationName)
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Commands to be executed:")
	display.PrintSectionDivider()

	steps := gcpBuildStepCommands(cfg)
	for i, s := range steps {
		masked := installer.MaskSecret(s, cfg.PlatformToken)
		fmt.Printf("  Step %d: %s\n", i+1, masked)
	}

	display.PrintSectionDivider()
	fmt.Println()
}

func gcpRunStep(n, total int, runner cmdRunner, name string, args []string, env []string, desc string) (string, error) {
	fmt.Printf("  Step %d/%d: %s...\n", n, total, desc)
	logger.Debug("running step", "step", n, "cmd", name, "args", args)
	out, err := runner(name, args, env)
	if err != nil {
		logger.Debug("step failed", "step", n, "cmd", name, "args", args, "error", err)
		return out, fmt.Errorf("step %d: %w%s", n, err, gcpPermissionHint(n, err))
	}
	logger.Debug("step output", "step", n, "stdout", out)
	return out, nil
}

func gcpPermissionHint(step int, err error) string {
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "permission") && !strings.Contains(msg, "forbidden") && !strings.Contains(msg, "auth_permission_denied") {
		return ""
	}

	switch step {
	case 1:
		return "\nHint: the active gcloud account needs permission to enable services on the project, for example roles/serviceusage.serviceUsageAdmin. Later steps also need IAM permissions to create a service account and grant bindings."
	case 3:
		return "\nHint: the active gcloud account needs permission to create service accounts, for example roles/iam.serviceAccountAdmin."
	case 4:
		return "\nHint: the active gcloud account needs permission to update project IAM policy, for example roles/resourcemanager.projectIamAdmin or a project Owner."
	case 5:
		return "\nHint: the active gcloud account needs permission to update the service account IAM policy, for example roles/iam.serviceAccountAdmin on the service account or project."
	default:
		return ""
	}
}

// gcpPartialFailureHint lists created resources after a mid-install failure.
// `dtwiz uninstall gcp` removes them all; the explicit commands are shown for transparency.
func gcpPartialFailureHint(cfg gcpConfig, completedSteps map[int]bool) {
	if !completedSteps[2] && !completedSteps[3] && !completedSteps[4] {
		return
	}
	saEmail := cfg.ServiceAccountEmail
	if saEmail == "" {
		saEmail = gcpServiceAccountEmail(cfg.ServiceAccountName, cfg.ProjectID)
	}
	fmt.Println()
	display.ColorWarning.Println("  The following resources were already created and may need to be cleaned up")
	display.ColorWarning.Println("  (or just re-run `dtwiz uninstall gcp`, which removes them all):")
	if completedSteps[2] {
		fmt.Printf("    • DT connection '%s': delete with: dtctl delete gcp connection --name %s\n",
			cfg.ConnectionName, cfg.ConnectionName)
	}
	if completedSteps[3] {
		fmt.Printf("    • GCP service account '%s': delete with: gcloud iam service-accounts delete %s --project=%s\n",
			saEmail, saEmail, cfg.ProjectID)
	}
	if completedSteps[4] {
		fmt.Printf("    • Project IAM binding: remove with: gcloud projects remove-iam-policy-binding %s --member=%s --role=%s --condition=None\n",
			cfg.ProjectID, serviceAccountMember(saEmail), viewerRole)
	}
}

// InstallGCP sets up the Dynatrace Google Cloud integration using the DT Platform API and gcloud CLI.
func InstallGCP(envURL, platformToken string, dryRun bool, startTime time.Time) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return installGCPWithRunner(envURL, platformToken, dryRun, startTime, realRunner, time.Sleep, dtc)
}

// installGCPWithRunner is the testable core; runner, sleeper, and dtclient are injected.
func installGCPWithRunner(
	envURL, platformToken string,
	dryRun bool,
	startTime time.Time,
	runner cmdRunner,
	sleeper func(time.Duration),
	dtc dtclient,
) error {
	projectID, account, err := gcpAccountInfo(runner)
	if err != nil {
		return err
	}

	existing, err := dtc.findAllConnections(integrationName)
	if err != nil {
		return fmt.Errorf("checking existing connection: %w", err)
	}
	if len(existing) > 0 {
		return fmt.Errorf("gcp connection '%s' already exists: run `dtwiz uninstall gcp` to remove it first", integrationName)
	}

	dtPrincipal, err := dtc.dtServiceAccount()
	if err != nil {
		return fmt.Errorf("resolving Dynatrace principal: %w", err)
	}

	cfg := gcpConfig{
		ConnectionName:     integrationName,
		ConfigurationName:  integrationName,
		EnvURL:             envURL,
		PlatformToken:      platformToken,
		ProjectID:          projectID,
		Account:            account,
		ServiceAccountName: serviceAccountName,
		DTServiceAccount:   dtPrincipal,
	}

	gcpPrintPreview(cfg)

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	ok, err := installer.ConfirmProceed("  Apply?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	if err := runInstallSteps(cfg, runner, sleeper, dtc); err != nil {
		return err
	}
	fmt.Println()
	display.ColorMessage.Println("  Google Cloud integration setup complete!")
	fmt.Println()

	gcpWatchIngest(cfg, startTime)
	return nil
}

// gcpWatchIngest is skipped when startTime is zero (the unit-test path).
func gcpWatchIngest(cfg gcpConfig, startTime time.Time) {
	if startTime.IsZero() {
		return
	}
	fromClause := startTime.UTC().Format("2006-01-02T15:04:05Z")
	installer.WatchIngest(cfg.EnvURL, cfg.PlatformToken, fromClause)
}

func runInstallSteps(cfg gcpConfig, runner cmdRunner, sleeper func(time.Duration), dtc dtclient) error {
	const total = 7
	completed := make(map[int]bool)

	_, err := gcpRunStep(1, total, runner, "gcloud",
		append([]string{"services", "enable"}, append(append([]string{}, requiredAPIs...), "--project", cfg.ProjectID)...),
		nil, "Enable required Google Cloud APIs")
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return err
	}
	completed[1] = true
	display.ColorOK.Println("  ✓ APIs enabled")

	fmt.Printf("  Step 2/%d: Create Dynatrace GCP connection...\n", total)
	connObjectID, err := dtc.createConnection(cfg.ConnectionName)
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return fmt.Errorf("step 2: %w%s", err, gcpConnectionConflictHint(err, cfg.ConnectionName))
	}
	completed[2] = true
	cfg.ConnectionID = connObjectID
	display.ColorOK.Printf("  ✓ Connection created: %s\n", connObjectID)

	fmt.Printf("  Step 3/%d: Create Google Cloud service account...\n", total)
	saEmail, err := gcpCreateServiceAccount(runner, cfg.ServiceAccountName, cfg.ProjectID)
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return fmt.Errorf("step 3: %w", err)
	}
	completed[3] = true
	cfg.ServiceAccountEmail = saEmail
	display.ColorOK.Printf("  ✓ Service account created: %s\n", saEmail)

	_, err = gcpRunStep(4, total, runner, "gcloud",
		[]string{"projects", "add-iam-policy-binding", cfg.ProjectID,
			"--member", serviceAccountMember(saEmail), "--role", viewerRole, "--condition=None"},
		nil, "Grant Viewer role to service account")
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return err
	}
	completed[4] = true
	display.ColorOK.Println("  ✓ Viewer role granted")

	_, err = gcpRunStep(5, total, runner, "gcloud",
		[]string{"iam", "service-accounts", "add-iam-policy-binding", saEmail,
			"--member", serviceAccountMember(cfg.DTServiceAccount), "--role", tokenCreatorRole},
		nil, "Grant impersonation to Dynatrace principal")
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return err
	}
	completed[5] = true
	display.ColorOK.Println("  ✓ Impersonation trust granted")

	fmt.Printf("  Step 6/%d: Update Dynatrace GCP connection...\n", total)
	if err := updateConnectionWithRetry(dtc, connObjectID, cfg.ConnectionName, saEmail, sleeper); err != nil {
		gcpPartialFailureHint(cfg, completed)
		return fmt.Errorf("step 6: %w", err)
	}
	display.ColorOK.Println("  ✓ Connection updated")

	fmt.Printf("  Step 7/%d: Create GCP monitoring configuration...\n", total)
	if err := dtc.createMonitoring(cfg.ConfigurationName, connObjectID, saEmail, cfg.ProjectID); err != nil {
		gcpPartialFailureHint(cfg, completed)
		return fmt.Errorf("step 7: %w", err)
	}
	display.ColorOK.Println("  ✓ Monitoring configuration created")

	return nil
}

// gcpConnectionConflictHint explains a name-uniqueness conflict on connection creation.
// dtwiz already checked findAllConnections (and `dtwiz uninstall gcp` would report the same
// empty result) before reaching this step, so a "name already taken" response here means a
// connection object exists that this token cannot see — most likely it carries read
// permissions restricted to a different app/user context (e.g. created via the Dynatrace UI).
// Retrying will not help: it's not an eventual-consistency lag, the object is durably hidden.
func gcpConnectionConflictHint(err error, connName string) string {
	if !strings.Contains(err.Error(), "another connection defined under this name") {
		return ""
	}
	return fmt.Sprintf("\nHint: dtwiz found no connection named %q via the API, but Dynatrace reports"+
		" the name is already taken. It is likely owned by a different app/user context and hidden from"+
		" this token's view. Open Settings > Cloud and Virtualization > Google Cloud in the Dynatrace UI"+
		" to find and delete the existing connection, then re-run this install.", connName)
}

// updateConnectionWithRetry retries DT connection finalization because the impersonation
// IAM binding can take several seconds to propagate before Dynatrace can validate it.
func updateConnectionWithRetry(dtc dtclient, connObjectID, connName, serviceAccountEmail string, sleeper func(time.Duration)) error {
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		if attempt > 0 {
			logger.Debug("connection update failed, retrying", "attempt", attempt, "error", lastErr)
			sleeper(5 * time.Second)
		}
		lastErr = dtc.updateConnection(connObjectID, connName, serviceAccountEmail)
		if lastErr == nil {
			return nil
		}
		if !strings.Contains(lastErr.Error(), "Constraints violated") && !strings.Contains(strings.ToLower(lastErr.Error()), "permission") {
			return lastErr
		}
	}
	return lastErr
}
