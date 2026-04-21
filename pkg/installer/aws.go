package installer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/fatih/color"
)

// awsTemplateURL is the pinned Dynatrace CloudFormation template.
const awsTemplateURL = "https://dynatrace-data-acquisition.s3.amazonaws.com/aws/deployment/cfn/v1.0.0/da-aws-activation.yaml"

// awsStackConfig holds all values required to render aws.tmpl and drive the
// CloudFormation deployment.
type awsStackConfig struct {
	// StackName is the CloudFormation stack name (unique per account+region).
	StackName string

	// DynatraceURL is the full Dynatrace Platform environment URL.
	DynatraceURL string

	// SettingsToken is a platform token (dt0s16.*) with scopes:
	// settings:objects:write, extensions:configurations:write/read.
	SettingsToken string

	// IngestToken is a platform token (dt0s16.*) with scopes:
	// data-acquisition:logs:ingest, data-acquisition:events:ingest.
	IngestToken string

	// MonitoringConfigID is the UUID of the Dynatrace monitoring configuration.
	MonitoringConfigID string

	// LogsEnabled controls whether the log-forwarder resources are deployed.
	LogsEnabled string // "TRUE" | "FALSE"

	// LogsRegions is a comma-separated list of AWS regions for log ingestion.
	LogsRegions string

	// EventsEnabled controls whether the event-forwarder resources are deployed.
	EventsEnabled string // "TRUE" | "FALSE"

	// EventsRegions is a comma-separated list of AWS regions for event ingestion.
	EventsRegions string

	// EventBridgeBusName is the EventBridge bus to consume events from.
	EventBridgeBusName string

	// EventSources is a comma-separated list of event sources to forward.
	EventSources string

	// UseCMK controls whether a Customer Managed Key is created for encryption.
	UseCMK string // "TRUE" | "FALSE"
}

// isAWSCLIInstalled returns true when the `aws` binary is on PATH.
func isAWSCLIInstalled() bool {
	_, err := exec.LookPath("aws")
	return err == nil
}

// promptLine prints a prompt, reads a single line from stdin and trims
// whitespace.  When the user enters nothing and defaultVal is non-empty the
// default is returned.
func promptLine(prompt, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val == "" {
			return defaultVal, nil
		}
		return val, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return defaultVal, nil
}

// classicAPIURL strips the ".apps." segment from a Dynatrace apps URL so that
// requests target the classic /api/v2 endpoint.
//
//	https://abc.apps.dynatracelabs.com  ->  https://abc.dynatracelabs.com
func classicAPIURL(envURL string) string {
	envURL = strings.TrimRight(envURL, "/")
	envURL = strings.ReplaceAll(envURL, ".apps.dynatracelabs.com", ".dynatracelabs.com")
	envURL = strings.ReplaceAll(envURL, ".apps.dynatrace.com", ".dynatrace.com")
	return envURL
}

// getAWSCallerInfo returns the AWS account ID and the configured default region.
func getAWSCallerInfo() (accountID, region string, err error) {
	// Account ID from STS
	var sterr strings.Builder
	cmd := exec.Command("aws", "sts", "get-caller-identity", "--output", "json")
	cmd.Stderr = &sterr
	out, runErr := cmd.Output()
	if runErr != nil {
		msg := strings.TrimSpace(sterr.String())
		if strings.Contains(msg, "ExpiredToken") {
			return "", "", fmt.Errorf("AWS credentials are expired — run `aws sso login` or refresh your credentials")
		}
		if msg != "" {
			return "", "", fmt.Errorf("aws sts get-caller-identity: %s", msg)
		}
		return "", "", fmt.Errorf("aws sts get-caller-identity: %w", runErr)
	}
	var identity struct {
		Account string `json:"Account"`
	}
	if err := json.Unmarshal(out, &identity); err != nil {
		return "", "", fmt.Errorf("parsing sts identity: %w", err)
	}
	accountID = identity.Account

	// Region: env vars first, then aws configure
	if r := os.Getenv("AWS_DEFAULT_REGION"); r != "" {
		return accountID, r, nil
	}
	if r := os.Getenv("AWS_REGION"); r != "" {
		return accountID, r, nil
	}
	rc, _ := exec.Command("aws", "configure", "get", "region").Output() //nolint:errcheck
	if region = strings.TrimSpace(string(rc)); region != "" {
		return accountID, region, nil
	}
	return accountID, "us-east-1", nil // safe default
}

// dtAuthHeader returns the correct Authorization header value for a Dynatrace
// token: Bearer for platform tokens (dt0s16.*), Api-Token otherwise.
func dtAuthHeader(token string) string {
	if strings.HasPrefix(token, "dt0s16.") {
		return "Bearer " + token
	}
	return "Api-Token " + token
}

// defaultFeatureSets is the standard set of AWS feature sets forwarded to Dynatrace.
var defaultFeatureSets = []string{
	"ApiGateway_essential", "ApplicationELB_essential", "AutoScaling_essential",
	"CloudFront_essential", "DynamoDB_essential", "EBS_essential", "EC2_essential",
	"ECR_essential", "ECS_ContainerInsights_essential", "ECS_essential", "EFS_essential",
	"ELB_essential", "ElastiCache_essential", "Firehose_essential", "Lambda_essential",
	"NATGateway_essential", "NetworkELB_essential", "PrivateLinkEndpoints_essential",
	"PrivateLinkServices_essential", "RDS_essential", "Route53_essential", "S3_essential",
	"SNS_essential", "SQS_essential",
}

// findExistingMonitoringConfig returns the objectId of an existing da-aws
// monitoring configuration for the given AWS account, or "" if none is found.
func findExistingMonitoringConfig(apiURL, token, accountID string) string {
	base := classicAPIURL(apiURL)
	listURL := base + "/api/v2/extensions/com.dynatrace.extension.da-aws/monitoringConfigurations"
	for {
		req, err := http.NewRequest(http.MethodGet, listURL, nil)
		if err != nil {
			return ""
		}
		req.Header.Set("Authorization", dtAuthHeader(token))
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return ""
		}
		var body struct {
			Items []struct {
				ObjectID string `json:"objectId"`
				Value    struct {
					AWS struct {
						Credentials []struct {
							AccountID string `json:"accountId"`
						} `json:"credentials"`
					} `json:"aws"`
				} `json:"value"`
			} `json:"items"`
			NextPageKey string `json:"nextPageKey"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		for _, item := range body.Items {
			for _, cred := range item.Value.AWS.Credentials {
				if cred.AccountID == accountID {
					return item.ObjectID
				}
			}
		}
		if body.NextPageKey == "" {
			break
		}
		listURL = base + "/api/v2/extensions/com.dynatrace.extension.da-aws/monitoringConfigurations?nextPageKey=" + body.NextPageKey
	}
	return ""
}

// createDTMonitoringConfig creates a new da-aws monitoring configuration in
// Dynatrace (mirroring the Python create_monitoring_config logic) and returns
// the objectId (UUID) to use as pMonitoringConfigId in CloudFormation.
func createDTMonitoringConfig(apiURL, token, accountID, region string) (string, error) {
	desc := fmt.Sprintf("dtwiz — account %s / %s", accountID, region)
	payload := []map[string]interface{}{
		{
			"scope": "integration-aws",
			"value": map[string]interface{}{
				"enabled":           false,
				"description":       desc,
				"version":           "1.0.0",
				"featureSets":       defaultFeatureSets,
				"activationContext": "DATA_ACQUISITION",
				"aws": map[string]interface{}{
					"deploymentRegion": region,
					"credentials": []map[string]interface{}{
						{
							"description":  desc,
							"enabled":      false,
							"connectionId": "*",
							"accountId":    accountID,
						},
					},
					"regionFiltering":             []string{region},
					"tagFiltering":                []interface{}{},
					"tagEnrichment":               []interface{}{},
					"smartscapeConfiguration":     map[string]interface{}{"enabled": true},
					"metricsConfiguration":        map[string]interface{}{"enabled": true, "regions": []string{region}},
					"cloudWatchLogsConfiguration": map[string]interface{}{"enabled": false, "regions": []string{region}},
					"namespaces":                  []interface{}{},
					"configurationMode":           "QUICK_START",
					"deploymentMode":              "AUTOMATED",
					"deploymentScope":             "SINGLE_ACCOUNT",
					"manualDeploymentStatus":      "NA",
					"automatedDeploymentStatus":   "NA",
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling monitoring config: %w", err)
	}

	url := classicAPIURL(apiURL) + "/api/v2/extensions/com.dynatrace.extension.da-aws/monitoringConfigurations"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", dtAuthHeader(token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("creating monitoring config: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 207 {
		return "", fmt.Errorf("creating monitoring config (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody))[:min(len(respBody), 400)])
	}

	var results []struct {
		Code     int    `json:"code"`
		ObjectID string `json:"objectId"`
	}
	if err := json.Unmarshal(respBody, &results); err != nil {
		return "", fmt.Errorf("parsing monitoring config response: %w", err)
	}
	for _, item := range results {
		if (item.Code == 200 || item.Code == 201) && item.ObjectID != "" {
			return item.ObjectID, nil
		}
	}
	return "", fmt.Errorf("monitoring config creation returned no objectId: %s", string(respBody))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// downloadAWSTemplate fetches the CloudFormation template from S3 to a
// temporary file and returns its path.  The caller is responsible for
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
					out[i] = p + val[:10] + "***"
				}
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

// InstallAWS deploys the Dynatrace AWS Data Acquisition CloudFormation stack.
//
// Parameters:
//   - envURL:         Dynatrace Platform environment URL
//   - token:          access token (used as default for prompt pre-fill)
//   - platformToken:  dt0s16.* token from --platform-token / DT_PLATFORM_TOKEN (used as default for prompts)
//   - dryRun:         when true, show what would be done without executing
//   - startTime:      RFC3339 timestamp used as the from-clause for WatchIngest (empty = skip watch)
func InstallAWS(envURL, token, platformToken string, dryRun bool, startTime string) error {
	cyan := color.New(color.FgMagenta)
	sep := strings.Repeat("─", 60)

	// Prefer the explicit platform token; fall back to the access token.
	defaultToken := platformToken
	if defaultToken == "" {
		defaultToken = token
	}

	fmt.Println()
	cyan.Println("  Dynatrace AWS CloudFormation Integration")
	fmt.Println()

	// ── Validate parameters ──────────────────────────────────────────────────

	stackName := "dynatrace-data-acquisition"
	dynatraceURL := envURL
	if dynatraceURL == "" {
		return fmt.Errorf("Dynatrace environment URL is required (--environment or DT_ENVIRONMENT)") //nolint:staticcheck // ST1005: keep brand capitalization
	}

	if defaultToken == "" {
		return fmt.Errorf("platform token is required (--platform-token or DT_PLATFORM_TOKEN)")
	}
	settingsToken := defaultToken
	ingestToken := defaultToken

	// ── Auto-create monitoring configuration ──────────────────────────────────

	// The classic /api/v2 endpoint rejects platform tokens (dt0s16.*).
	// Use DT_ACCESS_TOKEN (classic dt0c01.* token) for the monitoring config API.
	apiToken := os.Getenv("DT_ACCESS_TOKEN")
	if apiToken == "" {
		return fmt.Errorf("DT_ACCESS_TOKEN is not set — a classic API token (dt0c01.*) is required for the Dynatrace monitoring configuration API\n  Set it with: export DT_ACCESS_TOKEN=<your-token>")
	}

	fmt.Printf("\n  Fetching AWS account info...\n")
	accountID, region, err := getAWSCallerInfo()
	if err != nil {
		return fmt.Errorf("fetching AWS caller info: %w", err)
	}
	fmt.Printf("  AWS account: %s  region: %s\n", accountID, region)

	monitoringConfigID := findExistingMonitoringConfig(dynatraceURL, apiToken, accountID)
	if monitoringConfigID != "" {
		fmt.Printf("  Monitoring config: found existing %s\n", monitoringConfigID)
	} else {
		fmt.Printf("  Creating Dynatrace monitoring configuration...\n")
		monitoringConfigID, err = createDTMonitoringConfig(dynatraceURL, apiToken, accountID, region)
		if err != nil {
			return fmt.Errorf("creating monitoring configuration: %w", err)
		}
		fmt.Printf("  Monitoring config: created %s\n", monitoringConfigID)
	}
	fmt.Printf("  Template: %s\n", awsTemplateURL)

	cfg := awsStackConfig{
		StackName:          stackName,
		DynatraceURL:       strings.TrimRight(toAppsURL(dynatraceURL), "/"),
		SettingsToken:      settingsToken,
		IngestToken:        ingestToken,
		MonitoringConfigID: monitoringConfigID,
		LogsEnabled:        "TRUE",
		LogsRegions:        region,
		EventsEnabled:      "TRUE",
		EventsRegions:      region,
		EventBridgeBusName: "default",
		EventSources:       "aws.health",
		UseCMK:             "FALSE",
	}

	// ── Render preview ────────────────────────────────────────────────────────

	// Use a placeholder path in the preview; the real temp file is created just
	// before deployment.
	deployArgs := buildDeployArgs(cfg, "/tmp/da-aws-activation.yaml")

	fmt.Println()
	fmt.Printf("  %s\n", sep)
	cyan.Println("  Command to be executed:")
	fmt.Printf("  %s\n", sep)
	fmt.Printf("    aws %s\n", formatDeployCmd(maskTokenArgs(deployArgs)))
	fmt.Printf("  %s\n\n", sep)

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	// ── Preflight ─────────────────────────────────────────────────────────────

	if !isAWSCLIInstalled() {
		return fmt.Errorf("AWS CLI not found — install it from https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	// ── Confirm ───────────────────────────────────────────────────────────────

	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Deployment cancelled.")
		return nil
	}
	fmt.Println()

	// ── Deploy ────────────────────────────────────────────────────────────────

	fmt.Printf("  Downloading CloudFormation template...\n")
	tmplFile, err := downloadAWSTemplate()
	if err != nil {
		return err
	}
	defer os.Remove(tmplFile)

	realArgs := buildDeployArgs(cfg, tmplFile)

	// statusCh carries deployment progress messages into the watch display.
	statusCh := make(chan string, 4)

	// Start CFN deploy in the background immediately — it takes several minutes
	// and produces no meaningful intermediate output.
	var wg sync.WaitGroup
	var deployErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		statusCh <- fmt.Sprintf("CloudFormation stack %q deploying... (this may take a few minutes)", cfg.StackName)
		if err := runCommandSilent("aws", realArgs...); err != nil {
			deployErr = fmt.Errorf("CloudFormation deployment failed: %w", err)
			statusCh <- fmt.Sprintf("CloudFormation deployment failed: %s", err)
			return
		}
		statusCh <- fmt.Sprintf("CloudFormation stack %q deployed successfully.", cfg.StackName)
	}()

	// Run Lambda instrumentation on the main thread — it is quick but produces
	// a lot of output, so let it finish before handing the terminal to watch.
	lambdaErr := InstallAWSLambda(envURL, token, platformToken, false, false)
	if lambdaErr != nil {
		fmt.Printf("\n  Warning: Lambda instrumentation encountered an error: %s\n", lambdaErr)
		fmt.Println("  You can retry with: dtwiz install aws-lambda")
	}

	// Start watch after Lambda output is done — CFN deploy is still running
	// in the background and will send its result into statusCh.
	if startTime != "" && platformToken != "" {
		WatchIngestWithStatus(envURL, platformToken, startTime, statusCh)
	}

	wg.Wait()
	close(statusCh)

	if deployErr != nil {
		return deployErr
	}
	return nil
}

// runCommandSilent runs a command like RunCommand but discards stdout/stderr
// output, preventing interleaving with the watch display.
func runCommandSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
