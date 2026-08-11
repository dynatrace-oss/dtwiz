package otel

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
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
			if !bytes.Equal(nodeYAML(currentField), nodeYAML(refField)) {
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

// portsFromConfig reads the OTLP receiver ports and internal telemetry ports from
// an existing OTel Collector config, falling back to canonical defaults for any
// field that cannot be parsed.
func portsFromConfig(configPath string) (grpc, http, metrics, healthCheck int) {
	grpc, http, metrics, healthCheck = 4317, 4318, 8888, 13133
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return
	}
	root := doc.Content[0]

	parsePort := func(endpoint string) (int, bool) {
		_, portStr, err := net.SplitHostPort(endpoint)
		if err != nil {
			return 0, false
		}
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 {
			return 0, false
		}
		return p, true
	}

	if receivers := nodeMappingGet(root, "receivers"); receivers != nil {
		if otlp := nodeMappingGet(receivers, "otlp"); otlp != nil {
			if protocols := nodeMappingGet(otlp, "protocols"); protocols != nil {
				if grpcProto := nodeMappingGet(protocols, "grpc"); grpcProto != nil {
					if ep := nodeMappingGet(grpcProto, "endpoint"); ep != nil {
						if p, ok := parsePort(ep.Value); ok {
							grpc = p
						}
					}
				}
				if httpProto := nodeMappingGet(protocols, "http"); httpProto != nil {
					if ep := nodeMappingGet(httpProto, "endpoint"); ep != nil {
						if p, ok := parsePort(ep.Value); ok {
							http = p
						}
					}
				}
			}
		}
	}

	if svc := nodeMappingGet(root, "service"); svc != nil {
		if telemetry := nodeMappingGet(svc, "telemetry"); telemetry != nil {
			if metricsNode := nodeMappingGet(telemetry, "metrics"); metricsNode != nil {
				if readers := nodeMappingGet(metricsNode, "readers"); readers != nil && readers.Kind == yaml.SequenceNode && len(readers.Content) > 0 {
					pull := nodeMappingGet(readers.Content[0], "pull")
					exporter := nodeMappingGet(pull, "exporter")
					prom := nodeMappingGet(exporter, "prometheus")
					if portNode := nodeMappingGet(prom, "port"); portNode != nil {
						if p, err := strconv.Atoi(portNode.Value); err == nil && p > 0 {
							metrics = p
						}
					}
				}
			}
		}
	}

	if extensions := nodeMappingGet(root, "extensions"); extensions != nil {
		if hc := nodeMappingGet(extensions, "health_check"); hc != nil {
			if ep := nodeMappingGet(hc, "endpoint"); ep != nil {
				if p, ok := parsePort(ep.Value); ok {
					healthCheck = p
				}
			}
		}
	}

	return
}

// updateDynatraceCollector regenerates the Dynatrace OTel Collector config from
// the install template with new tenant credentials, preserving the existing
// receiver ports so connected app services keep their OTLP endpoint.
func updateDynatraceCollector(configPath string, runningProcs []otelProcessInfo, envURL, token, platformTok string, dryRun bool) error {
	startTime := time.Now()
	apiURL := installer.APIURL(envURL)

	origData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	// Preserve existing ports so connected app services keep their OTLP endpoint.
	grpcPort, httpPort, metricsPort, healthCheckPort := portsFromConfig(configPath)
	logger.Debug("existing collector ports", "grpc", grpcPort, "http", httpPort, "metrics", metricsPort, "health_check", healthCheckPort)

	cfgData := otelConfigData{
		Endpoint:    strings.TrimRight(apiURL, "/"),
		AuthHeader:  installer.AuthHeader(token),
		GRPCPort:    grpcPort,
		HTTPPort:    httpPort,
		MetricsPort: metricsPort,
	}
	if featureflags.IsEnabled(featureflags.Experimental) {
		cfgData.HostMonitoring = true
		cfgData.IncludeJournald = runtime.GOOS == "linux"
		cfgData.HealthCheckPort = healthCheckPort
	}

	freshConfig, err := renderOtelTemplate(cfgData)
	if err != nil {
		return fmt.Errorf("generating fresh collector config: %w", err)
	}
	// The template uses "host_metrics" (underscore), but older Dynatrace OTel Collector
	// versions register the factory as "hostmetrics". Preserve whichever name the
	// existing config uses so the diff stays focused on what actually changed.
	if bytes.Contains(origData, []byte("hostmetrics/")) && !bytes.Contains(origData, []byte("host_metrics/")) {
		freshConfig = strings.ReplaceAll(freshConfig, "host_metrics/", "hostmetrics/")
	}
	updatedData := []byte(freshConfig)

	display.Header(fmt.Sprintf("Recreating collector configuration in %s:", configPath))
	fmt.Println()

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
