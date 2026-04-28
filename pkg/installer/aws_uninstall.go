package installer

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/extensions"
)

// UninstallAWS removes the Dynatrace AWS CloudFormation stack and the
// associated Dynatrace monitoring configuration.
//
// Parameters:
//   - c:     Platform API client (used for Extensions API calls)
//   - envURL: Dynatrace environment URL (used for monitoring config lookup)
//   - dryRun: when true, show what would be done without executing
func UninstallAWS(c *client.PlatformClient, envURL string, dryRun bool) error {
	if !isAWSCLIInstalled() {
		return fmt.Errorf("AWS CLI not found — install it from https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace AWS Uninstall")
	fmt.Println()

	fmt.Printf("  Fetching AWS account info...\n")
	accountID, region, err := getAWSCallerInfo()
	if err != nil {
		return fmt.Errorf("fetching AWS caller info: %w", err)
	}
	fmt.Printf("  AWS account: %s  region: %s\n\n", accountID, region)

	stackName := "dynatrace-data-acquisition"

	fmt.Printf("  Looking up Dynatrace AWS monitoring configuration...\n")
	monitoringConfigID := findExistingMonitoringConfig(c, accountID)
	if monitoringConfigID != "" {
		fmt.Printf("  Found monitoring config: %s\n", monitoringConfigID)
	} else {
		fmt.Printf("  No monitoring configuration found for account %s — will skip DT cleanup.\n", accountID)
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
		if err := extensions.DeleteMonitoringConfig(c, daAWSExtension, monitoringConfigID); err != nil {
			return fmt.Errorf("deleting monitoring config: %w", err)
		}
		fmt.Printf("  Monitoring configuration %s deleted.\n", monitoringConfigID)
	}

	fmt.Println()
	fmt.Println("  Dynatrace AWS uninstalled successfully.")
	return nil
}
