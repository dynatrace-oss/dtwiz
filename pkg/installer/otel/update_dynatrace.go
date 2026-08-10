package otel

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// hostMonitoringReceiverKeys are the receiver names added by host monitoring.
var hostMonitoringReceiverKeys = []string{
	"host_metrics/10s", "host_metrics/5m", "host_metrics/1h", "journald",
}

// hostMonitoringProcessorKeys are the processor names added by host monitoring.
var hostMonitoringProcessorKeys = []string{
	"filter", "resource_detection", "transform", "filter/delete-metrics",
}

// hostMonitoringPipelineKeys are the pipeline names added by host monitoring.
var hostMonitoringPipelineKeys = []string{"metrics/host", "logs/host"}

// isHostMonitoringPresent reports whether data contains any hostmetrics/* receivers.
func isHostMonitoringPresent(data []byte) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return false
	}
	receivers := nodeMappingGet(doc.Content[0], "receivers")
	if receivers == nil || receivers.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(receivers.Content); i += 2 {
		k := receivers.Content[i].Value
		if strings.HasPrefix(k, "host_metrics") || strings.HasPrefix(k, "hostmetrics") {
			return true
		}
	}
	return false
}

// renderHostMonitoringRef renders the OTel template with HostMonitoring=true using
// canonical port numbers and returns the root mapping yaml.Node. The exporter
// endpoint and auth header are set from apiURL/token for a valid render but are
// not used in the host monitoring comparison or merge steps.
func renderHostMonitoringRef(apiURL, token string) (*yaml.Node, error) {
	tmpl, err := template.New("otel").Parse(otelConfigTemplateText)
	if err != nil {
		return nil, fmt.Errorf("parsing otel template: %w", err)
	}
	data := otelConfigData{
		Endpoint:        strings.TrimRight(apiURL, "/"),
		AuthHeader:      installer.AuthHeader(token),
		MetricsPort:     8888,
		GRPCPort:        4317,
		HTTPPort:        4318,
		HostMonitoring:  true,
		IncludeJournald: runtime.GOOS == "linux",
		HealthCheckPort: 13133,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering host monitoring reference: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(buf.Bytes(), &doc); err != nil {
		return nil, fmt.Errorf("parsing reference YAML: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("unexpected reference YAML structure")
	}
	return doc.Content[0], nil
}

// nodeYAML serialises a yaml.Node to canonical YAML for byte-level comparison.
func nodeYAML(n *yaml.Node) []byte {
	if n == nil {
		return nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(n)
	return bytes.TrimSpace(buf.Bytes())
}

// normalizeCumulativeName replaces the legacy processor name "cumulativetodelta"
// with "cumulative_to_delta" in serialised YAML bytes. The Dynatrace OTel
// Collector template was updated to use the new name; configs installed by
// older dtwiz versions still carry the old name. Normalising both sides before
// comparison avoids false "config out of date" results caused solely by the
// name change.
func normalizeCumulativeName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("cumulativetodelta"), []byte("cumulative_to_delta"))
}

// pipelinesNode returns the service.pipelines mapping node from root, or nil.
func pipelinesNode(root *yaml.Node) *yaml.Node {
	svc := nodeMappingGet(root, "service")
	if svc == nil || svc.Kind != yaml.MappingNode {
		return nil
	}
	return nodeMappingGet(svc, "pipelines")
}

// matchesHostMonitoring reports whether currentRoot's host monitoring sections
// are identical to those in refRoot. health_check port is excluded because it
// is dynamic.
func matchesHostMonitoring(currentRoot, refRoot *yaml.Node) bool {
	currentReceivers := nodeMappingGet(currentRoot, "receivers")
	refReceivers := nodeMappingGet(refRoot, "receivers")
	for _, key := range hostMonitoringReceiverKeys {
		refNode := nodeMappingGet(refReceivers, key)
		if refNode == nil {
			continue // not in reference for this OS; skip
		}
		currentNode := nodeMappingGet(currentReceivers, key)
		if !bytes.Equal(nodeYAML(currentNode), nodeYAML(refNode)) {
			logger.Debug("host monitoring mismatch in receiver", "key", key)
			return false
		}
	}

	currentProcessors := nodeMappingGet(currentRoot, "processors")
	refProcessors := nodeMappingGet(refRoot, "processors")
	for _, key := range hostMonitoringProcessorKeys {
		refNode := nodeMappingGet(refProcessors, key)
		if refNode == nil {
			continue
		}
		currentNode := nodeMappingGet(currentProcessors, key)
		if !bytes.Equal(nodeYAML(currentNode), nodeYAML(refNode)) {
			logger.Debug("host monitoring mismatch in processor", "key", key)
			return false
		}
	}

	currentPipelines := pipelinesNode(currentRoot)
	refPipelines := pipelinesNode(refRoot)
	for _, key := range hostMonitoringPipelineKeys {
		refNode := nodeMappingGet(refPipelines, key)
		if refNode == nil {
			continue
		}
		currentNode := nodeMappingGet(currentPipelines, key)
		// Compare only receivers and processors, not exporters. The exporters list
		// is user-managed: it may include otlp_http/dynatrace or other exporters
		// added by dtwiz or the user, which the reference template does not list.
		// Treating those as a mismatch would trigger an unnecessary re-merge on
		// every subsequent update.
		for _, field := range []string{"receivers", "processors"} {
			refField := nodeMappingGet(refNode, field)
			currentField := nodeMappingGet(currentNode, field)
			if !bytes.Equal(normalizeCumulativeName(nodeYAML(currentField)), normalizeCumulativeName(nodeYAML(refField))) {
				logger.Debug("host monitoring mismatch in pipeline field", "key", key, "field", field)
				return false
			}
		}
	}

	// Verify health_check is present in service.extensions (ignoring the port).
	svc := nodeMappingGet(currentRoot, "service")
	if svc != nil {
		extSeq := nodeMappingGet(svc, "extensions")
		if extSeq == nil || !seqContains(extSeq, "health_check") {
			logger.Debug("health_check missing from service.extensions")
			return false
		}
	}

	return true
}

// seqContains reports whether a sequence yaml.Node contains the given scalar value.
func seqContains(seq *yaml.Node, val string) bool {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return false
	}
	for _, item := range seq.Content {
		if item.Value == val {
			return true
		}
	}
	return false
}

// nodeMappingRename renames oldKey to newKey in mapping node m.
// Returns true if oldKey was found and renamed.
func nodeMappingRename(m *yaml.Node, oldKey, newKey string) bool {
	if m == nil || m.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == oldKey {
			m.Content[i].Value = newKey
			return true
		}
	}
	return false
}

// mergeHostMonitoringIntoConfig copies the host monitoring sections from refRoot
// into currentRoot. Existing non-host-monitoring sections are preserved. The
// health_check extension is only added when not already present, to preserve
// any existing port configuration.
func mergeHostMonitoringIntoConfig(currentRoot, refRoot *yaml.Node) {
	// Receivers.
	currentReceivers := ensureMappingNode(currentRoot, "receivers")
	refReceivers := nodeMappingGet(refRoot, "receivers")
	if refReceivers != nil {
		for _, key := range hostMonitoringReceiverKeys {
			if n := nodeMappingGet(refReceivers, key); n != nil {
				nodeMappingSet(currentReceivers, key, n)
			}
		}
	}

	// Processors.
	currentProcessors := ensureMappingNode(currentRoot, "processors")
	refProcessors := nodeMappingGet(refRoot, "processors")
	if refProcessors != nil {
		for _, key := range hostMonitoringProcessorKeys {
			if n := nodeMappingGet(refProcessors, key); n != nil {
				nodeMappingSet(currentProcessors, key, n)
			}
		}
	}

	// Extensions: only add health_check when absent (preserve existing port).
	currentExtensions := nodeMappingGet(currentRoot, "extensions")
	if currentExtensions == nil || nodeMappingGet(currentExtensions, "health_check") == nil {
		refExtensions := nodeMappingGet(refRoot, "extensions")
		if refExtensions != nil {
			if hc := nodeMappingGet(refExtensions, "health_check"); hc != nil {
				nodeMappingSet(ensureMappingNode(currentRoot, "extensions"), "health_check", hc)
			}
		}
	}

	// Service: ensure health_check is in service.extensions and add host pipelines.
	currentService := ensureMappingNode(currentRoot, "service")
	ensureInExtensionsList(currentService, "health_check")

	currentPipelines := ensureMappingNode(currentService, "pipelines")
	refPipelines := pipelinesNode(refRoot)
	if refPipelines != nil {
		for _, key := range hostMonitoringPipelineKeys {
			if n := nodeMappingGet(refPipelines, key); n != nil {
				nodeMappingSet(currentPipelines, key, n)
			}
		}
	}

	// Older configs use "cumulativetodelta" (no underscores). The reference template
	// uses "cumulative_to_delta". Adapt any pipeline processor references so the
	// collector can resolve them against what's actually defined.
	adaptCumulativeProcessorRefs(currentProcessors, currentPipelines)
}

// adaptCumulativeProcessorRefs rewrites processor references inside each pipeline's
// "processors" sequence to match the actual cumulative-to-delta key name in use.
// Both "cumulativetodelta" and "cumulative_to_delta" are accepted by different
// versions of the Dynatrace OTel Collector; this prevents a mismatch when an older
// config (using "cumulativetodelta") has host monitoring pipelines added that were
// generated from the current template (which uses "cumulative_to_delta").
func adaptCumulativeProcessorRefs(processors, pipelines *yaml.Node) {
	const oldName = "cumulativetodelta"
	const newName = "cumulative_to_delta"

	if processors == nil {
		return
	}
	// If the config already uses the new name, nothing to do.
	if nodeMappingGet(processors, newName) != nil {
		return
	}
	// If the config uses the old name, rewrite references in all pipeline processor lists.
	if nodeMappingGet(processors, oldName) == nil {
		return
	}
	if pipelines == nil || pipelines.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(pipelines.Content); i += 2 {
		pipeline := pipelines.Content[i+1]
		if pipeline.Kind != yaml.MappingNode {
			continue
		}
		seq := nodeMappingGet(pipeline, "processors")
		if seq == nil || seq.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range seq.Content {
			if item.Value == newName {
				item.Value = oldName
			}
		}
	}
}

// ensureInExtensionsList adds name to service.extensions if not already present.
func ensureInExtensionsList(service *yaml.Node, name string) {
	extSeq := nodeMappingGet(service, "extensions")
	if extSeq == nil {
		nodeMappingSet(service, "extensions", &yaml.Node{
			Kind:  yaml.SequenceNode,
			Tag:   "!!seq",
			Style: yaml.FlowStyle,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			},
		})
		return
	}
	if extSeq.Kind != yaml.SequenceNode || seqContains(extSeq, name) {
		return
	}
	extSeq.Content = append(extSeq.Content, &yaml.Node{
		Kind: yaml.ScalarNode, Tag: "!!str", Value: name,
	})
}

// updateOtlpExporter updates exporters.otlp_http.endpoint and
// exporters.otlp_http.headers.Authorization in root if they differ from the
// values derived from apiURL and token. Returns true if any field was changed.
func updateOtlpExporter(root *yaml.Node, apiURL, token string) bool {
	exporters := nodeMappingGet(root, "exporters")
	if exporters == nil {
		return false
	}
	otlpHTTP := nodeMappingGet(exporters, "otlp_http")
	if otlpHTTP == nil {
		return false
	}

	wantEndpoint := strings.TrimRight(apiURL, "/") + "/api/v2/otlp"
	wantAuth := installer.AuthHeader(token)

	changed := false

	if n := nodeMappingGet(otlpHTTP, "endpoint"); n != nil && n.Value != wantEndpoint {
		logger.Debug("updating otlp_http endpoint", "old", n.Value, "new", wantEndpoint)
		n.Value = wantEndpoint
		changed = true
	}

	if headers := nodeMappingGet(otlpHTTP, "headers"); headers != nil {
		if n := nodeMappingGet(headers, "Authorization"); n != nil && n.Value != wantAuth {
			logger.Debug("updating otlp_http Authorization header")
			n.Value = wantAuth
			changed = true
		}
	}

	return changed
}

// migrateDeprecatedAliases renames deprecated OTel component names to their canonical
// equivalents in-place. Returns true if any rename was performed.
//   - "hostmetrics/*" → "host_metrics/*": receiver definition keys and pipeline references
//   - "cumulativetodelta" → "cumulative_to_delta": processor definition key and pipeline references
func migrateDeprecatedAliases(root *yaml.Node) bool {
	const (
		oldCumulative  = "cumulativetodelta"
		newCumulative  = "cumulative_to_delta"
		oldHostMetrics = "hostmetrics"
		newHostMetrics = "host_metrics"
	)
	changed := false

	if receivers := nodeMappingGet(root, "receivers"); receivers != nil && receivers.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(receivers.Content); i += 2 {
			k := receivers.Content[i].Value
			if strings.HasPrefix(k, oldHostMetrics+"/") || k == oldHostMetrics {
				receivers.Content[i].Value = newHostMetrics + k[len(oldHostMetrics):]
				logger.Debug("migrated receiver alias", "old", k, "new", receivers.Content[i].Value)
				changed = true
			}
		}
	}

	if processors := nodeMappingGet(root, "processors"); processors != nil && processors.Kind == yaml.MappingNode {
		if nodeMappingRename(processors, oldCumulative, newCumulative) {
			logger.Debug("migrated processor alias", "old", oldCumulative, "new", newCumulative)
			changed = true
		}
	}

	if pipelines := pipelinesNode(root); pipelines != nil && pipelines.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(pipelines.Content); i += 2 {
			pipeline := pipelines.Content[i+1]
			if pipeline.Kind != yaml.MappingNode {
				continue
			}
			if seq := nodeMappingGet(pipeline, "receivers"); seq != nil && seq.Kind == yaml.SequenceNode {
				for _, item := range seq.Content {
					if strings.HasPrefix(item.Value, oldHostMetrics+"/") || item.Value == oldHostMetrics {
						item.Value = newHostMetrics + item.Value[len(oldHostMetrics):]
						changed = true
					}
				}
			}
			if seq := nodeMappingGet(pipeline, "processors"); seq != nil && seq.Kind == yaml.SequenceNode {
				for _, item := range seq.Content {
					if item.Value == oldCumulative {
						item.Value = newCumulative
						changed = true
					}
				}
			}
		}
	}

	return changed
}

// needsDTExporterUpdate reports whether the otlp_http/dynatrace exporter definition
// or pipeline references need to be added or updated to match apiURL and token.
func needsDTExporterUpdate(root *yaml.Node, apiURL, token string) bool {
	wantEndpoint := dtOTLPEndpoint(apiURL)
	wantAuth := installer.AuthHeader(token)

	exporters := nodeMappingGet(root, "exporters")
	if exporters == nil || exporters.Kind != yaml.MappingNode {
		return true
	}
	dtExp := nodeMappingGet(exporters, "otlp_http/dynatrace")
	if dtExp == nil || dtExp.Kind != yaml.MappingNode {
		return true
	}
	endpoint := nodeMappingGet(dtExp, "endpoint")
	if endpoint == nil || endpoint.Value != wantEndpoint {
		return true
	}
	headers := nodeMappingGet(dtExp, "headers")
	if headers == nil {
		return true
	}
	auth := nodeMappingGet(headers, "Authorization")
	if auth == nil || auth.Value != wantAuth {
		return true
	}

	pipelines := pipelinesNode(root)
	if pipelines == nil || pipelines.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(pipelines.Content); i += 2 {
		pipeline := pipelines.Content[i+1]
		if pipeline.Kind != yaml.MappingNode {
			continue
		}
		if !seqContains(nodeMappingGet(pipeline, "exporters"), "otlp_http/dynatrace") {
			return true
		}
	}
	return false
}

// marshalNode encodes root back to YAML bytes.
func marshalNode(root *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// updateDynatraceCollector implements the host monitoring update flow for a
// Dynatrace OTel Collector. When host monitoring is absent it is merged in;
// when present it is compared to the reference and updated only if stale.
func updateDynatraceCollector(configPath string, runningProcs []otelProcessInfo, envURL, token, platformTok string, dryRun bool) error {
	startTime := time.Now()
	apiURL := installer.APIURL(envURL)

	origData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	refRoot, err := renderHostMonitoringRef(apiURL, token)
	if err != nil {
		return fmt.Errorf("generating host monitoring reference: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(origData, &doc); err != nil {
		return fmt.Errorf("parsing config %s: %w", configPath, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure in %s", configPath)
	}
	currentRoot := doc.Content[0]

	aliasesChanged := migrateDeprecatedAliases(currentRoot)
	dtExporterChanged := needsDTExporterUpdate(currentRoot, apiURL, token)

	hostMonPresent := isHostMonitoringPresent(origData)
	hostMonNeedsUpdate := !hostMonPresent || !matchesHostMonitoring(currentRoot, refRoot)

	if !aliasesChanged && !dtExporterChanged && !hostMonNeedsUpdate {
		display.ColorOK.Println("  ✓ Collector configuration is already up to date.")
		return nil
	}

	if !hostMonPresent && hostMonNeedsUpdate {
		display.Header(fmt.Sprintf("Adding host monitoring to %s:", configPath))
	} else {
		display.Header(fmt.Sprintf("Updating collector configuration in %s:", configPath))
	}
	fmt.Println()

	if hostMonNeedsUpdate {
		mergeHostMonitoringIntoConfig(currentRoot, refRoot)
		// Re-check: the host monitoring merge overwrites host pipelines from the
		// reference template, which does not include otlp_http/dynatrace. If the
		// exporter was already wired before the merge, it needs to be re-added.
		if !dtExporterChanged {
			dtExporterChanged = needsDTExporterUpdate(currentRoot, apiURL, token)
		}
	}
	if dtExporterChanged {
		mergeDynatraceExporter(currentRoot, apiURL, token)
	}
	updatedData, err := marshalNode(currentRoot)
	if err != nil {
		return fmt.Errorf("marshalling updated config: %w", err)
	}

	showConfigDiff(origData, updatedData)

	fmt.Println()
	display.PrintSectionDivider()
	fmt.Println()

	if len(runningProcs) > 0 {
		display.ColorBold.Println("  Running collectors that will be restarted:")
		for _, p := range runningProcs {
			if p.containerRuntime != "" {
				fmt.Printf("    • %s  %s\n", p.containerName, display.ColorMuted.Sprint("("+p.containerRuntime+" restart)"))
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

	excludePIDs := make(map[int]bool, len(runningProcs))
	for _, p := range runningProcs {
		excludePIDs[p.pid] = true
	}
	connectedSvcs := detectConnectedServices(origData, excludePIDs)
	if len(connectedSvcs) > 0 {
		// Services that currently export directly to a Dynatrace tenant must be
		// routed through the local collector so that data reaches all configured
		// export destinations (both the existing and newly added exporter).
		collectorEndpoint := fmt.Sprintf("http://localhost:%d", otlpHTTPPortFromConfig(configPath))
		for i := range connectedSvcs {
			if _, changed := retargetEnvToCollector(connectedSvcs[i].env, collectorEndpoint); changed {
				connectedSvcs[i].collectorEndpoint = collectorEndpoint
			}
		}
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

	backupPath, err := backupFile(configPath, origData)
	if err != nil {
		return err
	}
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

	var nativeProcs, containerProcs []otelProcessInfo
	for _, p := range runningProcs {
		if p.containerRuntime != "" {
			containerProcs = append(containerProcs, p)
		} else {
			nativeProcs = append(nativeProcs, p)
		}
	}

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
		noCrash := make(chan error)
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

	installer.WatchIngest(envURL, platformTok, startTime.UTC().Format(installer.IngestTimeFormat))
	return nil
}
