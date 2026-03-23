package installer

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

// exporterSnippet is the YAML block to inject into an existing OTel Collector
// configuration as the `otlp_http/dynatrace` exporter.
const exporterSnippetTemplate = `otlp_http/dynatrace:
  endpoint: %s/api/v2/otlp
  headers:
    Authorization: "Api-Token %s"
`

// pipelineHint is the human-readable instruction for wiring the exporter.
const pipelineHint = `Add "otlp_http/dynatrace" to the exporters list of each pipeline you want
to forward to Dynatrace, for example:

  service:
    pipelines:
      traces:
        exporters: [otlp_http/dynatrace]
      metrics:
        exporters: [otlp_http/dynatrace]
      logs:
        exporters: [otlp_http/dynatrace]
`

// UpdateResult holds the outcome of an OTel config update operation.
type UpdateResult struct {
	ConfigPath  string
	BackupPath  string
	Modified    bool
	Description string
}

// GenerateExporterSnippet returns the YAML snippet for the Dynatrace OTLP
// exporter, ready to paste into an existing OTel Collector config.
func GenerateExporterSnippet(apiURL, token string) string {
	return fmt.Sprintf(exporterSnippetTemplate,
		strings.TrimRight(apiURL, "/"),
		token,
	)
}

// GeneratePipelineHint returns instructions for wiring the DT exporter into
// service pipelines.
func GeneratePipelineHint() string {
	return pipelineHint
}

// GenerateFullInstructions returns a human-readable guide for manually adding
// the Dynatrace exporter to an existing OTel Collector configuration.
func GenerateFullInstructions(apiURL, token string) string {
	var sb strings.Builder
	sb.WriteString("Add the following to the `exporters:` section of your OTel Collector config:\n\n")
	sb.WriteString(GenerateExporterSnippet(apiURL, token))
	sb.WriteString("\n")
	sb.WriteString(GeneratePipelineHint())
	return sb.String()
}

// editKind represents the type of a line diff operation.
type editKind int

const (
	editKeep editKind = iota
	editAdd
	editDel
)

type diffEdit struct {
	kind editKind
	line string
}

// lcsDP builds the LCS dynamic-programming table for two string slices.
func lcsDP(a, b []string) [][]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp
}

// diffLines computes a line-level diff, returning keep/add/delete operations.
func diffLines(oldLines, newLines []string) []diffEdit {
	dp := lcsDP(oldLines, newLines)
	var edits []diffEdit
	i, j := len(oldLines), len(newLines)
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			edits = append([]diffEdit{{editKeep, oldLines[i-1]}}, edits...)
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			edits = append([]diffEdit{{editAdd, newLines[j-1]}}, edits...)
			j--
		default:
			edits = append([]diffEdit{{editDel, oldLines[i-1]}}, edits...)
			i--
		}
	}
	return edits
}

// showConfigDiff prints a coloured line diff to stdout.
// Added lines are green (+), removed lines are red (-), unchanged lines are dimmed.
func showConfigDiff(origData, updatedData []byte) {
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed)
	dim := color.New()

	oldLines := strings.Split(strings.TrimRight(string(origData), "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(string(updatedData), "\n"), "\n")

	for _, e := range diffLines(oldLines, newLines) {
		switch e.kind {
		case editAdd:
			fmt.Println(green.Sprint("+ " + e.line))
		case editDel:
			fmt.Println(red.Sprint("- " + e.line))
		case editKeep:
			fmt.Println(dim.Sprint("  " + e.line))
		}
	}
}

// mergeDynatraceExporter deep-merges the Dynatrace exporter definition into
// the `exporters` key of the provided config map.  It also appends
// `otlp_http/dynatrace` to the exporters list of every existing pipeline.
func mergeDynatraceExporter(cfg map[string]interface{}, apiURL, token string) {
	// Ensure exporters key exists.
	exporters, ok := cfg["exporters"].(map[string]interface{})
	if !ok {
		exporters = make(map[string]interface{})
		cfg["exporters"] = exporters
	}

	exporters["otlp_http/dynatrace"] = map[string]interface{}{
		"endpoint": strings.TrimRight(apiURL, "/") + "/api/v2/otlp",
		"headers": map[string]interface{}{
			"Authorization": "Api-Token " + token,
		},
	}

	// Append to existing pipeline exporters.
	service, ok := cfg["service"].(map[string]interface{})
	if !ok {
		return
	}
	pipelines, ok := service["pipelines"].(map[string]interface{})
	if !ok {
		return
	}
	for pipelineName, pipelineVal := range pipelines {
		pipeline, ok := pipelineVal.(map[string]interface{})
		if !ok {
			continue
		}
		existing, _ := pipeline["exporters"].([]interface{})
		// Don't add duplicates.
		alreadyPresent := false
		for _, e := range existing {
			if e == "otlp_http/dynatrace" {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			pipeline["exporters"] = append(existing, "otlp_http/dynatrace")
			pipelines[pipelineName] = pipeline
		}
	}
}

// PatchConfigFile reads an existing OTel Collector YAML config file, backs it
// up, injects the Dynatrace exporter, and writes the updated config back.
func PatchConfigFile(configPath, apiURL, token string) (*UpdateResult, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML config %s: %w", configPath, err)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	// Create a timestamped backup.
	backupPath := fmt.Sprintf("%s.bak.%d", configPath, time.Now().Unix())
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("creating backup at %s: %w", backupPath, err)
	}

	mergeDynatraceExporter(cfg, apiURL, token)

	updated, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("serialising updated config: %w", err)
	}

	if err := os.WriteFile(configPath, updated, 0o600); err != nil {
		return nil, fmt.Errorf("writing updated config to %s: %w", configPath, err)
	}

	return &UpdateResult{
		ConfigPath:  configPath,
		BackupPath:  backupPath,
		Modified:    true,
		Description: "Dynatrace otlp_http/dynatrace exporter merged into existing config",
	}, nil
}

// UpdateOtelConfig updates an existing OTel Collector config file with the
// Dynatrace exporter.  Shows a coloured diff preview and asks for confirmation
// before writing.  After patching, any running collector processes are killed
// and restarted with the updated config, and a verification log is sent.
// If dryRun is true the preview is printed without prompting.
// If configPath is empty or points to a non-existent file the user is prompted
// to supply the correct path interactively.
func UpdateOtelConfig(configPath, envURL, token, platformToken string, dryRun bool) error {
	apiURL := APIURL(envURL)

	// Resolve the config path: prompt when it's missing or the file doesn't exist.
	for {
		if configPath != "" {
			if _, err := os.Stat(configPath); err == nil {
				break // file exists — proceed
			}
			fmt.Printf("  Config file not found: %s\n", configPath)
		}
		var err error
		configPath, err = promptLine("  Path to OTel Collector config file", "config.yaml")
		if err != nil {
			return fmt.Errorf("reading config path: %w", err)
		}
	}

	// Discover running collectors now so we can include them in the preview.
	runningProcs := findRunningOtelProcesses()

	// Build a preview of the updated config so we can diff it against the original.
	origData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", configPath, err)
	}
	var cfgPreview map[string]interface{}
	if err := yaml.Unmarshal(origData, &cfgPreview); err != nil {
		return fmt.Errorf("parsing YAML config %s: %w", configPath, err)
	}
	if cfgPreview == nil {
		cfgPreview = make(map[string]interface{})
	}
	mergeDynatraceExporter(cfgPreview, apiURL, token)
	updatedData, err := yaml.Marshal(cfgPreview)
	if err != nil {
		return fmt.Errorf("serialising preview: %w", err)
	}

	header := color.New(color.FgMagenta, color.Bold)
	muted := color.New()
	bold := color.New(color.FgWhite, color.Bold)
	green := color.New(color.FgGreen, color.Bold)

	header.Printf("  Preview of changes to %s:\n", configPath)
	muted.Println("  " + strings.Repeat("─", 60))
	fmt.Println()
	showConfigDiff(origData, updatedData)
	fmt.Println()
	muted.Println("  " + strings.Repeat("─", 60))
	fmt.Println()

	// Show restart plan.
	if len(runningProcs) > 0 {
		bold.Println("  Running collectors that will be restarted:")
		for _, p := range runningProcs {
			hint := p.binaryPath
			if hint == "" {
				hint = "(unknown binary)"
			}
			fmt.Printf("    • PID %d  %s\n", p.pid, muted.Sprint(hint))
		}
	} else {
		muted.Println("  No running collector found — config will be updated on disk only.")
	}
	fmt.Println()
	muted.Println("  " + strings.Repeat("─", 60))

	if dryRun {
		muted.Println("  [dry-run] No changes made.")
		return nil
	}

	ok, err := confirmProceed("  Apply changes and restart collector?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		muted.Println("  Cancelled — no changes written.")
		return nil
	}
	fmt.Println()

	// Patch the config file.
	result, err := PatchConfigFile(configPath, apiURL, token)
	if err != nil {
		return err
	}
	fmt.Printf("  Config updated: %s\n", result.ConfigPath)
	fmt.Printf("  Backup created: %s\n", result.BackupPath)
	fmt.Println()

	if len(runningProcs) == 0 {
		muted.Println("  No running collector to restart.")
		return nil
	}

	// Kill all running collectors (shared helper) and get the binary for restart.
	restartBinary := killCollectorProcesses(runningProcs)
	fmt.Println()

	if restartBinary == "" {
		muted.Println("  Could not determine binary path — skipping restart.")
		muted.Println("  Start the collector manually with the updated config.")
		return nil
	}

	fmt.Printf("  Restarting collector with updated config...\n")
	crashed, err := startOtelCollector(restartBinary, configPath)
	if err != nil {
		return fmt.Errorf("restarting collector: %w", err)
	}

	// Verify the restarted collector can deliver logs to Dynatrace.
	if err := verifyOtelInstall(envURL, platformToken, token, crashed); err != nil {
		fmt.Printf("\n  Warning: log verification failed: %v\n", err)
		fmt.Println("  The collector may still be working — check the Dynatrace UI.")
		return nil
	}

	green.Println("  ✓ Collector restarted and verified.")
	return nil
}

