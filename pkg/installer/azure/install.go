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

	return []string{
		fmt.Sprintf("DT Settings API: create Azure connection '%s' (federatedIdentityCredential)  [env=%s token=***]",
			cfg.ConnectionName, cfg.EnvURL),
		fmt.Sprintf("az ad sp create-for-rbac --name %s --create-password false -o json",
			cfg.ConnectionName),
		fmt.Sprintf(`az ad app federated-credential create --id %s --parameters '{"name":"%s","issuer":"https://token.dynatrace.com","subject":"dt:connection-id/%s","audiences":["%s"]}'`,
			clientID, fedCredName, connID, audience),
		fmt.Sprintf("az ad sp show --id %s -o json", clientID),
		fmt.Sprintf(`az role assignment create --assignee-object-id %s --role "Monitoring Reader" --scope %s --assignee-principal-type ServicePrincipal --description "Dynatrace Monitoring"`,
			objectID, cfg.ManagementGroupID),
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
	fmt.Printf("  Management group:   %s\n", cfg.ManagementGroupID)
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
	subscriptionID, tenantID, mgmtGroupID, err := azurePreflightChecks(runner, envURL, platformToken)
	if err != nil {
		return err
	}

	// Normalise: if the returned scope is a bare subscription ID, it's already
	// the full fallback path; otherwise ensure it's the full mgmt-group path.
	mgScope := mgmtGroupID
	if !strings.HasPrefix(mgmtGroupID, "/") {
		mgScope = "/providers/Microsoft.Management/managementGroups/" + mgmtGroupID
	}

	cfg := azureConfig{
		ConnectionName:    connectionName,
		ConfigurationName: configurationName,
		EnvURL:            envURL,
		PlatformToken:     platformToken,
		TenantID:          tenantID,
		SubscriptionID:    subscriptionID,
		ManagementGroupID: mgScope,
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

	completed := make(map[int]bool)

	// ── Step 1: create DT connection ──────────────────────────────────────────
	fmt.Printf("  Step 1/%d: Create Dynatrace Azure connection...\n", totalSteps)
	connObjectID, err := dtc.createConnection(connectionName)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 1: %w", err)
	}
	completed[1] = true
	cfg.ConnectionID = connObjectID
	display.ColorOK.Printf("  ✓ Connection created: %s\n", connObjectID)

	// ── Step 2: register Azure SP ─────────────────────────────────────────────
	out2, err := azureRunStep(2, totalSteps, runner, "az",
		[]string{"ad", "sp", "create-for-rbac", "--name", connectionName, "--create-password", "false", "-o", "json"},
		nil, "Register Azure Service Principal")
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return err
	}
	completed[2] = true

	var sp struct {
		AppID  string `json:"appId"`
		Tenant string `json:"tenant"`
	}
	if err = json.Unmarshal([]byte(out2), &sp); err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 2: parsing SP output: %w", err)
	}
	cfg.ClientID = sp.AppID
	if sp.Tenant != "" {
		cfg.TenantID = sp.Tenant
	}
	display.ColorOK.Printf("  ✓ Service Principal created: %s\n", cfg.ClientID)

	// ── Step 3: create federated credential ──────────────────────────────────
	fedJSON, err := azureBuildFedCredJSON(connObjectID, envURL)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return err
	}
	_, err = azureRunStep(3, totalSteps, runner, "az",
		[]string{"ad", "app", "federated-credential", "create", "--id", cfg.ClientID, "--parameters", fedJSON},
		nil, "Create federated credential")
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return err
	}
	completed[3] = true
	display.ColorOK.Println("  ✓ Federated credential created")

	// ── Step 4: get SP object ID (with retry) ─────────────────────────────────
	fmt.Printf("  Step 4/%d: Retrieve SP object ID...\n", totalSteps)
	objectID, err := azureGetSPObjectID(runner, cfg.ClientID, sleeper)
	if err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 4: %w", err)
	}
	cfg.ObjectID = objectID
	display.ColorOK.Printf("  ✓ SP object ID: %s\n", objectID)

	// ── Step 5: assign Monitoring Reader ─────────────────────────────────────
	_, err = azureRunStep(5, totalSteps, runner, "az",
		[]string{
			"role", "assignment", "create",
			"--assignee-object-id", cfg.ObjectID,
			"--role", "Monitoring Reader",
			"--scope", cfg.ManagementGroupID,
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

	// ── Step 6: update DT connection ─────────────────────────────────────────
	fmt.Printf("  Step 6/%d: Update Dynatrace Azure connection...\n", totalSteps)
	if err = dtc.updateConnection(connObjectID, connectionName, cfg.TenantID, cfg.ClientID); err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 6: %w", err)
	}
	display.ColorOK.Println("  ✓ Connection updated")

	// ── Step 7: create monitoring configuration ───────────────────────────────
	fmt.Printf("  Step 7/%d: Create Azure monitoring configuration...\n", totalSteps)
	if err = dtc.createMonitoring(configurationName, connObjectID); err != nil {
		azurePartialFailureHint(cfg, completed)
		return fmt.Errorf("step 7: %w", err)
	}
	display.ColorOK.Println("  ✓ Monitoring configuration created")

	fmt.Println()
	display.ColorMessage.Println("  Azure Monitor integration setup complete!")
	fmt.Println()
	return nil
}
