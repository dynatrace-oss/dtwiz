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
		fmt.Sprintf("az ad sp create-for-rbac --name %s --create-password false -o json",
			cfg.ConnectionName),
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
	display.ColorMessage.Println("  Dynatrace Azure Monitor Integration")
	fmt.Println()
	fmt.Printf("  Environment:        %s\n", cfg.EnvURL)
	fmt.Printf("  Tenant ID:          %s\n", cfg.TenantID)
	fmt.Printf("  Subscription:       %s\n", cfg.SubscriptionID)
	fmt.Printf("  Connection name:    %s\n", cfg.ConnectionName)
	fmt.Printf("  Configuration name: %s\n", cfg.ConfigurationName)
	fmt.Println()
	display.PrintSectionDivider()
	display.ColorMessage.Println("  Commands to be executed:")
	display.PrintSectionDivider()

	steps, err := azureBuildStepCommands(cfg)
	if err != nil {
		return err
	}
	for i, s := range steps {
		masked := installer.MaskSecret(s, cfg.PlatformToken)
		fmt.Printf("  Step %d: %s\n", i+1, masked)
	}

	display.PrintSectionDivider()
	fmt.Println()
	return nil
}

func azureRunStep(n, total int, runner cmdRunner, name string, args []string, env []string, desc string) (string, error) {
	fmt.Printf("  Step %d/%d: %s...\n", n, total, desc)
	logger.Debug("running step", "step", n, "cmd", name, "args", args)
	out, err := runner(name, args, env)
	if err != nil {
		return out, fmt.Errorf("step %d: %w", n, err)
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

	existing, err := dtc.findAllConnections(integrationName)
	if err != nil {
		return fmt.Errorf("checking existing connection: %w", err)
	}
	if len(existing) > 0 {
		// Complete connection found: reconcile monitoring config in place; don't recreate the SP (Entra "Constraints violated" hazard).
		if _, err := selectUpdatableConnection(existing); err == nil {
			fmt.Println("\n  Note: prerequisites already exist — running update instead of a fresh install.")
			return updateAzureWithRunner(envURL, platformToken, dryRun, startTime, runner, dtc)
		}
		return fmt.Errorf("azure connection '%s' already exists but is incomplete or duplicated: run `dtwiz uninstall azure` then `dtwiz install azure` for a clean setup", integrationName)
	}

	// Only check RBAC when actually installing — role assignment creation requires it; update does not.
	azureCheckRBAC(runner, "/subscriptions/"+subscriptionID)

	cfg := azureConfig{
		ConnectionName:    integrationName,
		ConfigurationName: integrationName,
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
	display.ColorMessage.Println("  Azure Monitor integration setup complete!")
	fmt.Println()

	installer.WatchIngestCloudFromTime(cfg.EnvURL, cfg.PlatformToken, startTime)
	return nil
}

func runInstallSteps(cfg azureConfig, runner cmdRunner, sleeper func(time.Duration), dtc dtclient) error {
	const total = 7
	completed := make(map[int]bool)

	fmt.Printf("  Step 1/%d: Create Dynatrace Azure connection...\n", total)
	connObjectID, err := dtc.createConnection(cfg.ConnectionName)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 1: %w", err)
	}
	completed[1] = true
	cfg.ConnectionID = connObjectID
	display.ColorOK.Printf("  ✓ Connection created: %s\n", connObjectID)

	out2, err := azureRunStep(2, total, runner, "az",
		[]string{"ad", "sp", "create-for-rbac", "--name", cfg.ConnectionName, "--create-password", "false", "-o", "json"},
		nil, "Register Azure Service Principal")
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return err
	}
	completed[2] = true
	clientID, tenantID, err := parseCreateSPOutput(out2)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 2: parsing SP output: %w", err)
	}
	if clientID == "" {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 2: az returned empty appId")
	}
	cfg.ClientID = clientID
	if tenantID != "" {
		cfg.TenantID = tenantID
	}
	display.ColorOK.Printf("  ✓ Service Principal created: %s\n", cfg.ClientID)

	fedJSON, err := azureBuildFedCredJSON(connObjectID, cfg.EnvURL)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return err
	}
	fmt.Printf("  Step 3/%d: Create federated credential...\n", total)
	if err := createOrReplaceFedCred(runner, cfg.ClientID, fedJSON); err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 3: %w", err)
	}
	completed[3] = true
	display.ColorOK.Println("  ✓ Federated credential created")

	fmt.Printf("  Step 4/%d: Retrieve SP object ID...\n", total)
	objectID, err := azureGetSPObjectID(runner, cfg.ClientID, sleeper)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 4: %w", err)
	}
	cfg.ObjectID = objectID
	display.ColorOK.Printf("  ✓ SP object ID: %s\n", objectID)

	_, err = azureRunStep(5, total, runner, "az",
		[]string{
			"role", "assignment", "create",
			"--assignee-object-id", cfg.ObjectID,
			"--role", "Monitoring Reader",
			"--scope", cfg.Scope,
			"--assignee-principal-type", "ServicePrincipal",
			"--description", "Dynatrace Monitoring",
		},
		nil, "Assign Monitoring Reader role")
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return err
	}
	completed[5] = true
	display.ColorOK.Println("  ✓ Monitoring Reader role assigned")

	fmt.Printf("  Step 6/%d: Update Dynatrace Azure connection...\n", total)
	if err := updateConnectionWithRetry(dtc, connObjectID, cfg.ConnectionName, cfg.TenantID, cfg.ClientID, sleeper); err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 6: %w", err)
	}
	display.ColorOK.Println("  ✓ Connection updated")

	fmt.Printf("  Step 7/%d: Create Azure monitoring configuration...\n", total)
	if err := dtc.createMonitoring(cfg.ConfigurationName, connObjectID, cfg.ClientID, cfg.SubscriptionID); err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 7: %w", err)
	}
	display.ColorOK.Println("  ✓ Monitoring configuration created")

	return nil
}

// parseCreateSPOutput extracts appId and tenant from `az ad sp create-for-rbac` output.
func parseCreateSPOutput(out string) (appID, tenantID string, err error) {
	var sp struct {
		AppID  string `json:"appId"`
		Tenant string `json:"tenant"`
	}
	if err = json.Unmarshal([]byte(out), &sp); err != nil {
		return "", "", err
	}
	return sp.AppID, sp.Tenant, nil
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
