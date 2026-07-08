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
	saEmail := cfg.serviceAccountEmail()
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
	fmt.Printf("  Service account:     %s\n", cfg.serviceAccountEmail())
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

// gcpStepMaxAttempts x gcpStepRetryDelay bounds how long a gcloud step waits for a
// just-created resource (e.g. a service account) to become usable elsewhere. Resource
// creation is eventually consistent: for a few seconds after `iam service-accounts
// create` returns, `add-iam-policy-binding` can still fail with "... does not exist".
// Retrying on a not-found error covers that gap; any other failure is returned immediately.
const (
	gcpStepMaxAttempts = 12
	gcpStepRetryDelay  = 5 * time.Second
)

func gcpRunStep(n, total int, runner cmdRunner, sleeper func(time.Duration), name string, args []string, env []string, desc string) (string, error) {
	fmt.Printf("  Step %d/%d: %s...\n", n, total, desc)
	var out string
	err := installer.Retry(sleeper, installer.RetryConfig{
		MaxAttempts: gcpStepMaxAttempts,
		Delay:       func(int) time.Duration { return installer.Jitter(gcpStepRetryDelay) },
		Retryable:   installer.IsNotFoundErr,
		OnRetry: func(attempt int, _ time.Duration, err error) {
			logger.Debug("gcloud resource not yet propagated, retrying", "step", n, "attempt", attempt, "error", err)
		},
	}, func() error {
		logger.Debug("running step", "step", n, "cmd", name, "args", args)
		var runErr error
		out, runErr = runner(name, args, env)
		if runErr == nil {
			logger.Debug("step output", "step", n, "stdout", out)
		}
		return runErr
	})
	if err != nil {
		logger.Debug("step failed", "step", n, "cmd", name, "args", args, "error", err)
		return out, fmt.Errorf("step %d: %w%s", n, err, gcpPermissionHint(n, err))
	}
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
	saEmail := cfg.serviceAccountEmail()
	fmt.Println()
	display.ColorWarning.Println("  Installation stopped. Resources created so far need to be cleaned up")
	display.ColorWarning.Println("  (run `dtwiz uninstall gcp` to remove them all, or delete manually):")
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

// gcpResumableConnection inspects connections already found under the integration name and
// decides how install should proceed. A complete connection (bound service account) means
// there is nothing to do — the caller must uninstall first. Exactly one incomplete connection
// is a resumable partial install (from a run that failed between step 2 and step 6): its
// object ID is returned so step 2 reuses it instead of creating a duplicate. More than one
// incomplete connection is ambiguous and asks for a clean slate, same as a complete one.
func gcpResumableConnection(conns []connRef) (string, error) {
	complete, incomplete := splitConnectionsByCompleteness(conns)
	if len(complete) > 0 {
		return "", fmt.Errorf("gcp connection '%s' already exists: run `dtwiz uninstall gcp` to remove it first", integrationName)
	}
	switch len(incomplete) {
	case 0:
		return "", nil
	case 1:
		return incomplete[0].objectID, nil
	default:
		return "", fmt.Errorf("found %d incomplete GCP connections named %q: run `dtwiz uninstall gcp` then `dtwiz install gcp` for a clean single integration", len(incomplete), integrationName)
	}
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
	projectID, _, err := gcpAccountInfo(runner)
	if err != nil {
		return err
	}

	existing, err := dtc.findAllConnections(integrationName)
	if err != nil {
		return fmt.Errorf("checking existing connection: %w", err)
	}
	// Complete connection found: reconcile monitoring config in place; don't recreate the SA/bindings.
	if _, err := selectUpdatableConnection(existing); err == nil {
		fmt.Println("\n  Note: prerequisites already exist — running update instead of a fresh install.")
		return updateGCPWithRunner(envURL, platformToken, dryRun, startTime, runner, dtc)
	}
	resumeConnID, err := gcpResumableConnection(existing)
	if err != nil {
		return err
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
		ServiceAccountName: serviceAccountName,
		DTServiceAccount:   dtPrincipal,
		ConnectionID:       resumeConnID,
	}

	gcpPrintPreview(cfg)

	if proceed, err := installer.ShouldProceed(dryRun, "Installation"); !proceed {
		return err
	}

	if err := runInstallSteps(cfg, runner, sleeper, dtc); err != nil {
		return err
	}
	fmt.Println()
	display.ColorMessage.Println("  Google Cloud integration setup complete!")
	fmt.Println()

	installer.WatchIngestCloudFromTime(cfg.EnvURL, cfg.PlatformToken, startTime)
	return nil
}

func runInstallSteps(cfg gcpConfig, runner cmdRunner, sleeper func(time.Duration), dtc dtclient) error {
	const total = 7
	completed := make(map[int]bool)

	_, err := gcpRunStep(1, total, runner, sleeper, "gcloud",
		append([]string{"services", "enable"}, append(append([]string{}, requiredAPIs...), "--project", cfg.ProjectID)...),
		nil, "Enable required Google Cloud APIs")
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return err
	}
	completed[1] = true
	display.ColorOK.Println("  ✓ APIs enabled")

	fmt.Printf("  Step 2/%d: Create Dynatrace GCP connection...\n", total)
	connObjectID := cfg.ConnectionID
	if connObjectID != "" {
		logger.Debug("resuming partial install, reusing existing connection", "objectId", connObjectID)
		display.ColorOK.Printf("  ✓ Connection already exists, resuming: %s\n", connObjectID)
	} else {
		connObjectID, err = dtc.createConnection(cfg.ConnectionName)
		if err != nil {
			gcpPartialFailureHint(cfg, completed)
			return fmt.Errorf("step 2: %w%s", err, gcpConnectionConflictHint(err, cfg.ConnectionName))
		}
		display.ColorOK.Printf("  ✓ Connection created: %s\n", connObjectID)
	}
	completed[2] = true
	cfg.ConnectionID = connObjectID

	fmt.Printf("  Step 3/%d: Create Google Cloud service account...\n", total)
	saEmail, err := gcpCreateServiceAccount(runner, cfg.ServiceAccountName, cfg.ProjectID)
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return fmt.Errorf("step 3: %w%s", err, gcpPermissionHint(3, err))
	}
	completed[3] = true
	cfg.ServiceAccountEmail = saEmail
	display.ColorOK.Printf("  ✓ Service account created: %s\n", saEmail)

	_, err = gcpRunStep(4, total, runner, sleeper, "gcloud",
		[]string{"projects", "add-iam-policy-binding", cfg.ProjectID,
			"--member", serviceAccountMember(saEmail), "--role", viewerRole, "--condition=None"},
		nil, "Grant Viewer role to service account")
	if err != nil {
		gcpPartialFailureHint(cfg, completed)
		return err
	}
	completed[4] = true
	display.ColorOK.Println("  ✓ Viewer role granted")

	_, err = gcpRunStep(5, total, runner, sleeper, "gcloud",
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

// The connection update triggers a live, synchronous GCP impersonation check server-side;
// until the step-5 roles/iam.serviceAccountTokenCreator grant has propagated, that check fails
// with 400 "GCP authentication failed". dtwiz can't probe that readiness locally — the check
// can only be made by Dynatrace's own principal — so the step-6 call itself is the probe.
//
// dtctl and the docs both say to wait "~1 minute" / "a couple of minutes" before this call, so
// updateConnectionInitialDelay skips the near-certain early failures before the first probe,
// and the attempt budget (initial + 30 x 5s ≈ 3 min) covers the rest.
const (
	updateConnectionInitialDelay = 30 * time.Second
	updateConnectionMaxAttempts  = 30
	updateConnectionRetryDelay   = 5 * time.Second
)

// updateConnectionRetryable reports whether err is worth retrying. The live impersonation
// check surfaces a not-yet-propagated binding as a constraint violation ("Constraints
// violated."), which is exactly the condition this retry exists for. "Unknown property"
// means the request shape itself is wrong (e.g. a field name mismatch against the live
// schema) — that never resolves by waiting, so it's excluded even though it also shows up
// as a constraint violation. Anything else (including a permanent Dynatrace-side 403 for
// a token lacking write scope) is not retried: those don't resolve by waiting either, and
// blindly matching "permission" would burn the full ~3 minute budget on an error that can
// never succeed.
func updateConnectionRetryable(err error) bool {
	if strings.Contains(err.Error(), "Unknown property") {
		return false
	}
	return strings.Contains(err.Error(), "Constraints violated")
}

// updateConnectionWithRetry retries DT connection finalization because the impersonation
// IAM binding can take a couple of minutes to propagate before Dynatrace's live impersonation
// check succeeds. See updateConnectionInitialDelay for the sizing rationale.
func updateConnectionWithRetry(dtc dtclient, connObjectID, connName, serviceAccountEmail string, sleeper func(time.Duration)) error {
	lastErr := installer.Retry(sleeper, installer.RetryConfig{
		MaxAttempts: updateConnectionMaxAttempts,
		// The first probe is the readiness check; only once it confirms propagation is
		// still in flight do we pay the longer initial wait (dtctl's "~1 minute"), then
		// fall back to the shorter poll interval. This fails fast on non-propagation
		// errors and wastes no time when the binding is already usable.
		Delay: func(attempt int) time.Duration {
			if attempt == 1 {
				return installer.Jitter(updateConnectionInitialDelay)
			}
			return installer.Jitter(updateConnectionRetryDelay)
		},
		Retryable: updateConnectionRetryable,
		OnRetry: func(attempt int, delay time.Duration, err error) {
			if attempt == 1 {
				fmt.Println("  Waiting for GCP IAM changes to propagate before linking (this can take a minute)...")
			}
			logger.Debug("connection update failed, retrying", "attempt", attempt, "delay", delay, "error", err)
		},
	}, func() error {
		return dtc.updateConnection(connObjectID, connName, serviceAccountEmail)
	})
	if lastErr == nil {
		return nil
	}
	// Mirror dtctl's guidance: this specific failure is usually IAM propagation still in flight.
	if strings.Contains(lastErr.Error(), "GCP authentication failed") {
		return fmt.Errorf("%w\nIAM policy changes can take a couple of minutes to become active; "+
			"the GCP resources and connection were already created — wait a moment and re-run to finish linking", lastErr)
	}
	return lastErr
}
