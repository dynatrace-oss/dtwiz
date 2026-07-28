package aws

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// maskTokenArgs returns a copy of args with token values truncated to their
// first 10 characters followed by "***".
func maskTokenArgs(args []string) []string {
	tokenPrefixes := []string{"pDtApiToken=", "pDtIngestToken="}
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		for _, p := range tokenPrefixes {
			if strings.HasPrefix(a, p) {
				val := a[len(p):]
				if len(val) > 10 {
					val = val[:10]
				}
				out[i] = p + val + "***"
				break
			}
		}
	}
	return out
}

// formatDeployCmd formats the argument slice for display, keeping --flag value
// pairs on the same line and placing each --parameter-overrides value on its
// own indented line.
func formatDeployCmd(args []string) string {
	var b strings.Builder
	indent := "\n        "
	paramIndent := "\n            "
	inParams := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--parameter-overrides" {
			b.WriteString(" \\")
			b.WriteString(indent)
			b.WriteString(a)
			inParams = true
			continue
		}
		if inParams {
			b.WriteString(" \\")
			b.WriteString(paramIndent)
			b.WriteString(a)
			continue
		}
		if strings.HasPrefix(a, "--") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			b.WriteString(" \\")
			b.WriteString(indent)
			b.WriteString(a)
			b.WriteString(" ")
			i++
			b.WriteString(args[i])
		} else {
			if b.Len() > 0 {
				b.WriteString(" ")
			}
			b.WriteString(a)
		}
	}
	return b.String()
}

// buildDeployArgs returns the argument slice for `aws cloudformation deploy`.
// templateFile must be a local path (aws deploy accepts --template-file only).
// Each CloudFormation parameter is passed as a separate ParameterKey=Value
// word in --parameter-overrides so the AWS CLI can correctly handle values
// that contain commas (e.g. the regions list).
func buildDeployArgs(cfg awsStackConfig, templateFile string) []string {
	return []string{
		"cloudformation", "deploy",
		"--stack-name", cfg.StackName,
		"--template-file", templateFile,
		"--capabilities", "CAPABILITY_NAMED_IAM",
		"--parameter-overrides",
		fmt.Sprintf("pDynatraceUrl=%s", cfg.DynatraceURL),
		fmt.Sprintf("pDtApiToken=%s", cfg.SettingsToken),
		fmt.Sprintf("pDtIngestToken=%s", cfg.IngestToken),
		fmt.Sprintf("pMonitoringConfigId=%s", cfg.MonitoringConfigID),
		fmt.Sprintf("pDtLogsIngestEnabled=%s", cfg.LogsEnabled),
		fmt.Sprintf("pDtLogsIngestRegions=%s", cfg.LogsRegions),
		fmt.Sprintf("pDtEventsIngestEnabled=%s", cfg.EventsEnabled),
		fmt.Sprintf("pDtEventsIngestRegions=%s", cfg.EventsRegions),
		fmt.Sprintf("pEventBridgeBusName=%s", cfg.EventBridgeBusName),
		fmt.Sprintf("pEventSources=%s", cfg.EventSources),
		fmt.Sprintf("pUseCMK=%s", cfg.UseCMK),
	}
}

// downloadAWSTemplate fetches the CloudFormation template from S3 to a
// temporary file and returns its path. The caller is responsible for
// removing the file when done.
func downloadAWSTemplate() (string, error) {
	resp, err := http.Get(awsTemplateURL) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("downloading CloudFormation template: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("downloading CloudFormation template: HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "da-aws-activation-*.yaml")
	if err != nil {
		return "", fmt.Errorf("creating temp file for CloudFormation template: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("writing CloudFormation template: %w", err)
	}
	return tmp.Name(), nil
}

// InstallAWS deploys the Dynatrace AWS Data Acquisition CloudFormation stack.
//
// Parameters:
//   - envURL:    Dynatrace Platform environment URL
//   - token:     platform token (dt0s16.*) used for settings, ingest, and watch
//   - dryRun:    when true, show what would be done without executing
//   - startTime: RFC3339 timestamp used as the from-clause for WatchIngest (empty = skip watch)
func InstallAWS(envURL, token string, dryRun bool, startTime string) error {
	dtc, err := newSDKDTClient(envURL, token)
	if err != nil {
		return err
	}
	return installAWSWithClient(envURL, token, dryRun, startTime, dtc)
}

// installAWSWithClient is the testable core; dtclient is injected.
func installAWSWithClient(envURL, token string, dryRun bool, startTime string, dtc dtclient) error {
	sep := strings.Repeat("─", 60)

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace AWS CloudFormation Integration")
	fmt.Println()

	if envURL == "" {
		return fmt.Errorf("Dynatrace environment URL is required (--environment or DT_ENVIRONMENT)") //nolint:staticcheck // ST1005: keep brand capitalization
	}
	if token == "" {
		return fmt.Errorf("platform token is required (--platform-token or DT_PLATFORM_TOKEN)")
	}

	if !installer.IsAWSCLIInstalled() {
		return fmt.Errorf("AWS CLI not found — install it from https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	fmt.Printf("\n  Fetching AWS account info...\n")
	accountID, region, err := installer.GetAWSCallerInfo()
	if err != nil {
		return fmt.Errorf("fetching AWS caller info: %w", err)
	}
	fmt.Printf("  AWS account: %s  region: %s\n", accountID, region)
	fmt.Printf("  Template: %s\n", awsTemplateURL)

	// Monitoring config ID is resolved after confirmation; show placeholder in preview.
	cfg := awsStackConfig{
		StackName:          "dynatrace-data-acquisition",
		DynatraceURL:       strings.TrimRight(installer.AppsURL(envURL), "/"),
		SettingsToken:      token,
		IngestToken:        token,
		MonitoringConfigID: "(auto-assigned)",
		LogsEnabled:        "TRUE",
		LogsRegions:        region,
		EventsEnabled:      "TRUE",
		EventsRegions:      region,
		EventBridgeBusName: "default",
		EventSources:       "aws.health",
		UseCMK:             "FALSE",
	}
	deployArgs := buildDeployArgs(cfg, "<temp-file>")

	fmt.Println()
	fmt.Printf("  %s\n", sep)
	display.ColorMessage.Println("  Command to be executed:")
	fmt.Printf("  %s\n", sep)
	fmt.Printf("    aws %s\n", formatDeployCmd(maskTokenArgs(deployArgs)))
	fmt.Printf("  %s\n\n", sep)

	if proceed, err := installer.ShouldProceed(dryRun, "Installation"); !proceed {
		return err
	}
	fmt.Println()

	freshlyInstalled, err := dtc.installExtension()
	if err != nil {
		return fmt.Errorf("installing extension %s: %w", extensionName, err)
	}
	if freshlyInstalled {
		logger.Debug("extension freshly installed (async), waiting for it to become active")
		fmt.Println("  Extension freshly installed — waiting for it to become active...")
		if waitErr := installer.WaitForExtensionActive(dtc.isExtensionActive, time.Sleep); waitErr != nil {
			logger.Debug("extension did not become active in time, proceeding anyway", "error", waitErr)
		} else {
			display.ColorOK.Println("  ✓ Extension is active")
		}
	}

	monitoringConfigID, err := dtc.findExistingMonitoringConfig(accountID)
	if err != nil {
		return fmt.Errorf("looking up monitoring configuration: %w", err)
	}
	if monitoringConfigID != "" {
		fmt.Printf("  Monitoring config: found existing %s\n", monitoringConfigID)
	} else {
		extVersion, vErr := dtc.latestExtensionVersion()
		if vErr != nil {
			return fmt.Errorf("resolving installed extension version: %w", vErr)
		}
		fmt.Printf("  Creating Dynatrace monitoring configuration (extension %s)...\n", extVersion)
		monitoringConfigID, err = dtc.createMonitoringConfig(accountID, region, extVersion)
		if err != nil {
			return fmt.Errorf("creating monitoring configuration: %w", err)
		}
		fmt.Printf("  Monitoring config: created %s\n", monitoringConfigID)
	}

	cfg.MonitoringConfigID = monitoringConfigID

	fmt.Printf("  Downloading CloudFormation template...\n")
	tmplFile, err := downloadAWSTemplate()
	if err != nil {
		return err
	}
	defer os.Remove(tmplFile)

	realArgs := buildDeployArgs(cfg, tmplFile)

	statusCh := make(chan string, 4)

	// Start CFN deploy in the background — it takes several minutes and produces
	// no meaningful intermediate output.
	var wg sync.WaitGroup
	var deployErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		statusCh <- fmt.Sprintf("CloudFormation stack %q deploying... (this may take a few minutes)", cfg.StackName)
		if err := installer.RunCommandQuiet("aws", realArgs...); err != nil {
			deployErr = fmt.Errorf("CloudFormation deployment failed: %w", err)
			statusCh <- fmt.Sprintf("CloudFormation deployment failed: %s", err)
			return
		}
		statusCh <- fmt.Sprintf("CloudFormation stack %q deployed successfully.", cfg.StackName)

		statusCh <- "Enabling Dynatrace AWS monitoring configuration..."
		if err := dtc.enableMonitoringConfig(monitoringConfigID); err != nil {
			deployErr = fmt.Errorf("enabling monitoring configuration: %w", err)
			statusCh <- fmt.Sprintf("Enabling monitoring configuration failed: %s", err)
			return
		}
		statusCh <- "Dynatrace AWS monitoring configuration enabled."
	}()

	// Run Lambda instrumentation on the main thread — it is quick but produces
	// a lot of output, so let it finish before handing the terminal to watch.
	lambdaErr := installer.InstallAWSLambda(envURL, token, false, false)
	if lambdaErr != nil {
		fmt.Printf("\n  Warning: Lambda instrumentation encountered an error: %s\n", lambdaErr)
		fmt.Println("  You can retry with: dtwiz install aws-lambda")
	}

	// Start watch after Lambda output is done — CFN deploy is still running
	// in the background and will send its result into statusCh. Scope the
	// cloud-platform signal queries to this AWS account to filter out noise
	// from other accounts in the same tenant.
	if startTime != "" && token != "" {
		installer.WatchIngestAWS(envURL, token, startTime, statusCh, accountID)
	}

	wg.Wait()
	close(statusCh)

	return deployErr
}
