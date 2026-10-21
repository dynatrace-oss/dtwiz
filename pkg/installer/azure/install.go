package azure

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// azureBuildStepCommands returns human-readable step descriptions for the install preview.
// ConnectionID / ClientID / ObjectID may be placeholders before the install runs.
func azureBuildStepCommands(cfg azureConfig) ([]string, error) {
	connID := cfg.ConnectionID
	if connID == "" {
		connID = "<connection-id>"
	}
	clientID := cfg.ClientID
	if clientID == "" {
		clientID = "<client-id>"
	}
	objectID := cfg.ObjectID
	if objectID == "" {
		objectID = "<object-id>"
	}

	appsURL := installer.AppsURL(cfg.EnvURL)
	audience := strings.TrimPrefix(appsURL, "https://") + "/svc-id/com.dynatrace.da"
	issuer, err := azureIssuerURL(cfg.EnvURL)
	if err != nil {
		return nil, err
	}

	return []string{
		fmt.Sprintf("DT Settings API: create Azure connection '%s' (federatedIdentityCredential)  [env=%s token=***]",
			cfg.ConnectionName, cfg.EnvURL),
		fmt.Sprintf("az ad sp create-for-rbac --name %s --create-password false -o json", cfg.ConnectionName),
		fmt.Sprintf(`az ad app federated-credential create --id %s --parameters '{"name":"%s","issuer":"%s","subject":"dt:connection-id/%s","audiences":["%s"]}'`,
			clientID, fedCredName, issuer, connID, audience),
		fmt.Sprintf("az ad sp show --id %s -o json", clientID),
		fmt.Sprintf(`az role assignment create --assignee-object-id %s --role "Monitoring Reader" --scope %s --assignee-principal-type ServicePrincipal --description "Dynatrace Monitoring"`,
			objectID, cfg.Scope),
		fmt.Sprintf("DT Settings API: update Azure connection '%s' with tenantId=%s applicationId=%s  [env=%s token=***]",
			cfg.ConnectionName, cfg.TenantID, clientID, cfg.EnvURL),
		fmt.Sprintf("DT Extensions API: create Azure monitoring configuration '%s'  [env=%s token=***]",
			cfg.ConfigurationName, cfg.EnvURL),
	}, nil
}

func azurePrintPreview(cfg azureConfig) error {
	fmt.Println()
	display.Header("dtwiz · Azure")
	display.PrintAlignedStatusLines(display.ColorDefault,
		"Subscription", cfg.SubscriptionID,
		"Connection", cfg.ConnectionName,
		"Configuration", cfg.ConfigurationName,
	)
	display.Detail("environment: %s", cfg.EnvURL)
	display.Detail("tenant ID: %s", cfg.TenantID)

	steps, err := azureBuildStepCommands(cfg)
	if err != nil {
		return err
	}
	if logger.Enabled() {
		fmt.Println()
		display.PrintSectionDivider()
		display.ColorMessage.Println("  Commands to be executed:")
		display.PrintSectionDivider()
		for i, s := range steps {
			fmt.Printf("  Step %d: %s\n", i+1, installer.MaskSecret(s, cfg.PlatformToken))
		}
		display.PrintSectionDivider()
	}
	fmt.Println()
	return nil
}

func azureRunStep(n, total int, runner cmdRunner, name string, args []string, env []string, desc string) (string, error) {
	logger.Debug("running step", "step", n, "total", total, "desc", desc, "cmd", name, "args", args)
	display.Detail("%s %s", name, strings.Join(args, " "))
	out, err := runner(name, args, env)
	if err != nil {
		return out, err
	}
	logger.Debug("step output", "step", n, "stdout", out)
	return out, nil
}

// azurePartialFailureHint lists created resources after a mid-install failure.
// `dtwiz uninstall azure` removes them all; the explicit commands are shown for transparency.
func azurePartialFailureHint(cfg azureConfig, completedSteps map[int]bool) {
	if !completedSteps[1] && !completedSteps[2] && !completedSteps[5] {
		return
	}
	fmt.Println()
	display.ColorWarning.Println("  Installation stopped. Resources created so far need to be cleaned up")
	display.ColorWarning.Println("  (run `dtwiz uninstall azure` to remove them all, or delete manually):")
	if completedSteps[1] {
		fmt.Printf("    • DT connection '%s': delete with: dtctl delete azure connection --name %s\n",
			cfg.ConnectionName, cfg.ConnectionName)
	}
	if completedSteps[2] {
		fmt.Printf("    • Azure App Registration '%s' (incl. Service Principal + federated credential): delete with: az ad app delete --id %s\n",
			cfg.ConnectionName, cfg.ClientID)
	}
	if completedSteps[5] {
		fmt.Printf("    • Role assignment: delete with: az role assignment delete --assignee %s --role 'Monitoring Reader'\n", cfg.ClientID)
	}
}

// InstallAzure sets up the Dynatrace Azure Monitor integration using the DT Platform API and az CLI.
func InstallAzure(envURL, platformToken string, dryRun bool, startTime time.Time) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return installAzureWithRunner(envURL, platformToken, dryRun, startTime, realRunner, time.Sleep, dtc)
}

// installAzureWithRunner is the testable core; runner, sleeper, and dtclient are injected.
func installAzureWithRunner(
	envURL, platformToken string,
	dryRun bool,
	startTime time.Time,
	runner cmdRunner,
	sleeper func(time.Duration),
	dtc dtclient,
) error {
	subscriptionID, tenantID, err := azureAccountInfo(runner)
	if err != nil {
		return err
	}

	name := integrationNameForEnv(envURL)

	existing, err := dtc.findAllConnections(integrationPrefix)
	if err != nil {
		return fmt.Errorf("checking existing connection: %w", err)
	}
	if len(existing) > 0 {
		// Complete connection found: reconcile monitoring config in place; don't recreate the SP (Entra "Constraints violated" hazard).
		if _, err := selectUpdatableConnection(existing, name); err == nil {
			fmt.Println("\n  Note: prerequisites already exist — running update instead of a fresh install.")
			return updateAzureWithRunner(envURL, platformToken, dryRun, startTime, runner, sleeper, dtc)
		}
		return fmt.Errorf("azure connection '%s' already exists but is incomplete or duplicated: run `dtwiz uninstall azure` then `dtwiz install azure` for a clean setup", name)
	}
	if err := requireSupportedAzVersion(); err != nil {
		return err
	}

	// Only check RBAC when actually installing — role assignment creation requires it; update does not.
	azureCheckRBAC(runner, "/subscriptions/"+subscriptionID)

	cfg := azureConfig{
		ConnectionName:    name,
		ConfigurationName: name,
		EnvURL:            envURL,
		PlatformToken:     platformToken,
		TenantID:          tenantID,
		SubscriptionID:    subscriptionID,
		Scope:             "/subscriptions/" + subscriptionID,
	}

	if err := azurePrintPreview(cfg); err != nil {
		return err
	}

	if proceed, err := installer.ShouldProceed(dryRun, "Installation"); !proceed {
		return err
	}

	if err := runInstallSteps(cfg, runner, sleeper, dtc); err != nil {
		return err
	}
	fmt.Println()

	installer.MarkInstallDone()
	installer.WatchIngestCloudFromTimeWithEvent(cfg.EnvURL, cfg.PlatformToken, startTime, installer.OnWatchSnapshot, installer.OnWatchComplete)
	return nil
}

func runInstallSteps(cfg azureConfig, runner cmdRunner, sleeper func(time.Duration), dtc dtclient) error {
	const total = 7
	completed := make(map[int]bool)

	// step runs one install step as a spinner line; a failure prints the
	// cleanup hint for the resources created so far.
	step := func(n int, label string, fn func(*display.Step) error) error {
		if err := display.RunStep(label, fn); err != nil {
			azurePartialFailureHint(cfg, completed)
			return fmt.Errorf("step %d: %w", n, err)
		}
		completed[n] = true
		return nil
	}

	var connObjectID string
	if err := step(1, "Created Dynatrace connection", func(s *display.Step) error {
		var err error
		connObjectID, err = dtc.createConnection(cfg.ConnectionName)
		s.Detail("%s", connObjectID)
		return err
	}); err != nil {
		return err
	}
	cfg.ConnectionID = connObjectID

	if err := step(2, "Registered Azure service principal", func(s *display.Step) error {
		out, err := azureRunStep(2, total, runner, "az",
			[]string{"ad", "sp", "create-for-rbac", "--name", cfg.ConnectionName, "--create-password", "false", "-o", "json"},
			nil, "Register Azure Service Principal")
		if err != nil {
			return err
		}
		clientID, err := parseAppID(out)
		if err != nil {
			return fmt.Errorf("parsing SP output: %w", err)
		}
		if clientID == "" {
			return fmt.Errorf("az returned empty appId")
		}
		cfg.ClientID = clientID
		display.Detail("service principal: %s", clientID)
		return nil
	}); err != nil {
		return err
	}

	fedJSON, err := azureBuildFedCredJSON(connObjectID, cfg.EnvURL)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return err
	}
	if err := step(3, "Created federated credential", func(*display.Step) error {
		return createOrReplaceFedCred(runner, cfg.ClientID, fedJSON)
	}); err != nil {
		return err
	}

	if err := step(4, "Retrieved service principal object ID", func(*display.Step) error {
		objectID, err := azureGetSPObjectID(runner, cfg.ClientID, sleeper)
		if err != nil {
			return err
		}
		cfg.ObjectID = objectID
		display.Detail("object ID: %s", objectID)
		return nil
	}); err != nil {
		return err
	}

	if err := step(5, "Assigned Monitoring Reader role", func(*display.Step) error {
		_, err := azureRunStep(5, total, runner, "az",
			[]string{
				"role", "assignment", "create",
				"--assignee-object-id", cfg.ObjectID,
				"--role", "Monitoring Reader",
				"--scope", cfg.Scope,
				"--assignee-principal-type", "ServicePrincipal",
				"--description", "Dynatrace Monitoring",
			},
			nil, "Assign Monitoring Reader role")
		return err
	}); err != nil {
		return err
	}

	if err := step(6, "Updated Dynatrace connection", func(*display.Step) error {
		return updateConnectionWithRetry(dtc, connObjectID, cfg.ConnectionName, cfg.TenantID, cfg.ClientID, sleeper)
	}); err != nil {
		return err
	}

	if err := step(7, "Created monitoring configuration", func(s *display.Step) error {
		freshlyInstalled, err := dtc.installExtension()
		if err != nil {
			return fmt.Errorf("installing extension %s: %w", extensionName, err)
		}
		if freshlyInstalled {
			logger.Debug("extension freshly installed (async), waiting for it to become active")
			s.Detail("waiting for extension to become active")
			if waitErr := waitForExtensionActive(dtc, sleeper); waitErr != nil {
				logger.Debug("extension did not become active in time, proceeding anyway", "error", waitErr)
			}
			s.Detail("")
		}
		if err := dtc.createMonitoring(cfg.ConfigurationName, connObjectID, cfg.ClientID, cfg.SubscriptionID); err != nil {
			return fmt.Errorf("failed to create monitoring configuration: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// parseAppID extracts appId from `az ad sp create-for-rbac` output.
func parseAppID(out string) (string, error) {
	var app struct {
		AppID string `json:"appId"`
	}
	if err := json.Unmarshal([]byte(out), &app); err != nil {
		return "", err
	}
	return app.AppID, nil
}

// createOrReplaceFedCred creates the federated credential, replacing a stale one from a previous partial install if present.
func createOrReplaceFedCred(runner cmdRunner, clientID, fedJSON string) error {
	_, err := runner("az", []string{"ad", "app", "federated-credential", "create",
		"--id", clientID, "--parameters", fedJSON}, nil)
	if err == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return err
	}
	if delErr := azureDeleteFedCred(runner, clientID); delErr != nil {
		return fmt.Errorf("removing stale federated credential: %w", delErr)
	}
	_, err = runner("az", []string{"ad", "app", "federated-credential", "create",
		"--id", clientID, "--parameters", fedJSON}, nil)
	return err
}

// updateConnectionMaxAttempts x updateConnectionRetryDelay bounds how long dtwiz waits for Entra
// to propagate a new federated credential before giving up on DT connection finalization.
const (
	updateConnectionMaxAttempts = 10
	updateConnectionRetryDelay  = 5 * time.Second
)

// updateConnectionWithRetry retries DT connection finalization because Entra can take several seconds
// to propagate a new federated credential; AADSTS70025 and "Constraints violated" signal this.
func updateConnectionWithRetry(dtc dtclient, connObjectID, connName, tenantID, clientID string, sleeper func(time.Duration)) error {
	return installer.Retry(sleeper, installer.RetryConfig{
		MaxAttempts: updateConnectionMaxAttempts,
		Delay:       func(int) time.Duration { return updateConnectionRetryDelay },
		Retryable: func(err error) bool {
			return strings.Contains(err.Error(), "AADSTS70025") || strings.Contains(err.Error(), "Constraints violated")
		},
		OnRetry: func(attempt int, _ time.Duration, err error) {
			logger.Debug("federated credential not yet propagated, retrying", "attempt", attempt, "error", err)
		},
	}, func() error {
		return dtc.updateConnection(connObjectID, connName, tenantID, clientID)
	})
}

func waitForExtensionActive(dtc dtclient, sleeper func(time.Duration)) error {
	return installer.WaitForExtensionActive(dtc.isExtensionActive, sleeper)
}
