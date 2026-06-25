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

// azureBuildStepCommands returns a human-readable one-liner per step.
// ConnectionID / ClientID / ObjectID may be placeholders at preview time.
func azureBuildStepCommands(cfg azureConfig) []string {
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
	issuer := azureIssuerURL(cfg.EnvURL)

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
	}
}

// azurePrintPreview prints the installation summary and command list.
func azurePrintPreview(cfg azureConfig) {
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

	steps := azureBuildStepCommands(cfg)
	for i, s := range steps {
		masked := maskToken(s, cfg.PlatformToken)
		fmt.Printf("  Step %d: %s\n", i+1, masked)
	}

	display.PrintSectionDivider()
	fmt.Println()
}

// azureRunStep executes one numbered step, prints progress, and returns stdout.
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

// azurePartialFailureHint prints cleanup hints based on how far the install got.
func azurePartialFailureHint(cfg azureConfig, completedSteps map[int]bool) {
	if !completedSteps[1] && !completedSteps[2] && !completedSteps[3] && !completedSteps[5] {
		return
	}
	fmt.Println()
	display.ColorWarning.Println("  The following resources were already created and may need to be cleaned up:")
	if completedSteps[1] {
		fmt.Printf("    • DT connection '%s' — delete with: dtctl delete azure connection --name %s\n",
			cfg.ConnectionName, cfg.ConnectionName)
	}
	if completedSteps[2] {
		fmt.Printf("    • Azure SP '%s' — delete with: az ad sp delete --id %s\n", cfg.ConnectionName, cfg.ClientID)
	}
	if completedSteps[3] {
		fmt.Printf("    • Federated credential — delete with: az ad app federated-credential delete --id %s --federated-credential-id %s\n",
			cfg.ClientID, fedCredName)
	}
	if completedSteps[5] {
		fmt.Printf("    • Role assignment — delete with: az role assignment delete --assignee %s --role 'Monitoring Reader'\n", cfg.ObjectID)
	}
}

// InstallAzure sets up the Dynatrace Azure Monitor integration using the DT Platform API and az.
//
// The 7-step workflow:
//  1. DT Settings API: create Azure connection (federatedIdentityCredential)
//  2. az ad sp create-for-rbac
//  3. az ad app federated-credential create
//  4. az ad sp show (with retry for Entra propagation delay)
//  5. az role assignment create
//  6. DT Settings API: update Azure connection (set tenantId + applicationId)
//  7. DT Extensions API: create Azure monitoring configuration
func InstallAzure(envURL, platformToken string, dryRun bool, startTime time.Time) error {
	dtc, err := newSDKDTClient(envURL, platformToken)
	if err != nil {
		return err
	}
	return installAzureWithRunner(envURL, platformToken, dryRun, startTime, realRunner, time.Sleep, dtc)
}

// installAzureWithRunner is the testable core of InstallAzure. It accepts
// injected runner, sleeper, and dtclient to allow unit-testing without real az/API calls.
func installAzureWithRunner(
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
		totalSteps        = 7
	)

	// ── Preflight ──────────────────────────────────────────────────────────────
	subscriptionID, tenantID, err := azurePreflightChecks(runner, envURL, platformToken)
	if err != nil {
		return err
	}

	// ── Existence check ────────────────────────────────────────────────────────
	existingConnID, _, err := dtc.findConnection(connectionName)
	if err != nil {
		return fmt.Errorf("checking existing connection: %w", err)
	}
	if existingConnID != "" {
		return fmt.Errorf("azure connection '%s' already exists — run `dtwiz uninstall azure` to remove it first", connectionName)
	}

	cfg := azureConfig{
		ConnectionName:    connectionName,
		ConfigurationName: configurationName,
		EnvURL:            envURL,
		PlatformToken:     platformToken,
		TenantID:          tenantID,
		SubscriptionID:    subscriptionID,
		Scope:             "/subscriptions/" + subscriptionID,
	}

	// ── Preview ────────────────────────────────────────────────────────────────
	azurePrintPreview(cfg)

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
		fmt.Println("  Installation cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	_, err = runInstallSteps(0, totalSteps, cfg, runner, sleeper, dtc)
	if err != nil {
		return err
	}
	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration setup complete!")
	fmt.Println()
	return nil
}

// runInstallSteps executes the 7-step Azure installation without a preview or confirmation.
// offset shifts the displayed step numbers (0 for a standalone install, N for a reinstall).
// total is the grand total of steps shown to the user.
// Semantic completed keys 1/2/3/5 are used by azurePartialFailureHint regardless of offset.
func runInstallSteps(offset, total int, cfg azureConfig, runner cmdRunner, sleeper func(time.Duration), dtc dtclient) (azureConfig, error) {
	completed := make(map[int]bool)

	// ── Step 1 ────────────────────────────────────────────────────────────────
	fmt.Printf("  Step %d/%d: Create Dynatrace Azure connection...\n", offset+1, total)
	connObjectID, err := dtc.createConnection(cfg.ConnectionName)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return cfg, fmt.Errorf("step %d: %w", offset+1, err)
	}
	completed[1] = true
	cfg.ConnectionID = connObjectID
	display.ColorOK.Printf("  ✓ Connection created: %s\n", connObjectID)

	// ── Step 2 ────────────────────────────────────────────────────────────────
	out2, err := azureRunStep(offset+2, total, runner, "az",
		[]string{"ad", "sp", "create-for-rbac", "--name", cfg.ConnectionName, "--create-password", "false", "-o", "json"},
		nil, "Register Azure Service Principal")
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return cfg, err
	}
	completed[2] = true

	var sp struct {
		AppID  string `json:"appId"`
		Tenant string `json:"tenant"`
	}
	if err = json.Unmarshal([]byte(out2), &sp); err != nil {
		azurePartialFailureHint(cfg, completed)
		return cfg, fmt.Errorf("step %d: parsing SP output: %w", offset+2, err)
	}
	cfg.ClientID = sp.AppID
	if sp.Tenant != "" {
		cfg.TenantID = sp.Tenant
	}
	display.ColorOK.Printf("  ✓ Service Principal created: %s\n", cfg.ClientID)

	// ── Step 3 ────────────────────────────────────────────────────────────────
	fedJSON, err := azureBuildFedCredJSON(connObjectID, cfg.EnvURL)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return cfg, err
	}
	_, err = azureRunStep(offset+3, total, runner, "az",
		[]string{"ad", "app", "federated-credential", "create", "--id", cfg.ClientID, "--parameters", fedJSON},
		nil, "Create federated credential")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			// Stale credential left from a previous partial install — replace it.
			if delErr := azureDeleteFedCred(runner, cfg.ClientID); delErr != nil {
				azurePartialFailureHint(cfg, completed)
				return cfg, fmt.Errorf("step %d: removing stale federated credential: %w", offset+3, delErr)
			}
			_, err = runner("az", []string{"ad", "app", "federated-credential", "create",
				"--id", cfg.ClientID, "--parameters", fedJSON}, nil)
		}
		if err != nil {
			azurePartialFailureHint(cfg, completed)
			return cfg, fmt.Errorf("step %d: %w", offset+3, err)
		}
	}
	completed[3] = true
	display.ColorOK.Println("  ✓ Federated credential created")

	// ── Step 4 ────────────────────────────────────────────────────────────────
	fmt.Printf("  Step %d/%d: Retrieve SP object ID...\n", offset+4, total)
	objectID, err := azureGetSPObjectID(runner, cfg.ClientID, sleeper)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return cfg, fmt.Errorf("step %d: %w", offset+4, err)
	}
	cfg.ObjectID = objectID
	display.ColorOK.Printf("  ✓ SP object ID: %s\n", objectID)

	// ── Step 5 ────────────────────────────────────────────────────────────────
	_, err = azureRunStep(offset+5, total, runner, "az",
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
		return cfg, err
	}
	completed[5] = true
	display.ColorOK.Println("  ✓ Monitoring Reader role assigned")

	// ── Step 6 ────────────────────────────────────────────────────────────────
	// Retry on AADSTS70025 ("no configured federated identity credentials"):
	// Entra takes several seconds to propagate a newly created federated credential.
	fmt.Printf("  Step %d/%d: Update Dynatrace Azure connection...\n", offset+6, total)
	var updateErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			logger.Debug("federated credential not yet propagated, retrying step 6", "attempt", attempt)
			sleeper(5 * time.Second)
		}
		updateErr = dtc.updateConnection(connObjectID, cfg.ConnectionName, cfg.TenantID, cfg.ClientID)
		if updateErr == nil || !strings.Contains(updateErr.Error(), "AADSTS70025") {
			break
		}
	}
	if updateErr != nil {
		azurePartialFailureHint(cfg, completed)
		return cfg, fmt.Errorf("step %d: %w", offset+6, updateErr)
	}
	display.ColorOK.Println("  ✓ Connection updated")

	// ── Step 7 ────────────────────────────────────────────────────────────────
	fmt.Printf("  Step %d/%d: Create Azure monitoring configuration...\n", offset+7, total)
	if err = dtc.createMonitoring(cfg.ConfigurationName, connObjectID, cfg.ClientID, cfg.SubscriptionID); err != nil {
		azurePartialFailureHint(cfg, completed)
		return cfg, fmt.Errorf("step %d: %w", offset+7, err)
	}
	display.ColorOK.Println("  ✓ Monitoring configuration created")

	return cfg, nil
}
