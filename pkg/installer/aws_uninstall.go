package installer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
)

// deleteDTMonitoringConfig deletes a Dynatrace AWS monitoring configuration
// by its objectId.
func deleteDTMonitoringConfig(apiURL, token, objectID string) error {
	url := classicAPIURL(apiURL) + "/api/v2/extensions/com.dynatrace.extension.da-aws/monitoringConfigurations/" + objectID
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", dtAuthHeader(token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("deleting monitoring config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleting monitoring config (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// UninstallAWS removes the Dynatrace AWS CloudFormation stack and the
// associated Dynatrace monitoring configuration.
//
// Parameters:
//   - envURL:  Dynatrace environment URL (used for monitoring config lookup)
//   - token:   access token for Dynatrace API (falls back to DT_ACCESS_TOKEN env var)
//   - dryRun:  when true, show what would be done without executing
func UninstallAWS(envURL, token string, dryRun bool) error {
	cyan := color.New(color.FgMagenta)

	if !isAWSCLIInstalled() {
		return fmt.Errorf("AWS CLI not found — install it from https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	fmt.Println()
	cyan.Println("  Dynatrace AWS Uninstall")
	fmt.Println()

	fmt.Printf("  Fetching AWS account info...\n")
	accountID, region, err := getAWSCallerInfo()
	if err != nil {
		return fmt.Errorf("fetching AWS caller info: %w", err)
	}
	fmt.Printf("  AWS account: %s  region: %s\n\n", accountID, region)

	stackName := "dynatrace-data-acquisition"

	// Resolve the API token for monitoring config lookup.
	// Prefer DT_ACCESS_TOKEN (classic dt0c01.*) per the same convention as install.
	apiToken := os.Getenv("DT_ACCESS_TOKEN")
	if apiToken == "" {
		apiToken = token
	}

	var monitoringConfigID string
	if apiToken != "" && envURL != "" {
		fmt.Printf("  Looking up Dynatrace AWS monitoring configuration...\n")
		monitoringConfigID = findExistingMonitoringConfig(envURL, apiToken, accountID)
		if monitoringConfigID != "" {
			fmt.Printf("  Found monitoring config: %s\n", monitoringConfigID)
		} else {
			fmt.Printf("  No monitoring configuration found for account %s — will skip DT cleanup.\n", accountID)
		}
	} else {
		fmt.Printf("  Dynatrace credentials not configured — skipping monitoring config deletion.\n")
	}

	fmt.Println()
	fmt.Println("  This will perform the following steps:")
	fmt.Printf("    1. Delete CloudFormation stack %q in region %q\n", stackName, region)
	if monitoringConfigID != "" {
		fmt.Printf("    2. Delete Dynatrace AWS monitoring config %s\n", monitoringConfigID)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	ok, err := confirmProceed("  Proceed with uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Uninstall cancelled.")
		return nil
	}
	fmt.Println()

	// Step 1: Delete the CloudFormation stack.
	fmt.Printf("  Step 1: Deleting CloudFormation stack %q...\n", stackName)
	if err := RunCommand("aws", "cloudformation", "delete-stack",
		"--stack-name", stackName, "--region", region); err != nil {
		return fmt.Errorf("deleting CloudFormation stack: %w", err)
	}
	fmt.Printf("  Waiting for stack deletion to complete (this may take several minutes)...\n")
	if err := RunCommand("aws", "cloudformation", "wait", "stack-delete-complete",
		"--stack-name", stackName, "--region", region); err != nil {
		return fmt.Errorf("waiting for stack deletion: %w", err)
	}
	fmt.Printf("  Stack %q deleted.\n", stackName)

	// Step 2: Delete the Dynatrace monitoring configuration.
	if monitoringConfigID != "" {
		fmt.Printf("\n  Step 2: Deleting Dynatrace AWS monitoring configuration %s...\n", monitoringConfigID)
		if err := deleteDTMonitoringConfig(envURL, apiToken, monitoringConfigID); err != nil {
			return fmt.Errorf("deleting monitoring config: %w", err)
		}
		fmt.Printf("  Monitoring configuration %s deleted.\n", monitoringConfigID)
	}

	fmt.Println()
	fmt.Println("  Dynatrace AWS uninstalled successfully.")
	return nil
}
