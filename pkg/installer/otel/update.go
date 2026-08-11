package otel

import (
	"bytes"
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

// UpdateResult holds the outcome of an OTel config update operation.
type UpdateResult struct {
	ConfigPath  string
	BackupPath  string
	Modified    bool
	Description string
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

// showConfigDiff prints a focused diff: only changed lines and up to 2 YAML
// ancestor lines above each change as context, with "..." between gaps.
func showConfigDiff(origData, updatedData []byte) {
	const parentContext = 2

	oldLines := strings.Split(strings.TrimRight(string(origData), "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(string(updatedData), "\n"), "\n")
	edits := diffLines(oldLines, newLines)

	yamlIndent := func(s string) int {
		return len(s) - len(strings.TrimLeft(s, " \t"))
	}
	isStructural := func(s string) bool {
		t := strings.TrimSpace(s)
		return t != "" && !strings.HasPrefix(t, "#")
	}

	show := make([]bool, len(edits))
	for i, e := range edits {
		if e.kind != editAdd && e.kind != editDel {
			continue
		}
		show[i] = true
		indent := yamlIndent(e.line)
		found := 0
		for j := i - 1; j >= 0 && found < parentContext; j-- {
			if e.kind == editAdd && edits[j].kind == editDel {
				continue
			}
			if e.kind == editDel && edits[j].kind == editAdd {
				continue
			}
			if !isStructural(edits[j].line) {
				continue
			}
			if yamlIndent(edits[j].line) < indent {
				show[j] = true
				indent = yamlIndent(edits[j].line)
				found++
			}
		}
	}

	// redactAuth replaces the token value on Authorization lines with "Bearer ***"
	// so secrets are never printed, while the diff kind (add/del/keep) is unchanged.
	redactAuth := func(s string) string {
		if strings.HasPrefix(strings.TrimSpace(s), "Authorization:") {
			return s[:len(s)-len(strings.TrimLeft(s, " \t"))] + `Authorization: "Bearer ***"`
		}
		return s
	}

	lastShown := -2
	for i, e := range edits {
		if !show[i] {
			continue
		}
		if lastShown >= 0 && i > lastShown+1 {
			fmt.Println(display.ColorMuted.Sprint("  ..."))
		}
		displayLine := redactAuth(e.line)
		switch e.kind {
		case editAdd:
			fmt.Println(display.ColorOK.Sprint("+ " + displayLine))
		case editDel:
			fmt.Println(display.ColorError.Sprint("- " + displayLine))
		case editKeep:
			fmt.Println(display.ColorMuted.Sprint("  " + displayLine))
		}
		lastShown = i
	}
}

// nodeMappingGet returns the value node for key in a YAML mapping node,
// or nil if not found.
func nodeMappingGet(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// nodeMappingSet sets the value for key in a YAML mapping node.
// If the key already exists its value is replaced in-place, preserving the
// original key node (and any line comment on it).  Otherwise a new key-value
// pair is appended, preserving insertion order.
func nodeMappingSet(m *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		val,
	)
}

// ensureMappingNode returns the existing mapping value for key in parent,
// creating and inserting an empty mapping node when absent.
func ensureMappingNode(parent *yaml.Node, key string) *yaml.Node {
	if n := nodeMappingGet(parent, key); n != nil && n.Kind == yaml.MappingNode {
		return n
	}
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	nodeMappingSet(parent, key, n)
	return n
}

// buildDTExporterNode returns the yaml.Node subtree for the
// otlp_http/dynatrace exporter definition.
func buildDTExporterNode(apiURL, token string) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "endpoint"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: dtOTLPEndpoint(apiURL)},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "headers"},
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Authorization"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: installer.AuthHeader(token), Style: yaml.DoubleQuotedStyle},
				},
			},
		},
	}
}

// appendExporterToPipeline appends "otlp_http/dynatrace" to the exporters list
// of a single pipeline mapping node.  The existing flow/block style of the
// sequence is preserved.  It is a no-op if the exporter is already listed.
func appendExporterToPipeline(pipeline *yaml.Node, name string) {
	const dtKey = "otlp_http/dynatrace"
	exportersNode := nodeMappingGet(pipeline, "exporters")
	if exportersNode == nil {
		nodeMappingSet(pipeline, "exporters", &yaml.Node{
			Kind:  yaml.SequenceNode,
			Tag:   "!!seq",
			Style: yaml.FlowStyle,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: dtKey},
			},
		})
		logger.Debug("adding Dynatrace exporter to pipeline", "pipeline", name)
		return
	}
	if exportersNode.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range exportersNode.Content {
		if item.Value == dtKey {
			logger.Debug("Dynatrace exporter already present in pipeline", "pipeline", name)
			return
		}
	}
	logger.Debug("adding Dynatrace exporter to pipeline", "pipeline", name)
	exportersNode.Content = append(exportersNode.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: dtKey,
	})
}

// mergeDynatraceExporter injects the otlp_http/dynatrace exporter into the
// root yaml.Node of an OTel Collector config and appends it to every existing
// pipeline's exporters list.  Comments, key order, and sequence style are
// preserved because the yaml.Node tree is edited in place rather than
// unmarshalled into a plain map.
func mergeDynatraceExporter(root *yaml.Node, apiURL, token string) {
	exporters := ensureMappingNode(root, "exporters")
	nodeMappingSet(exporters, "otlp_http/dynatrace", buildDTExporterNode(apiURL, token))

	service := nodeMappingGet(root, "service")
	if service == nil || service.Kind != yaml.MappingNode {
		return
	}
	pipelines := nodeMappingGet(service, "pipelines")
	if pipelines == nil || pipelines.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(pipelines.Content); i += 2 {
		pipeline := pipelines.Content[i+1]
		if pipeline.Kind != yaml.MappingNode {
			continue
		}
		appendExporterToPipeline(pipeline, pipelines.Content[i].Value)
	}
}

// mergeExporterIntoYAML parses data as a yaml.Node tree, injects the
// Dynatrace exporter via mergeDynatraceExporter, and re-serialises the result.
// Unlike a plain Unmarshal→Marshal roundtrip, this preserves YAML comments,
// key insertion order, and flow/block sequence style.
func mergeExporterIntoYAML(data []byte, apiURL, token string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	var root *yaml.Node
	switch {
	case doc.Kind == yaml.DocumentNode && len(doc.Content) > 0:
		root = doc.Content[0]
		if root.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("YAML root must be a mapping, got kind %d", root.Kind)
		}
	default:
		// Empty or null document — start with a fresh block mapping.
		root = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	// An inline empty map ("{}") must become a block mapping once we add keys.
	if root.Style == yaml.FlowStyle {
		root.Style = 0
	}

	mergeDynatraceExporter(root, apiURL, token)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, fmt.Errorf("marshalling updated YAML: %w", err)
	}
	return buf.Bytes(), nil
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
	var selectedIsDynatrace bool

	allProcs := findAllRunningOtelCollectorsFunc()
	if len(allProcs) > 0 {
		display.ColorMessage.Println("  Running OTel Collectors:")
		fmt.Println()
		selected, err := selectCollector(allProcs)
		if err != nil {
			return err // includes installer.ErrInstallCancelled
		}
		if selected != nil {
			selectedIsDynatrace = selected.isDynatrace
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

	if selectedIsDynatrace {
		return updateDynatraceCollector(configPath, runningProcs, envURL, token, platformTok, dryRun)
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
	var isDynatraceCollector bool
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
			isDynatraceCollector = isDynatraceCollector || inst.isDynatrace
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

	if isDynatraceCollector {
		return updateDynatraceCollector(configPath, runningProcs, envURL, token, platformTok, dryRun)
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
				name := filepath.Base(p.binaryPath)
				if name == "" || name == "." {
					name = "(unknown)"
				}
				fmt.Printf("    • PID %d  %s\n", p.pid, display.ColorDefault.Sprint(name))
			}
		}
	} else {
		display.ColorDefault.Println("  No running collector found — config will be updated on disk only.")
	}

	// Detect app services; exclude the collector's own PID(s).
	excludePIDs := make(map[int]bool, len(runningProcs))
	for _, p := range runningProcs {
		excludePIDs[p.pid] = true
	}
	connectedSvcs := detectConnectedServices(origData, excludePIDs)
	if len(connectedSvcs) > 0 {
		fmt.Println()
		printConnectedServices(connectedSvcs)
		fmt.Println()
		display.ColorDefault.Println("  These services will be restarted after the collector.")
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

		if err := verifyOtelInstall(envURL, platformTok, token, otlpHTTPPortFromConfig(configPath), crashed); err != nil {
			fmt.Printf("\n  Warning: log verification failed: %v\n", err)
			fmt.Println("  The collector may still be working — check the Dynatrace UI.")
			return nil
		}
	}

	if len(containerProcs) > 0 {
		// Best-effort verification for containers: the OTLP port may or may not be
		// exposed to the host depending on how the container was started.
		noCrash := make(chan error) // never sends — no process to monitor
		if err := verifyOtelInstall(envURL, platformTok, token, otlpHTTPPortFromConfig(configPath), noCrash); err != nil {
			fmt.Printf("\n  Warning: log verification failed: %v\n", err)
			fmt.Println("  The collector may still be working — check the Dynatrace UI.")
			return nil
		}
	}

	display.ColorOK.Println("  ✓ Collector restarted and verified.")

	if len(connectedSvcs) > 0 {
		fmt.Println()
		restartConnectedServices(connectedSvcs)
	}

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
