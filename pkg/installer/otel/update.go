package otel

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// exporterSnippetTemplate is the YAML block to inject into an existing OTel Collector
// configuration as the `otlp_http/dynatrace` exporter.
// The second %s receives the full Authorization header value (e.g. "Bearer …" or "Api-Token …").
const exporterSnippetTemplate = `otlp_http/dynatrace:
  endpoint: %s
  headers:
    Authorization: "%s"
`

// backupFile writes data to a timestamped .bak.<unix> copy of path and returns
// the backup path.
func backupFile(path string, data []byte) (string, error) {
	backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return "", fmt.Errorf("creating backup at %s: %w", backupPath, err)
	}
	logger.Debug("config backup created", "backup", backupPath, "originalBytes", len(data))
	return backupPath, nil
}

// dtOTLPEndpoint returns the full Dynatrace OTLP ingest endpoint for apiURL.
func dtOTLPEndpoint(apiURL string) string {
	return strings.TrimRight(apiURL, "/") + "/api/v2/otlp"
}

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
		dtOTLPEndpoint(apiURL),
		installer.AuthHeader(token),
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
			switch {
			case a[i-1] == b[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] > dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
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
			edits = append(edits, diffEdit{editKeep, oldLines[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			edits = append(edits, diffEdit{editAdd, newLines[j-1]})
			j--
		default:
			edits = append(edits, diffEdit{editDel, oldLines[i-1]})
			i--
		}
	}
	slices.Reverse(edits)
	return edits
}

// showConfigDiff prints a coloured line diff to stdout.
// Added lines are green (+), removed lines are red (-), unchanged lines are dimmed.
func showConfigDiff(origData, updatedData []byte) {
	oldLines := strings.Split(strings.TrimRight(string(origData), "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(string(updatedData), "\n"), "\n")

	for _, e := range diffLines(oldLines, newLines) {
		switch e.kind {
		case editAdd:
			fmt.Println(display.ColorOK.Sprint("+ " + e.line))
		case editDel:
			fmt.Println(display.ColorError.Sprint("- " + e.line))
		case editKeep:
			fmt.Println(display.ColorMuted.Sprint("  " + e.line))
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
		"endpoint": dtOTLPEndpoint(apiURL),
		"headers": map[string]interface{}{
			"Authorization": installer.AuthHeader(token),
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
			logger.Debug("adding Dynatrace exporter to pipeline", "pipeline", pipelineName)
			pipeline["exporters"] = append(existing, "otlp_http/dynatrace")
			pipelines[pipelineName] = pipeline
		} else {
			logger.Debug("Dynatrace exporter already present in pipeline", "pipeline", pipelineName)
		}
	}
}

// mergeExporterIntoYAML unmarshals data, injects the Dynatrace exporter via
// mergeDynatraceExporter, and returns the re-marshalled YAML.
func mergeExporterIntoYAML(data []byte, apiURL, token string) ([]byte, error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	mergeDynatraceExporter(cfg, apiURL, token)
	return yaml.Marshal(cfg)
}

// writeConfig writes updatedData to configPath and returns the result.
func writeConfig(configPath string, updatedData []byte) (*UpdateResult, error) {
	if err := os.WriteFile(configPath, updatedData, 0o600); err != nil {
		return nil, fmt.Errorf("writing updated config to %s: %w", configPath, err)
	}

	return &UpdateResult{
		ConfigPath:  configPath,
		Modified:    true,
		Description: "Dynatrace otlp_http/dynatrace exporter merged into existing config",
	}, nil
}

// PatchConfigFile reads an existing OTel Collector YAML config file, backs it
// up, injects the Dynatrace exporter, and writes the updated config back.
func PatchConfigFile(configPath, apiURL, token string) (*UpdateResult, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	updated, err := mergeExporterIntoYAML(data, apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("patching config %s: %w", configPath, err)
	}

	// Create a timestamped backup.
	backupPath, err := backupFile(configPath, data)
	if err != nil {
		return nil, err
	}

	result, err := writeConfig(configPath, updated)
	if err != nil {
		return nil, err
	}
	result.BackupPath = backupPath
	return result, nil
}

// UpdateOtelConfigInteractive presents a list of running OTel Collectors, lets
// the user pick one, resolves its config path, and then delegates to
// UpdateOtelConfig to apply the Dynatrace exporter patch.
func UpdateOtelConfigInteractive(envURL, token, platformTok string, dryRun bool) error {
	var configPath string
	var runningProcs []otelProcessInfo

	allProcs := findAllRunningOtelCollectorsFunc()
	if len(allProcs) > 0 {
		display.ColorMessage.Println("  Running OTel Collectors:")
		fmt.Println()
		selected, err := selectCollector(allProcs)
		if err != nil {
			return err // includes installer.ErrInstallCancelled
		}
		if selected != nil {
			if selected.configPath != "" {
				configPath = selected.configPath
			}
			installDir := ""
			if selected.binaryPath != "" {
				installDir = filepath.Dir(selected.binaryPath)
			}
			proc := otelProcessInfo{
				pid:              selected.pid,
				binaryPath:       selected.binaryPath,
				installDir:       installDir,
				containerRuntime: selected.containerRuntime,
				containerName:    selected.containerName,
			}
			// Track container-internal config path for copy-back only when
			// the config is not host-mounted (configPath is still empty here).
			if configPath == "" && selected.containerRuntime != "" {
				proc.containerCfgPath = selected.containerConfigPath
			}
			runningProcs = []otelProcessInfo{proc}
		}
	} else {
		display.ColorMuted.Println("  No running OTel Collectors found.")
		fmt.Println()
	}

	if configPath == "" {
		if len(allProcs) == 0 {
			return fmt.Errorf("no running OTel Collectors found — use --config to specify the config file path")
		}
		// Container with config inside the image: extract to a temp file so we
		// can patch it, then copy it back and restart the container.
		if len(runningProcs) == 1 && runningProcs[0].containerRuntime != "" && runningProcs[0].containerCfgPath != "" {
			tmpPath, err := extractContainerConfig(
				runningProcs[0].containerRuntime,
				runningProcs[0].containerName,
				runningProcs[0].containerCfgPath,
			)
			if err != nil {
				return fmt.Errorf("extracting config from container %s: %w", runningProcs[0].containerName, err)
			}
			defer os.Remove(tmpPath)
			configPath = tmpPath
			fmt.Printf("  Extracted config from container (%s → temp file)\n", runningProcs[0].containerCfgPath)
		} else {
			return fmt.Errorf("could not determine config path for the selected collector — use --config to specify it")
		}
	}
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	return updateOtelConfig(configPath, runningProcs, envURL, token, platformTok, dryRun)
}

// UpdateOtelConfig updates an existing OTel Collector config file with the
// Dynatrace exporter. Shows a coloured diff preview and asks for confirmation
// before writing. After patching, the selected running collector is killed and
// restarted with the updated config, and a verification log is sent.
// If dryRun is true the preview is printed without prompting.
// configPath must be non-empty; use UpdateOtelConfigInteractive when the path
// is not known and should be resolved from running collectors.
func UpdateOtelConfig(configPath, envURL, token, platformTok string, dryRun bool) error {
	if configPath == "" {
		return fmt.Errorf("config path must not be empty — use --config or UpdateOtelConfigInteractive")
	}

	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config file not found: %s", configPath)
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	var runningProcs []otelProcessInfo
	for _, inst := range findAllRunningOtelCollectorsFunc() {
		if inst.configPath == "" {
			continue
		}
		// Skip non-running native processes (pid == 0 with no container runtime).
		// Containers always have pid == 0 but are running — include them when their
		// host-mounted config path matches.
		if inst.containerRuntime == "" && inst.pid <= 0 {
			continue
		}
		instAbs, err := filepath.Abs(inst.configPath)
		if err != nil {
			continue
		}
		if instAbs == absConfig {
			installDir := ""
			if inst.binaryPath != "" {
				installDir = filepath.Dir(inst.binaryPath)
			}
			runningProcs = append(runningProcs, otelProcessInfo{
				pid:              inst.pid,
				binaryPath:       inst.binaryPath,
				installDir:       installDir,
				containerRuntime: inst.containerRuntime,
				containerName:    inst.containerName,
				// containerCfgPath not needed: config is already on the host
			})
		}
	}

	return updateOtelConfig(configPath, runningProcs, envURL, token, platformTok, dryRun)
}

// updateOtelConfig is the shared implementation that applies the Dynatrace
// exporter patch given a resolved configPath and the set of running collectors
// to restart afterward.
func updateOtelConfig(configPath string, runningProcs []otelProcessInfo, envURL, token, platformTok string, dryRun bool) error {
	apiURL := installer.APIURL(envURL)

	logger.Debug("running collectors for restart", "count", len(runningProcs))
	for _, p := range runningProcs {
		logger.Debug("collector for restart", "pid", p.pid, "binary", p.binaryPath)
	}

	// Build a preview of the updated config so we can diff it against the original.
	origData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", configPath, err)
	}
	updatedData, err := mergeExporterIntoYAML(origData, apiURL, token)
	if err != nil {
		return fmt.Errorf("building config preview for %s: %w", configPath, err)
	}

	display.Header(fmt.Sprintf("Preview of changes to %s:", configPath))
	fmt.Println()

	showConfigDiff(origData, updatedData)

	fmt.Println()
	display.PrintSectionDivider()
	fmt.Println()

	// Show restart plan.
	if len(runningProcs) > 0 {
		display.ColorBold.Println("  Running collectors that will be restarted:")
		for _, p := range runningProcs {
			if p.containerRuntime != "" {
				fmt.Printf("    • %s  %s\n",
					p.containerName,
					display.ColorMuted.Sprint("("+p.containerRuntime+" restart)"))
			} else {
				hint := p.binaryPath
				if hint == "" {
					hint = "(unknown binary)"
				}
				fmt.Printf("    • PID %d  %s\n", p.pid, display.ColorDefault.Sprint(hint))
			}
		}
	} else {
		display.ColorDefault.Println("  No running collector found — config will be updated on disk only.")
	}
	fmt.Println()
	display.PrintSectionDivider()

	if dryRun {
		display.ColorDefault.Println("  [dry-run] No changes made.")
		return nil
	}

	ok, err := installer.ConfirmProceed("  Apply changes and restart collector?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		display.ColorDefault.Println("  Cancelled — no changes written.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	// Create a timestamped backup before writing.
	backupPath, err := backupFile(configPath, origData)
	if err != nil {
		return err
	}

	// Write updated config using data already computed for the diff preview.
	result, err := writeConfig(configPath, updatedData)
	if err != nil {
		return err
	}
	fmt.Printf("  Config updated: %s\n", result.ConfigPath)
	fmt.Printf("  Backup created: %s\n", backupPath)
	fmt.Println()

	if len(runningProcs) == 0 {
		display.ColorDefault.Println("  No running collector to restart.")
		return nil
	}

	// Partition into container and native-process restarts.
	var nativeProcs []otelProcessInfo
	var containerProcs []otelProcessInfo
	for _, p := range runningProcs {
		if p.containerRuntime != "" {
			containerProcs = append(containerProcs, p)
		} else {
			nativeProcs = append(nativeProcs, p)
		}
	}

	// Container restart: copy patched config back (when not host-mounted), then restart.
	for _, p := range containerProcs {
		if p.containerCfgPath != "" {
			fmt.Printf("  Copying updated config into container %s...\n", p.containerName)
			if err := copyFileToContainer(p.containerRuntime, p.containerName, configPath, p.containerCfgPath); err != nil {
				return fmt.Errorf("copying config to container %s: %w", p.containerName, err)
			}
		}
		fmt.Printf("  Restarting container %s...\n", p.containerName)
		if err := restartContainer(p.containerRuntime, p.containerName); err != nil {
			return fmt.Errorf("restarting container %s: %w", p.containerName, err)
		}
		fmt.Printf("  Container %s restarted.\n", p.containerName)
	}

	// Native process restart: kill old process, start new one.
	if len(nativeProcs) > 0 {
		restartBinary := killCollectorProcesses(nativeProcs)
		fmt.Println()

		if restartBinary == "" {
			display.ColorDefault.Println("  Could not determine binary path — skipping restart.")
			display.ColorDefault.Println("  Start the collector manually with the updated config.")
			return nil
		}

		fmt.Printf("  Restarting collector with updated config...\n")
		crashed, err := startOtelCollector(restartBinary, configPath)
		if err != nil {
			return fmt.Errorf("restarting collector: %w", err)
		}

		if err := verifyOtelInstall(envURL, platformTok, token, crashed); err != nil {
			fmt.Printf("\n  Warning: log verification failed: %v\n", err)
			fmt.Println("  The collector may still be working — check the Dynatrace UI.")
			return nil
		}
	}

	if len(containerProcs) > 0 {
		// Best-effort verification for containers: port 4318 may or may not be
		// exposed to the host depending on how the container was started.
		noCrash := make(chan error) // never sends — no process to monitor
		if err := verifyOtelInstall(envURL, platformTok, token, noCrash); err != nil {
			fmt.Printf("\n  Warning: log verification failed: %v\n", err)
			fmt.Println("  The collector may still be working — check the Dynatrace UI.")
			return nil
		}
	}

	display.ColorOK.Println("  ✓ Collector restarted and verified.")
	return nil
}

// updateOtelCollectorIfPresent checks for a dtwiz-managed OTel Collector config
// at the well-known paths and patches it with the Dynatrace exporter if found.
// Checks the home-based path first, then falls back to the legacy CWD-based path for
// collectors installed with older versions of dtwiz. No output if the file is absent.
// If dryRun is true, prints what would be updated without making changes.
func updateOtelCollectorIfPresent(envURL, token string, dryRun bool) {
	configPath := findExistingCollectorConfig()
	if configPath == "" {
		return
	}
	if dryRun {
		display.PrintStatusLine("collector", fmt.Sprintf("would update config: %s", configPath), display.ColorMuted)
		return
	}
	_, err := PatchConfigFile(configPath, installer.APIURL(envURL), token)
	if err != nil {
		logger.Debug("failed to update OTel Collector config", "path", configPath, "error", err)
		return
	}
	display.PrintStatusLine("collector", "config updated", display.ColorOK)
}

// findExistingCollectorConfig returns the path to a dtwiz-managed OTel Collector
// config.yaml if one exists, checking the home-based path first, then the legacy
// CWD-based path. Returns "" if neither exists.
func findExistingCollectorConfig() string {
	var candidates []string

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "opentelemetry", "config.yaml"))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "opentelemetry", "config.yaml"))
	}

	for _, p := range candidates {
		if fileExists(p) {
			logger.Debug("otel collector config found", "path", p)
			return p
		}
		logger.Debug("otel collector config not found, skipping", "path", p)
	}
	return ""
}
