package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/fatih/color"
)

// ── Data types ───────────────────────────────────────────────────────────────

// lambdaFunction represents a discovered AWS Lambda function.
type lambdaFunction struct {
	Name         string
	Runtime      string
	Architecture string // "x86_64" or "arm64"
	Layers       []string
	EnvVars      map[string]string
	PackageType  string // "Zip" or "Image"
}

// dtConnectionInfo holds Dynatrace connection details for Lambda env vars.
type dtConnectionInfo struct {
	TenantUUID string
	ClusterID  string // numeric cluster ID from /api/v1/config/clusterid
	BaseURL    string // classic API URL (no .apps.)
	Token      string // agent connection token (dt0a01.* from /api/v2/tokens/agentConnectionToken)
}

// dtEnvVarKeys lists the env var keys managed by the Dynatrace Lambda layer.
var dtEnvVarKeys = []string{
	"AWS_LAMBDA_EXEC_WRAPPER",
	"DT_TENANT",
	"DT_CLUSTER",
	"DT_CONNECTION_BASE_URL",
	"DT_CONNECTION_AUTH_TOKEN",
	"DT_ENABLE_ESM_LOADERS",
}

// ── Runtime mapping ──────────────────────────────────────────────────────────

// mapRuntimeToTechtype maps an AWS Lambda runtime string (e.g. "nodejs18.x")
// to the Dynatrace layer API techtype parameter. Returns false for unsupported
// runtimes.
func mapRuntimeToTechtype(runtime string) (string, bool) {
	prefixes := []struct {
		prefix   string
		techtype string
	}{
		{"nodejs", "nodejs"},
		{"python", "python"},
		{"java", "java"},
		{"go", "go"},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(runtime, p.prefix) {
			return p.techtype, true
		}
	}
	return "", false
}

// archToDTArch maps the AWS architecture value to the DT API parameter.
func archToDTArch(arch string) string {
	if arch == "arm64" {
		return "arm"
	}
	return "x86"
}

// ── AWS helpers ──────────────────────────────────────────────────────────────

// getLambdaRegion returns the current AWS region from env vars or aws configure.
func getLambdaRegion() (string, error) {
	_, region, err := getAWSCallerInfo()
	if err != nil {
		return "", err
	}
	return region, nil
}

// listLambdaFunctions lists all Lambda functions in the given region, handling
// pagination. Functions with PackageType "Image" are excluded.
func listLambdaFunctions() ([]lambdaFunction, error) {
	var all []lambdaFunction
	var marker string

	for {
		args := []string{"lambda", "list-functions", "--output", "json"}
		if marker != "" {
			args = append(args, "--marker", marker)
		}

		cmd := exec.Command("aws", args...)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				return nil, fmt.Errorf("aws lambda list-functions: %s", msg)
			}
			return nil, fmt.Errorf("aws lambda list-functions: %w", err)
		}

		var resp struct {
			Functions []struct {
				FunctionName  string   `json:"FunctionName"`
				Runtime       string   `json:"Runtime"`
				PackageType   string   `json:"PackageType"`
				Architectures []string `json:"Architectures"`
				Layers        []struct {
					Arn string `json:"Arn"`
				} `json:"Layers"`
				Environment struct {
					Variables map[string]string `json:"Variables"`
				} `json:"Environment"`
			} `json:"Functions"`
			NextMarker string `json:"NextMarker"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("parsing lambda list-functions: %w", err)
		}

		for _, f := range resp.Functions {
			// Skip container image functions — layers cannot be attached.
			if f.PackageType == "Image" {
				continue
			}

			arch := "x86_64"
			if len(f.Architectures) > 0 {
				arch = f.Architectures[0]
			}

			layers := make([]string, 0, len(f.Layers))
			for _, l := range f.Layers {
				layers = append(layers, l.Arn)
			}

			envVars := f.Environment.Variables
			if envVars == nil {
				envVars = make(map[string]string)
			}

			all = append(all, lambdaFunction{
				Name:         f.FunctionName,
				Runtime:      f.Runtime,
				Architecture: arch,
				Layers:       layers,
				EnvVars:      envVars,
				PackageType:  f.PackageType,
			})
		}

		if resp.NextMarker == "" {
			break
		}
		marker = resp.NextMarker
	}

	return all, nil
}

// ── Dynatrace API helpers ────────────────────────────────────────────────────

// getDTConnectionInfo calls the Dynatrace connection info API to obtain tenant
// UUID and cluster ID for Lambda env vars.
func getDTConnectionInfo(envURL, token string) (*dtConnectionInfo, error) {
	apiURL := classicAPIURL(envURL)
	endpoint := apiURL + "/api/v1/deployment/installer/agent/connectioninfo"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building connection info request: %w", err)
	}
	req.Header.Set("Authorization", dtAuthHeader(token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching connection info: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("access token needs InstallerDownload scope for connection info API (HTTP 403)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("connection info API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var info struct {
		TenantUUID string `json:"tenantUUID"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing connection info: %w", err)
	}

	// Fetch the numeric cluster ID from a dedicated endpoint.
	clusterID, err := getClusterID(envURL, token)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster ID: %w", err)
	}

	// Fetch the agent connection token (dt0a01.*) from the v2 API.
	// This is the token the Lambda extension uses for trace ingest auth.
	connToken, err := getAgentConnectionToken(envURL, token)
	if err != nil {
		return nil, fmt.Errorf("resolving agent connection token: %w", err)
	}

	return &dtConnectionInfo{
		TenantUUID: info.TenantUUID,
		ClusterID:  clusterID,
		BaseURL:    classicAPIURL(envURL),
		Token:      connToken,
	}, nil
}

// getClusterID fetches the numeric cluster ID from the Dynatrace Classic API.
// Endpoint: GET /api/v1/config/clusterid → { "clusterId": 997993252 }
func getClusterID(envURL, token string) (string, error) {
	apiURL := classicAPIURL(envURL)
	endpoint := apiURL + "/api/v1/config/clusterid"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("building cluster ID request: %w", err)
	}
	req.Header.Set("Authorization", dtAuthHeader(token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching cluster ID: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 403 {
		return "", fmt.Errorf("access token lacks permission to read cluster ID (HTTP 403)")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("cluster ID API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		ClusterID json.Number `json:"clusterId"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing cluster ID response: %w", err)
	}

	clusterID := result.ClusterID.String()
	if clusterID == "" {
		return "", fmt.Errorf("cluster ID API returned empty clusterId")
	}
	return clusterID, nil
}

// getAgentConnectionToken fetches the agent connection token from the
// Dynatrace v2 API. This is the dt0a01.* token that the Lambda extension uses
// to authenticate when sending traces back to Dynatrace.
// Endpoint: GET /api/v2/tokens/agentConnectionToken
// Required scope: environment-api:agent-connection-tokens:read
func getAgentConnectionToken(envURL, token string) (string, error) {
	apiURL := classicAPIURL(envURL)
	endpoint := apiURL + "/api/v2/agentConnectionToken"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("building agent connection token request: %w", err)
	}
	req.Header.Set("Authorization", dtAuthHeader(token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching agent connection token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 403 {
		return "", fmt.Errorf("access token needs environment-api:agent-connection-tokens:read scope (HTTP 403)")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("agent connection token API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing agent connection token response: %w", err)
	}

	if result.Token == "" {
		return "", fmt.Errorf("agent connection token API returned empty token")
	}
	return result.Token, nil
}

// truncate returns s truncated to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// layerARNCache caches resolved layer ARNs by "techtype-arch" key.
type layerARNCache map[string]string

// getLambdaLayerARN resolves the latest Dynatrace Lambda layer ARN from the DT
// API for the given techtype, architecture, and region. Results are cached in
// the provided cache to avoid redundant API calls.
func getLambdaLayerARN(cache layerARNCache, envURL, token, techtype, arch, region string) (string, error) {
	cacheKey := techtype + "-" + arch
	if arn, ok := cache[cacheKey]; ok {
		return arn, nil
	}

	apiURL := classicAPIURL(envURL)
	endpoint := fmt.Sprintf("%s/api/v1/deployment/lambda/layer?arch=%s&techtype=%s&region=%s&withCollector=excluded",
		apiURL, archToDTArch(arch), techtype, region)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("building layer ARN request: %w", err)
	}
	req.Header.Set("Authorization", dtAuthHeader(token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching layer ARN: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 403 {
		return "", fmt.Errorf("access token needs InstallerDownload scope for Lambda layer resolution (HTTP 403)")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("lambda layer API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		ARNs []struct {
			Arch          string `json:"arch"`
			ARN           string `json:"arn"`
			Region        string `json:"region"`
			TechType      string `json:"techType"`
			WithCollector string `json:"withCollector"`
		} `json:"arns"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing layer ARN response: %w", err)
	}

	if len(result.ARNs) == 0 {
		return "", fmt.Errorf("no Lambda layer ARN found for techtype=%s arch=%s region=%s", techtype, arch, region)
	}

	arn := result.ARNs[0].ARN
	cache[cacheKey] = arn
	return arn, nil
}

// ── Classification ───────────────────────────────────────────────────────────

// isDynatraceInternal returns true if the function is a Dynatrace-managed
// Lambda function that should never be instrumented.
func isDynatraceInternal(fn lambdaFunction) bool {
	return strings.Contains(fn.Name, "DynatraceApiClientFunction")
}

// classifyFunction determines what action to take for a Lambda function.
// Returns "new", "update", or "skip".
func classifyFunction(fn lambdaFunction) string {
	// Skip Dynatrace-internal functions.
	if isDynatraceInternal(fn) {
		return "skip"
	}
	// Check if the runtime is supported first.
	if _, ok := mapRuntimeToTechtype(fn.Runtime); !ok {
		return "skip"
	}
	// Check if already instrumented with Dynatrace.
	if hasDynatraceLayer(fn.Layers) {
		return "update"
	}
	return "new"
}

// hasDynatraceLayer returns true if any layer ARN contains "Dynatrace_OneAgent".
func hasDynatraceLayer(layers []string) bool {
	for _, arn := range layers {
		if strings.Contains(arn, "Dynatrace_OneAgent") {
			return true
		}
	}
	return false
}

// isInstrumented returns true if the function has a Dynatrace Lambda layer.
func isInstrumented(fn lambdaFunction) bool {
	return hasDynatraceLayer(fn.Layers)
}

// ── Preview ──────────────────────────────────────────────────────────────────

// printLambdaPreviewTable renders the preview table of Lambda functions.
func printLambdaPreviewTable(functions []lambdaFunction, actions []string) (actionable, skipped int) {
	cyan := color.New(color.FgMagenta)
	sep := strings.Repeat("─", 70)

	fmt.Println()
	cyan.Println("  Lambda functions to instrument:")
	fmt.Printf("  %s\n", sep)
	fmt.Printf("  %-30s %-14s %-8s %s\n", "Function", "Runtime", "Arch", "Action")
	fmt.Printf("  %s\n", sep)

	for i, fn := range functions {
		action := actions[i]
		actionStr := action
		if action == "skip" {
			actionStr = fmt.Sprintf("skip (%s)", skipReason(fn))
			skipped++
		} else {
			actionable++
		}
		name := fn.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		fmt.Printf("  %-30s %-14s %-8s %s\n", name, fn.Runtime, fn.Architecture, actionStr)
	}

	fmt.Printf("  %s\n", sep)
	fmt.Printf("  %d to instrument, %d skipped\n", actionable, skipped)
	return actionable, skipped
}

// printLambdaUninstallPreview renders the preview for uninstallation.
func printLambdaUninstallPreview(functions []lambdaFunction) {
	cyan := color.New(color.FgMagenta)
	sep := strings.Repeat("─", 55)

	fmt.Println()
	cyan.Println("  Lambda functions to uninstrument:")
	fmt.Printf("  %s\n", sep)
	fmt.Printf("  %-30s %-14s %s\n", "Function", "Runtime", "Arch")
	fmt.Printf("  %s\n", sep)

	for _, fn := range functions {
		name := fn.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		fmt.Printf("  %-30s %-14s %s\n", name, fn.Runtime, fn.Architecture)
	}

	fmt.Printf("  %s\n", sep)
	fmt.Printf("  %d functions to uninstrument\n", len(functions))
}

// skipReason returns a human-readable reason for skipping a function.
func skipReason(fn lambdaFunction) string {
	if isDynatraceInternal(fn) {
		return "Dynatrace internal"
	}
	if _, ok := mapRuntimeToTechtype(fn.Runtime); !ok {
		return "unsupported runtime"
	}
	return "unknown"
}

// ── Instrumentation ──────────────────────────────────────────────────────────

// mergeDTEnvVars adds the Dynatrace env vars to the function's existing env
// vars, preserving all non-DT values.
func mergeDTEnvVars(existing map[string]string, conn *dtConnectionInfo, runtime string) map[string]string {
	merged := make(map[string]string, len(existing)+5)
	for k, v := range existing {
		merged[k] = v
	}
	merged["AWS_LAMBDA_EXEC_WRAPPER"] = "/opt/dynatrace"
	merged["DT_TENANT"] = conn.TenantUUID
	merged["DT_CLUSTER"] = conn.ClusterID
	merged["DT_CONNECTION_BASE_URL"] = conn.BaseURL
	merged["DT_CONNECTION_AUTH_TOKEN"] = conn.Token
	if strings.HasPrefix(runtime, "nodejs") {
		merged["DT_ENABLE_ESM_LOADERS"] = "true"
	}
	return merged
}

// removeDTEnvVars removes all Dynatrace-managed env vars from the map.
func removeDTEnvVars(existing map[string]string) map[string]string {
	cleaned := make(map[string]string, len(existing))
	for k, v := range existing {
		isDT := false
		for _, dtKey := range dtEnvVarKeys {
			if k == dtKey {
				isDT = true
				break
			}
		}
		if !isDT {
			cleaned[k] = v
		}
	}
	return cleaned
}

// updateLayers replaces any existing Dynatrace layer with the new ARN, or
// appends it if not present.
func updateLayers(existing []string, newARN string) []string {
	var updated []string
	replaced := false
	for _, arn := range existing {
		if strings.Contains(arn, "Dynatrace_OneAgent") {
			updated = append(updated, newARN)
			replaced = true
		} else {
			updated = append(updated, arn)
		}
	}
	if !replaced {
		updated = append(updated, newARN)
	}
	return updated
}

// removeDynatraceLayers removes all Dynatrace layers from the list.
func removeDynatraceLayers(layers []string) []string {
	var cleaned []string
	for _, arn := range layers {
		if !strings.Contains(arn, "Dynatrace_OneAgent") {
			cleaned = append(cleaned, arn)
		}
	}
	return cleaned
}

// instrumentFunction attaches the Dynatrace Lambda layer and sets env vars on
// a single function. It reads the current config first to merge env vars.
func instrumentFunction(fn lambdaFunction, layerARN string, conn *dtConnectionInfo) error {
	// Read current configuration to get fresh env vars and layers.
	currentEnv, currentLayers, err := getFunctionConfig(fn.Name)
	if err != nil {
		return fmt.Errorf("reading config for %s: %w", fn.Name, err)
	}

	// Merge env vars and layers.
	mergedEnv := mergeDTEnvVars(currentEnv, conn, fn.Runtime)
	mergedLayers := updateLayers(currentLayers, layerARN)

	return updateFunctionConfig(fn.Name, mergedEnv, mergedLayers)
}

// uninstrumentFunction removes the Dynatrace layer and DT_* env vars from a
// single function.
func uninstrumentFunction(fn lambdaFunction) error {
	currentEnv, currentLayers, err := getFunctionConfig(fn.Name)
	if err != nil {
		return fmt.Errorf("reading config for %s: %w", fn.Name, err)
	}

	cleanedEnv := removeDTEnvVars(currentEnv)
	cleanedLayers := removeDynatraceLayers(currentLayers)

	return updateFunctionConfig(fn.Name, cleanedEnv, cleanedLayers)
}

// getFunctionConfig retrieves the current environment variables and layers for
// a Lambda function via the AWS CLI.
func getFunctionConfig(functionName string) (envVars map[string]string, layers []string, err error) {
	cmd := exec.Command("aws", "lambda", "get-function-configuration",
		"--function-name", functionName, "--output", "json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, nil, fmt.Errorf("%s", msg)
		}
		return nil, nil, runErr
	}

	var config struct {
		Environment struct {
			Variables map[string]string `json:"Variables"`
		} `json:"Environment"`
		Layers []struct {
			Arn string `json:"Arn"`
		} `json:"Layers"`
	}
	if err := json.Unmarshal(out, &config); err != nil {
		return nil, nil, fmt.Errorf("parsing function config: %w", err)
	}

	envVars = config.Environment.Variables
	if envVars == nil {
		envVars = make(map[string]string)
	}

	layers = make([]string, 0, len(config.Layers))
	for _, l := range config.Layers {
		layers = append(layers, l.Arn)
	}

	return envVars, layers, nil
}

// updateFunctionConfig updates a Lambda function's environment variables and
// layers via the AWS CLI.
func updateFunctionConfig(functionName string, envVars map[string]string, layers []string) error {
	// Build the --environment argument as JSON.
	envJSON, err := json.Marshal(map[string]interface{}{
		"Variables": envVars,
	})
	if err != nil {
		return fmt.Errorf("marshalling environment: %w", err)
	}

	args := []string{
		"lambda", "update-function-configuration",
		"--function-name", functionName,
		"--environment", string(envJSON),
	}

	// Add layers (empty list to clear all layers if needed).
	if len(layers) > 0 {
		args = append(args, "--layers")
		args = append(args, layers...)
	} else {
		args = append(args, "--layers")
	}

	return RunCommandQuiet("aws", args...)
}

// ── Main entry points ────────────────────────────────────────────────────────

// InstallAWSLambda instruments all Lambda functions in the current AWS region
// with the Dynatrace Lambda Layer. When confirm is true, the user is prompted
// before applying changes; when false, changes are applied immediately after
// the preview (used when called from install aws).
func InstallAWSLambda(envURL, token, platformToken string, dryRun, confirm bool) error {
	cyan := color.New(color.FgMagenta)

	fmt.Println()
	cyan.Println("  Dynatrace AWS Lambda Instrumentation")
	fmt.Println()

	// ── Validate ─────────────────────────────────────────────────────────────

	if envURL == "" {
		return fmt.Errorf("Dynatrace environment URL is required (--environment or DT_ENVIRONMENT)") //nolint:ST1005 to keep brand capitalization
	}
	if token == "" {
		return fmt.Errorf("access token is required (--access-token or DT_ACCESS_TOKEN)")
	}

	if !isAWSCLIInstalled() {
		return fmt.Errorf("AWS CLI not found — install it from https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	// ── Get region ───────────────────────────────────────────────────────────

	fmt.Printf("  Fetching AWS region...\n")
	region, err := getLambdaRegion()
	if err != nil {
		return fmt.Errorf("getting AWS region: %w", err)
	}
	fmt.Printf("  Region: %s\n", region)

	// ── Get DT connection info ───────────────────────────────────────────────

	fmt.Printf("  Fetching Dynatrace connection info...\n")
	connInfo, err := getDTConnectionInfo(envURL, token)
	if err != nil {
		return err
	}
	fmt.Printf("  Tenant: %s\n", connInfo.TenantUUID)

	// ── List Lambda functions ────────────────────────────────────────────────

	fmt.Printf("  Listing Lambda functions...\n")
	functions, err := listLambdaFunctions()
	if err != nil {
		return err
	}

	if len(functions) == 0 {
		fmt.Printf("  No Lambda functions found in region %s\n", region)
		return nil
	}

	// ── Classify and resolve layer ARNs ──────────────────────────────────────

	actions := make([]string, len(functions))
	layerARNs := make([]string, len(functions))
	cache := make(layerARNCache)

	for i, fn := range functions {
		action := classifyFunction(fn)
		actions[i] = action

		if action == "skip" {
			continue
		}

		techtype, _ := mapRuntimeToTechtype(fn.Runtime)
		arn, err := getLambdaLayerARN(cache, envURL, token, techtype, fn.Architecture, region)
		if err != nil {
			return fmt.Errorf("resolving layer ARN for %s: %w", fn.Name, err)
		}
		layerARNs[i] = arn
	}

	// ── Preview ──────────────────────────────────────────────────────────────

	actionable, _ := printLambdaPreviewTable(functions, actions)
	fmt.Println()

	if actionable == 0 {
		fmt.Println("  No functions to instrument.")
		return nil
	}

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	// ── Confirm ──────────────────────────────────────────────────────────────

	if confirm {
		ok, err := confirmProceed("  Apply?")
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		if !ok {
			fmt.Println("  Cancelled.")
			return nil
		}
	}
	fmt.Println()

	// ── Instrument ───────────────────────────────────────────────────────────

	succeeded := 0
	failed := 0

	for i, fn := range functions {
		if actions[i] == "skip" {
			continue
		}

		fmt.Printf("  %s %s...", actions[i], fn.Name)
		if err := instrumentFunction(fn, layerARNs[i], connInfo); err != nil {
			fmt.Printf(" failed: %s\n", err)
			failed++
			continue
		}
		fmt.Printf(" done\n")
		succeeded++
	}

	// ── Summary ──────────────────────────────────────────────────────────────

	fmt.Println()
	if failed > 0 {
		fmt.Printf("  %d succeeded, %d failed\n", succeeded, failed)
	} else {
		fmt.Printf("  %d functions instrumented successfully\n", succeeded)
	}

	return nil
}

// UninstallAWSLambda removes the Dynatrace Lambda Layer and DT_* environment
// variables from all instrumented functions in the current AWS region.
func UninstallAWSLambda(dryRun bool) error {
	cyan := color.New(color.FgMagenta)

	fmt.Println()
	cyan.Println("  Dynatrace AWS Lambda — Remove Instrumentation")
	fmt.Println()

	if !isAWSCLIInstalled() {
		return fmt.Errorf("AWS CLI not found — install it from https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	// ── List and filter ──────────────────────────────────────────────────────

	fmt.Printf("  Listing Lambda functions...\n")
	functions, err := listLambdaFunctions()
	if err != nil {
		return err
	}

	var instrumented []lambdaFunction
	for _, fn := range functions {
		if isInstrumented(fn) && !isDynatraceInternal(fn) {
			instrumented = append(instrumented, fn)
		}
	}

	if len(instrumented) == 0 {
		fmt.Println("  No Lambda functions with Dynatrace instrumentation found")
		return nil
	}

	// ── Preview ──────────────────────────────────────────────────────────────

	printLambdaUninstallPreview(instrumented)
	fmt.Println()

	if dryRun {
		fmt.Println("  [dry-run] No changes were made.")
		return nil
	}

	// ── Confirm ──────────────────────────────────────────────────────────────

	ok, err := confirmProceed("  Apply?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Cancelled.")
		return nil
	}
	fmt.Println()

	// ── Uninstrument ─────────────────────────────────────────────────────────

	succeeded := 0
	failed := 0

	for _, fn := range instrumented {
		fmt.Printf("  removing %s...", fn.Name)
		if err := uninstrumentFunction(fn); err != nil {
			fmt.Printf(" failed: %s\n", err)
			failed++
			continue
		}
		fmt.Printf(" done\n")
		succeeded++
	}

	// ── Summary ──────────────────────────────────────────────────────────────

	fmt.Println()
	if failed > 0 {
		fmt.Printf("  %d succeeded, %d failed\n", succeeded, failed)
	} else {
		fmt.Printf("  %d functions uninstrumented successfully\n", succeeded)
	}
	return nil
}
