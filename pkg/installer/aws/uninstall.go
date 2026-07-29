package aws

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// UninstallAWS removes the Dynatrace AWS CloudFormation stack and the
// associated Dynatrace monitoring configuration.
//
// Parameters:
//   - envURL:  Dynatrace Platform environment URL
//   - token:   platform token (dt0s16.*) used for Extensions API calls
//   - dryRun:  when true, show what would be done without executing
func UninstallAWS(envURL, token string, dryRun bool) error {
	dtc, err := newSDKDTClient(envURL, token)
	if err != nil {
		return err
	}
	return uninstallAWSWithClient(dryRun, dtc)
}

// uninstallAWSWithClient is the testable core; dtclient is injected.
func uninstallAWSWithClient(dryRun bool, dtc dtclient) error {
	if !installer.IsAWSCLIInstalled() {
		return fmt.Errorf("AWS CLI not found — install it from https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace AWS Uninstall")
	fmt.Println()

	fmt.Printf("  Fetching AWS account info...\n")
	accountID, region, err := installer.GetAWSCallerInfo()
	if err != nil {
		return fmt.Errorf("fetching AWS caller info: %w", err)
	}
	fmt.Printf("  AWS account: %s  region: %s\n\n", accountID, region)

	stackName := "dynatrace-data-acquisition"

	fmt.Printf("  Looking up Dynatrace AWS monitoring configuration...\n")
	monitoringConfigID, err := dtc.findExistingMonitoringConfig(accountID)
	if err != nil {
		return fmt.Errorf("looking up monitoring configuration: %w", err)
	}
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

	if proceed, err := installer.ShouldProceed(dryRun, "Uninstall"); !proceed {
		return err
	}
	fmt.Println()

	fmt.Printf("  Step 1: Deleting CloudFormation stack %q...\n", stackName)
	if err := installer.RunCommand("aws", "cloudformation", "delete-stack",
		"--stack-name", stackName, "--region", region); err != nil {
		return fmt.Errorf("deleting CloudFormation stack: %w", err)
	}
	fmt.Printf("  Waiting for stack deletion to complete (this may take several minutes)...\n")
	if err := installer.RunCommand("aws", "cloudformation", "wait", "stack-delete-complete",
		"--stack-name", stackName, "--region", region); err != nil {
		return fmt.Errorf("waiting for stack deletion: %w", err)
	}
	fmt.Printf("  Stack %q deleted.\n", stackName)

	if monitoringConfigID != "" {
		fmt.Printf("\n  Step 2: Deleting Dynatrace AWS monitoring configuration %s...\n", monitoringConfigID)
		if err := dtc.deleteMonitoringConfig(monitoringConfigID); err != nil {
			return fmt.Errorf("deleting monitoring config: %w", err)
		}
		fmt.Printf("  Monitoring configuration %s deleted.\n", monitoringConfigID)
	}

	fmt.Println()
	fmt.Println("  Dynatrace AWS uninstalled successfully.")
	return nil
}
