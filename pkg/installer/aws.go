package installer

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/fatih/color"
)

//go:embed aws.tmpl
var awsTemplateText string

// awsTemplateURL is the pinned Dynatrace CloudFormation template.
const awsTemplateURL = "https://dynatrace-data-acquisition.s3.amazonaws.com/aws/deployment/cfn/v1.0.0/da-aws-activation.yaml"

// defaultLogsRegions is the pre-selected set of AWS regions for log ingestion.
// Only generally-available regions are included; newer opt-in-only regions
// (e.g. ap-east-2, ap-southeast-6, ap-southeast-7, mx-central-1) are excluded
// because they cause CREATE_FAILED on accounts that haven't enabled them.
const defaultLogsRegions = "us-east-1,us-east-2,us-west-1,us-west-2," +
	"ca-central-1,ca-west-1," +
	"ap-east-1,ap-northeast-1,ap-northeast-2,ap-northeast-3," +
	"ap-south-1,ap-south-2," +
	"ap-southeast-1,ap-southeast-2,ap-southeast-3,ap-southeast-4,ap-southeast-5," +
	"eu-central-1,eu-central-2,eu-north-1,eu-south-1,eu-south-2," +
	"eu-west-1,eu-west-2,eu-west-3," +
	"me-central-1,me-south-1," +
	"af-south-1,il-central-1,sa-east-1"

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

// promptLineSecret is like promptLine but masks the default value as "[set]"
// to avoid printing sensitive values like tokens to the terminal.
func promptLineSecret(prompt, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("  %s [set]: ", prompt)
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
					"regionFiltering": []string{region},
					"tagFiltering":    []interface{}{},
					"tagEnrichment":   []interface{}{},
					"smartscapeConfiguration":        map[string]interface{}{"enabled": true},
					"metricsConfiguration":           map[string]interface{}{"enabled": true, "regions": []string{region}},
					"cloudWatchLogsConfiguration":    map[string]interface{}{"enabled": false, "regions": []string{region}},
					"namespaces":                     []interface{}{},
					"configurationMode":              "QUICK_START",
					"deploymentMode":                 "AUTOMATED",
					"deploymentScope":                "SINGLE_ACCOUNT",
					"manualDeploymentStatus":         "NA",
					"automatedDeploymentStatus":      "NA",
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

// renderAWSTemplate fills aws.tmpl with the provided config and returns the
// rendered YAML string.
func renderAWSTemplate(cfg awsStackConfig) (string, error) {
	tmpl, err := template.New("aws").Parse(awsTemplateText)
	if err != nil {
		return "", fmt.Errorf("parsing aws template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("rendering aws template: %w", err)
	}
	return buf.String(), nil
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
func InstallAWS(envURL, token, platformToken string, dryRun bool) error {
	cyan := color.New(color.FgMagenta)
	sep := strings.Repeat("─", 60)

	// Prefer the explicit platform token; fall back to the dtctl token.
	defaultToken := platformToken
	if defaultToken == "" {
		defaultToken = token
	}

	fmt.Println()
	cyan.Println("  Dynatrace AWS CloudFormation Integration")
	fmt.Println()
	fmt.Println("  Please provide the following configuration values.")
	fmt.Println("  Press Enter to accept the default shown in [brackets].")
	fmt.Println()

	// ── Collect parameters ────────────────────────────────────────────────────

	stackName, err := promptLine("CloudFormation stack name", "dynatrace-data-acquisition")
	if err != nil {
		return fmt.Errorf("reading stack name: %w", err)
	}
	if stackName == "" {
		return fmt.Errorf("stack name is required")
	}

	dynatraceURL, err := promptLine("Dynatrace environment URL", envURL)
	if err != nil {
		return fmt.Errorf("reading Dynatrace URL: %w", err)
	}
	if dynatraceURL == "" {
		return fmt.Errorf("Dynatrace environment URL is required")
	}

	var settingsToken, ingestToken string
	settingsToken, err = promptLineSecret("API token (scopes: settings:objects:write, extensions:configurations:write/read)", defaultToken)
	if err != nil {
		return fmt.Errorf("reading settings token: %w", err)
	}
	if settingsToken == "" {
		return fmt.Errorf("API token is required")
	}

	ingestToken, err = promptLineSecret("Ingest token (scopes: data-acquisition:logs:ingest, data-acquisition:events:ingest)", defaultToken)
	if err != nil {
		return fmt.Errorf("reading ingest token: %w", err)
	}
	if ingestToken == "" {
		return fmt.Errorf("ingest token is required")
	}

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

	cfg := awsStackConfig{
		StackName:          stackName,
		DynatraceURL:       strings.TrimRight(toAppsURL(dynatraceURL), "/"),
		SettingsToken:      settingsToken,
		IngestToken:        ingestToken,
		MonitoringConfigID: monitoringConfigID,
		LogsEnabled:        "TRUE",
		LogsRegions:        defaultLogsRegions,
		EventsEnabled:      "FALSE",
		EventsRegions:      "",
		EventBridgeBusName: "default",
		EventSources:       "aws.health",
		UseCMK:             "FALSE",
	}

	// ── Render preview ────────────────────────────────────────────────────────

	rendered, err := renderAWSTemplate(cfg)
	if err != nil {
		return err
	}

	// Use a placeholder path in the preview; the real temp file is created just
	// before deployment.
	deployArgs := buildDeployArgs(cfg, "/tmp/da-aws-activation.yaml")

	fmt.Println()
	fmt.Printf("  %s\n", sep)
	fmt.Println("  CloudFormation stack parameters:")
	fmt.Printf("  %s\n", sep)
	for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		fmt.Printf("    %s\n", line)
	}

	fmt.Printf("\n  %s\n", sep)
	cyan.Printf("  Command to be executed:\n")
	fmt.Printf("  %s\n", sep)
	cyan.Printf("    aws %s\n", strings.Join(deployArgs, " \\\n        "))
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

	ok, err := confirmProceed("  Proceed with deployment?")
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

	fmt.Printf("  Deploying CloudFormation stack %q...\n", cfg.StackName)
	fmt.Println("  (This may take a few minutes)")
	fmt.Println()

	if err := RunCommand("aws", realArgs...); err != nil {
		return fmt.Errorf("CloudFormation deployment failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("  Stack %q deployed successfully.\n", cfg.StackName)
	fmt.Printf("  View in the AWS Console: https://console.aws.amazon.com/cloudformation/home#/stacks\n")
	return nil
}
