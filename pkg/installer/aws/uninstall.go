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
	display.Header("dtwiz · AWS uninstall")

	var accountID, region string
	if err := display.RunStep("Fetched AWS account info", func(s *display.Step) error {
		var err error
		accountID, region, err = installer.GetAWSCallerInfo()
		if err != nil {
			return fmt.Errorf("fetching AWS caller info: %w", err)
		}
		s.Detail("account %s · %s", accountID, region)
		return nil
	}); err != nil {
		return err
	}

	stackName := "dynatrace-data-acquisition"

	var monitoringConfigID string
	if err := display.RunStep("Looked up monitoring configuration", func(s *display.Step) error {
		var err error
		monitoringConfigID, err = dtc.findExistingMonitoringConfig(accountID)
		if err != nil {
			return fmt.Errorf("looking up monitoring configuration: %w", err)
		}
		if monitoringConfigID != "" {
			s.Detail("found")
			display.Detail("monitoring config: %s", monitoringConfigID)
		} else {
			s.Detail("none for this account, skipping Dynatrace cleanup")
		}
		return nil
	}); err != nil {
		return err
	}

	fmt.Println()
	rows := []string{"Removes", fmt.Sprintf("CloudFormation stack %s (%s)", stackName, region)}
	if monitoringConfigID != "" {
		rows = append(rows, "Removes", "Dynatrace AWS monitoring configuration")
	}
	display.PrintAlignedStatusLines(display.ColorDefault, rows...)
	fmt.Println()

	if proceed, err := installer.ShouldProceed(dryRun, "Uninstall"); !proceed {
		return err
	}

	if err := display.RunStep("Deleted CloudFormation stack", func(s *display.Step) error {
		s.Detail("waiting for deletion, this may take several minutes")
		if err := installer.RunCommandQuiet("aws", "cloudformation", "delete-stack",
			"--stack-name", stackName, "--region", region); err != nil {
			return fmt.Errorf("deleting CloudFormation stack: %w", err)
		}
		if err := installer.RunCommandQuiet("aws", "cloudformation", "wait", "stack-delete-complete",
			"--stack-name", stackName, "--region", region); err != nil {
			return fmt.Errorf("waiting for stack deletion: %w", err)
		}
		s.Detail("")
		return nil
	}); err != nil {
		return err
	}

	if monitoringConfigID != "" {
		if err := display.RunStep("Deleted monitoring configuration", func(*display.Step) error {
			if err := dtc.deleteMonitoringConfig(monitoringConfigID); err != nil {
				return fmt.Errorf("deleting monitoring config: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
