package otel

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// gatewayExporterKey is the exporter name added to a non-Dynatrace collector's
// config to forward a copy of its data to the dtwiz-managed Dynatrace Gateway
// Collector. Unlike the direct-merge path's "otlp_http/dynatrace" exporter,
// this one carries no auth header or Dynatrace-specific processing: all of
// that lives on the gateway collector, which the foreign config never needs
// to know about.
const gatewayExporterKey = "otlp/dt-gateway"

// docsFallbackURL is shown when dtwiz cannot safely auto-patch a non-Dynatrace
// collector's config, so the user can wire up forwarding and host monitoring
// by hand.
const docsFallbackURL = "https://docs.dynatrace.com/docs/observe/infrastructure-observability/extensions/opentelemetry-host-monitoring"

// ---------------------------------------------------------------------------
// Config-source validation (design Decision 3 / spec "Config source must be a
// single writable file before any change is applied")
// ---------------------------------------------------------------------------

// configSourceResult is the outcome of validating that a collector's effective
// config resolves to a single, writable, local YAML file dtwiz can safely patch.
type configSourceResult struct {
	OK     bool
	Reason string
}

// validateConfigSource checks whether inst's config can be safely auto-patched.
// It fails closed: any ambiguity (multiple --config flags, an inline env:/yaml:
// provider, a container config with no host-mounted path, or an unwritable
// file) is treated as "not a single writable file", per design Decision 3.
func validateConfigSource(inst collectorInstance) configSourceResult {
	if inst.containerRuntime != "" && inst.configPath == "" {
		return configSourceResult{
			OK: false,
			Reason: "the collector's config appears to be baked into its container image " +
				"with no host-mounted path — a live edit would be silently discarded on the next image update",
		}
	}

	if inst.configPath == "" {
		return configSourceResult{OK: false, Reason: "could not determine the collector's config file path"}
	}

	if looksLikeInlineProvider(inst.configPath) {
		return configSourceResult{
			OK:     false,
			Reason: fmt.Sprintf("the collector's config is provided inline (%s), not a file dtwiz can patch", inst.configPath),
		}
	}

	// Only native (non-container) processes have a full command line to inspect
	// for multiple --config flags; a host-mounted container path was already
	// resolved to a single file by container detection.
	if inst.containerRuntime == "" && inst.pid > 0 {
		if n := countConfigFlags(processFullArgs(inst.pid)); n > 1 {
			return configSourceResult{
				OK: false,
				Reason: fmt.Sprintf(
					"the collector was started with %d --config flags (merged configs) — not a single file dtwiz can safely patch", n),
			}
		}
	}

	info, err := os.Stat(inst.configPath)
	if err != nil {
		return configSourceResult{OK: false, Reason: fmt.Sprintf("config path %s is not accessible: %v", inst.configPath, err)}
	}
	if info.IsDir() {
		return configSourceResult{OK: false, Reason: fmt.Sprintf("%s is a directory, not a config file", inst.configPath)}
	}
	if !isWritableFile(inst.configPath) {
		return configSourceResult{OK: false, Reason: fmt.Sprintf("config file %s is not writable", inst.configPath)}
	}

	return configSourceResult{OK: true}
}

// countConfigFlags counts --config/-c occurrences in a process command line.
func countConfigFlags(args string) int {
	count := 0
	fields := splitArgs(args)
	for _, f := range fields {
		if f == "--config" || f == "-c" || strings.HasPrefix(f, "--config=") {
			count++
		}
	}
	return count
}

// looksLikeInlineProvider reports whether a resolved config value is an OTel
// Collector confmap provider URI (e.g. "env:MY_CONFIG", "yaml:...") rather
// than a filesystem path.
func looksLikeInlineProvider(configVal string) bool {
	return strings.HasPrefix(configVal, "env:") || strings.HasPrefix(configVal, "yaml:")
}

// isWritableFile reports whether path can be opened for writing.
func isWritableFile(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// printConfigSourceFallback tells the user dtwiz cannot safely auto-patch this
// collector and points them at manual instructions. No filesystem or process
// changes have been made at this point.
func printConfigSourceFallback(reason string) {
	fmt.Println()
	display.ColorDefault.Printf("  Could not safely update this collector automatically: %s\n", reason)
	fmt.Println()
	display.ColorMessage.Println("  Add a Dynatrace OTLP exporter to your collector's config yourself, and enable")
	display.ColorMessage.Println("  host monitoring using the reference configuration described here:")
	fmt.Printf("    %s\n", docsFallbackURL)
}

// ---------------------------------------------------------------------------
// Foreign-config patch: exactly one additive, forwarding-only exporter
// (design Decision 2 / spec "Foreign collector config receives only an
// additive forwarding exporter")
// ---------------------------------------------------------------------------

// buildGatewayExporterDef returns the map[string]interface{} definition for
// the forwarding-only exporter. No auth header is included — the gateway
// collector owns all Dynatrace-specific credentials and processing.
// block_on_overflow is false so a slow/unreachable gateway can never
// backpressure the foreign collector's original exporter(s).
func buildGatewayExporterDef(gatewayPort int) map[string]interface{} {
	return map[string]interface{}{
		"endpoint": fmt.Sprintf("localhost:%d", gatewayPort),
		"tls": map[string]interface{}{
			"insecure": true,
		},
		"sending_queue": map[string]interface{}{
			"block_on_overflow": false,
		},
	}
}

// mergeGatewayExporter is the gateway-forwarding analogue of
// mergeDynatraceExporter (update.go): it adds exactly one exporter and
// appends its name to every existing pipeline's exporters list, using the
// same unmarshal-mutate-remarshal technique (a plain map[string]interface{},
// not a node-level edit — comments/order/flow-style are not preserved, same
// as the existing direct-merge path). No receiver, processor, or pipeline is
// ever added or removed. Returns true when the exporter was already present
// (idempotent no-op on repeat runs).
func mergeGatewayExporter(cfg map[string]interface{}, gatewayPort int) bool {
	exporters, ok := cfg["exporters"].(map[string]interface{})
	if !ok {
		exporters = make(map[string]interface{})
		cfg["exporters"] = exporters
	}

	if _, exists := exporters[gatewayExporterKey]; exists {
		logger.Debug("gateway forwarding exporter already present, skipping patch")
		return true
	}

	exporters[gatewayExporterKey] = buildGatewayExporterDef(gatewayPort)

	service, ok := cfg["service"].(map[string]interface{})
	if !ok {
		return false
	}
	pipelines, ok := service["pipelines"].(map[string]interface{})
	if !ok {
		return false
	}
	for pipelineName, pipelineVal := range pipelines {
		pipeline, ok := pipelineVal.(map[string]interface{})
		if !ok {
			continue
		}
		existing, _ := pipeline["exporters"].([]interface{})
		alreadyPresent := false
		for _, e := range existing {
			if e == gatewayExporterKey {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			logger.Debug("adding gateway forwarding exporter to pipeline", "pipeline", pipelineName)
			pipeline["exporters"] = append(existing, gatewayExporterKey)
			pipelines[pipelineName] = pipeline
		}
	}
	return false
}

// patchForeignConfigForForwarding unmarshals data, merges in the gateway
// forwarding exporter, and returns the re-marshalled YAML. alreadyPresent is
// true when no change was needed.
func patchForeignConfigForForwarding(data []byte, gatewayPort int) (updated []byte, alreadyPresent bool, err error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("parsing YAML: %w", err)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	if mergeGatewayExporter(cfg, gatewayPort) {
		return data, true, nil
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, false, fmt.Errorf("marshalling patched YAML: %w", err)
	}
	return out, false, nil
}

// ---------------------------------------------------------------------------
// Dynatrace Gateway Collector deployment (design Decision 1)
// ---------------------------------------------------------------------------

// otelGatewayInstallDir returns the directory where the dtwiz-managed
// Dynatrace Gateway Collector is installed — distinct from the app-monitoring
// collector's directory (otelCollectorInstallDir) so the two never collide or
// interfere with each other's lifecycle.
func otelGatewayInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting user home directory: %w", err)
	}
	return filepath.Join(home, "opentelemetry-gateway"), nil
}

// findRunningGatewayCollector returns the running gateway collector instance,
// if any, identified by its install directory rather than by process name
// alone — so it is never confused with dtwiz's app-monitoring collector, or a
// user's own Dynatrace collector running from a different directory.
func findRunningGatewayCollector(gatewayDir string) *collectorInstance {
	prefix := gatewayDir + string(filepath.Separator)
	for _, inst := range findAllRunningOtelCollectorsFunc() {
		if inst.pid > 0 && inst.binaryPath != "" && strings.HasPrefix(inst.binaryPath, prefix) {
			return &inst
		}
	}
	return nil
}

// parseOtlpGRPCPort extracts the OTLP gRPC receiver port from a rendered
// collector config.
func parseOtlpGRPCPort(data []byte) (int, error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return 0, fmt.Errorf("parsing config: %w", err)
	}
	receivers, _ := cfg["receivers"].(map[string]interface{})
	otlp, _ := receivers["otlp"].(map[string]interface{})
	protocols, _ := otlp["protocols"].(map[string]interface{})
	grpcCfg, _ := protocols["grpc"].(map[string]interface{})
	endpoint, _ := grpcCfg["endpoint"].(string)
	idx := strings.LastIndex(endpoint, ":")
	if idx < 0 {
		return 0, fmt.Errorf("could not find an otlp gRPC receiver endpoint in the config")
	}
	port, err := strconv.Atoi(endpoint[idx+1:])
	if err != nil {
		return 0, fmt.Errorf("could not parse port from %q: %w", endpoint, err)
	}
	return port, nil
}

// gatewayPlan holds the pre-computed state for deploying, or reusing, the
// dtwiz-managed Dynatrace Gateway Collector.
type gatewayPlan struct {
	installDir     string
	configPath     string
	binaryPath     string
	configContent  string
	configPreview  string
	receiverPort   int
	alreadyRunning bool
}

// prepareGatewayPlan builds a deployment plan for the Dynatrace Gateway
// Collector. If a gateway collector is already running from its own install
// directory, its existing receiver port is reused and no redeploy is
// planned — an idempotent re-run of the update flow never spawns a second
// gateway instance.
func prepareGatewayPlan(envURL, token string) (*gatewayPlan, error) {
	gatewayDir, err := otelGatewayInstallDir()
	if err != nil {
		return nil, err
	}

	if existing := findRunningGatewayCollector(gatewayDir); existing != nil {
		if data, err := os.ReadFile(existing.configPath); err == nil {
			if port, err := parseOtlpGRPCPort(data); err == nil {
				logger.Debug("gateway collector already running, reusing", "port", port)
				return &gatewayPlan{
					installDir:     gatewayDir,
					configPath:     existing.configPath,
					binaryPath:     existing.binaryPath,
					receiverPort:   port,
					alreadyRunning: true,
				}, nil
			}
		}
		logger.Debug("gateway collector process detected but its config/port could not be read, redeploying")
	}

	apiURL := installer.APIURL(envURL)
	configContent, err := generateOtelConfig(apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("generating gateway collector config: %w", err)
	}
	port, err := parseOtlpGRPCPort([]byte(configContent))
	if err != nil {
		return nil, fmt.Errorf("resolving gateway receiver port: %w", err)
	}

	return &gatewayPlan{
		installDir:    gatewayDir,
		configPath:    filepath.Join(gatewayDir, "config.yaml"),
		binaryPath:    filepath.Join(gatewayDir, otelCollectorBinaryName()),
		configContent: configContent,
		configPreview: installer.MaskSecret(configContent, token),
		receiverPort:  port,
	}, nil
}

// deploy downloads the collector binary, writes its config, and starts it as
// a background process. Unlike collectorPlan.execute (used for the
// app-monitoring collector), this never stops any other running collector:
// gp is only built for a fresh deploy when no gateway is already running from
// its own directory (see prepareGatewayPlan), so there is nothing else here
// competing for the app-monitoring collector's process or ports.
func (gp *gatewayPlan) deploy() (<-chan error, error) {
	if err := os.MkdirAll(gp.installDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating gateway install directory: %w", err)
	}
	binaryPath, err := downloadOtelCollector(gp.installDir)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(gp.configPath, []byte(gp.configContent), 0o600); err != nil {
		return nil, fmt.Errorf("writing gateway config: %w", err)
	}
	fmt.Printf("  Gateway config written to: %s\n", gp.configPath)

	crashed, err := startOtelCollector(binaryPath, gp.configPath)
	if err != nil {
		return nil, err
	}
	gp.binaryPath = binaryPath
	return crashed, nil
}

// waitForPortOpen polls a local TCP port until it accepts connections or the
// timeout elapses. Unlike waitForOtelCollectorReady (hardcoded to port 4318
// for the standalone app-monitoring install, where a port conflict is
// unlikely), the gateway collector's receiver port is dynamically probed and
// frequently shares a host with another collector, so the actual configured
// port must be checked, not a fixed default.
func waitForPortOpen(port int, timeout time.Duration, crashed <-chan error) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case crashErr := <-crashed:
			if crashErr != nil {
				return fmt.Errorf("collector process exited unexpectedly: %w", crashErr)
			}
			return fmt.Errorf("collector process exited unexpectedly")
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("collector did not open port %d within %s: %w", port, timeout, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ---------------------------------------------------------------------------
// Supervisor detection and restart (design Decision 4 / spec "Restart is
// attempted only when it can be done safely")
// ---------------------------------------------------------------------------

// restartCapability classifies how (or whether) dtwiz can safely restart a
// foreign collector after patching its config.
type restartCapability int

const (
	restartUnavailable restartCapability = iota
	restartViaSystemd
	restartViaContainer
	restartViaBareProcess
)

// supervisorInfo describes the detected process supervisor for a foreign
// collector and what restart action (if any) dtwiz can safely take.
type supervisorInfo struct {
	capability  restartCapability
	systemdUnit string
	note        string // human-readable reason, shown when capability is restartUnavailable
}

// launchContext holds a bare/manual process's full original invocation,
// captured so it can be faithfully relaunched. complete is false when any
// part (argv, env, or cwd) could not be captured with confidence — per design
// Decision 4, an incomplete capture is always treated as "cannot restart
// automatically" rather than risking a lossy relaunch.
type launchContext struct {
	argv     []string
	env      []string
	cwd      string
	complete bool
}

// detectSupervisor determines whether dtwiz can safely restart inst
// automatically, and by what mechanism. Container and systemd cases are
// resolved with high confidence; anything else (Kubernetes pod, ambiguous
// cgroup, or an incomplete launch-context capture) is conservatively treated
// as unavailable, per design Decision 4.
func detectSupervisor(inst collectorInstance) (supervisorInfo, *launchContext) {
	if inst.containerRuntime != "" {
		return supervisorInfo{capability: restartViaContainer}, nil
	}

	if inst.pid <= 0 {
		return supervisorInfo{capability: restartUnavailable, note: "collector is not currently running"}, nil
	}

	if isKubernetesPod(inst.pid) {
		return supervisorInfo{
			capability: restartUnavailable,
			note:       "collector appears to run inside a Kubernetes pod — not supported for automatic restart in this version",
		}, nil
	}

	if unit, ok := detectSystemdUnit(inst.pid); ok {
		return supervisorInfo{capability: restartViaSystemd, systemdUnit: unit}, nil
	}

	lc, ok := captureLaunchContext(inst.pid)
	if !ok {
		return supervisorInfo{
			capability: restartUnavailable,
			note:       "could not capture the process's full launch context (arguments, environment, working directory) with confidence",
		}, nil
	}
	return supervisorInfo{capability: restartViaBareProcess}, lc
}

// restartForeignCollector restarts inst using the mechanism identified by
// sup. It never kills a process directly when a supervisor owns it — systemd
// and container restarts go through their own tooling; only the bare/manual
// case (no external supervisor) has dtwiz kill and relaunch the process
// itself, using the faithfully captured launch context rather than a
// synthesized command line.
func restartForeignCollector(inst collectorInstance, sup supervisorInfo, lc *launchContext) error {
	switch sup.capability {
	case restartViaSystemd:
		return restartViaSystemctl(sup.systemdUnit)
	case restartViaContainer:
		return restartContainerAndVerify(inst.containerRuntime, inst.containerName)
	case restartViaBareProcess:
		if lc == nil || !lc.complete {
			return fmt.Errorf("internal error: bare-process restart requested without a complete launch context")
		}
		proc, err := os.FindProcess(inst.pid)
		if err == nil {
			_ = installer.KillAndWaitProcess(proc)
		}
		return relaunchWithContext(lc)
	default:
		return fmt.Errorf("no restart mechanism available for this collector")
	}
}

// containerRestartTimeout bounds how long to wait for a restarted container
// to report itself running again after restartContainer issues the restart.
const containerRestartTimeout = 15 * time.Second

// restartContainerAndVerify restarts a container via the existing
// restartContainer helper, then polls its running state so a restart whose
// command exits 0 but leaves the container crash-looping (or stopped) is
// reported as a failure, not silently treated as success.
func restartContainerAndVerify(cli, name string) error {
	if err := restartContainer(cli, name); err != nil {
		return err
	}
	deadline := time.Now().Add(containerRestartTimeout)
	var lastState string
	for {
		running, state := containerRunningState(cli, name)
		lastState = state
		if running {
			logger.Debug("restartContainerAndVerify: container running", "container", name)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("container %s did not report running within %s after restart (last state: %s)",
				name, containerRestartTimeout, lastState)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// containerRunningState reports whether the named container is currently
// running, and the raw state string for diagnostics when it is not.
func containerRunningState(cli, name string) (running bool, state string) {
	out, err := exec.Command(cli, "inspect", "--format", "{{.State.Running}}", name).Output()
	if err != nil {
		return false, fmt.Sprintf("inspect failed: %v", err)
	}
	state = strings.TrimSpace(string(out))
	return state == "true", state
}

// ---------------------------------------------------------------------------
// Orchestration (implements the full non-Dynatrace update-otel flow)
// ---------------------------------------------------------------------------

// UpdateNonDynatraceCollector implements the update-otel flow for a selected
// non-Dynatrace collector: deploy a dedicated Dynatrace Gateway Collector
// (with host monitoring), patch the foreign collector with exactly one
// additive forwarding exporter, and restart it when that can be done safely.
// See design.md and specs/otel-gateway-collector-update/spec.md in
// openspec/changes/otel-update-non-dynatrace-gateway/ for the decision
// rationale behind each step.
func UpdateNonDynatraceCollector(envURL, token, platformTok string, inst collectorInstance, dryRun bool) error {
	check := validateConfigSource(inst)
	if !check.OK {
		printConfigSourceFallback(check.Reason)
		return nil
	}

	origData, err := os.ReadFile(inst.configPath)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", inst.configPath, err)
	}

	sup, lc := detectSupervisor(inst)

	gp, err := prepareGatewayPlan(envURL, token)
	if err != nil {
		return fmt.Errorf("preparing Dynatrace Gateway Collector: %w", err)
	}

	updatedData, alreadyForwarding, err := patchForeignConfigForForwarding(origData, gp.receiverPort)
	if err != nil {
		return fmt.Errorf("building forwarding patch: %w", err)
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Gateway Collector")
	if gp.alreadyRunning {
		fmt.Printf("  Already running: %s (receiver port %d)\n", gp.binaryPath, gp.receiverPort)
	} else {
		fmt.Printf("  Install dir:   %s\n", gp.installDir)
		fmt.Printf("  Receiver port: %d (localhost only)\n", gp.receiverPort)
	}

	fmt.Println()
	display.Header(fmt.Sprintf("Preview of changes to %s:", inst.configPath))
	fmt.Println()
	if alreadyForwarding {
		display.ColorMuted.Println("  (forwarding exporter already present — no config change needed)")
	} else {
		showConfigDiff(origData, updatedData)
	}

	fmt.Println()
	display.PrintSectionDivider()
	switch sup.capability {
	case restartViaSystemd:
		fmt.Printf("  Restart plan: systemctl restart %s\n", sup.systemdUnit)
	case restartViaContainer:
		fmt.Printf("  Restart plan: restart container %s (%s)\n", inst.containerName, inst.containerRuntime)
	case restartViaBareProcess:
		fmt.Printf("  Restart plan: relaunch PID %d with its original arguments\n", inst.pid)
	default:
		display.ColorDefault.Printf("  Restart plan: none (%s) — you will need to restart it manually.\n", sup.note)
	}
	fmt.Println()
	display.PrintSectionDivider()

	proceed, err := installer.ShouldProceed(dryRun, "Update")
	if err != nil || !proceed {
		return err
	}

	backupPath, err := backupFile(inst.configPath, origData)
	if err != nil {
		return err
	}
	fmt.Printf("  Backup created: %s\n", backupPath)

	if !gp.alreadyRunning {
		crashed, err := gp.deploy()
		if err != nil {
			return fmt.Errorf("deploying Dynatrace Gateway Collector: %w", err)
		}
		if err := waitForPortOpen(gp.receiverPort, 30*time.Second, crashed); err != nil {
			return fmt.Errorf("gateway collector did not become ready: %w", err)
		}
	}

	if !alreadyForwarding {
		if err := os.WriteFile(inst.configPath, updatedData, 0o600); err != nil {
			return fmt.Errorf("writing updated config to %s: %w", inst.configPath, err)
		}
		fmt.Printf("  Config updated: %s\n", inst.configPath)
	}

	if sup.capability == restartUnavailable {
		printManualRestartInstructions(envURL, inst.configPath, backupPath, sup.note)
		return nil
	}

	if err := restartForeignCollector(inst, sup, lc); err != nil {
		// Restore the pre-patch config so the collector isn't left patched but down.
		if restoreErr := os.WriteFile(inst.configPath, origData, 0o600); restoreErr != nil {
			logger.Debug("failed to restore config backup after restart failure", "err", restoreErr)
		}
		fmt.Printf("\n  Restart failed: %v\n", err)
		printManualRestartInstructions(envURL, inst.configPath, backupPath, "the automatic restart failed")
		return nil
	}

	fmt.Println()
	display.ColorOK.Println("  ✓ Collector restarted.")
	startTime := time.Now().Add(-1 * time.Minute)
	installer.WatchIngestWithStatus(envURL, platformTok, startTime.UTC().Format(installer.IngestTimeFormat), nil)
	return nil
}

// printManualRestartInstructions prints the config and backup paths and links
// to check ingest manually, used whenever dtwiz did not (or could not)
// restart the foreign collector itself.
func printManualRestartInstructions(envURL, configPath, backupPath, reason string) {
	fmt.Println()
	display.ColorDefault.Printf("  Restart the collector manually to pick up the new config (%s).\n", reason)
	fmt.Printf("    Config: %s\n", configPath)
	fmt.Printf("    Backup: %s\n", backupPath)
	fmt.Println()
	display.ColorMessage.Println("  Check ingest once restarted:")
	appsURL := installer.AppsURL(envURL)
	fmt.Printf("    %s\n", termLink("Open Dynatrace Services", appsURL+"/ui/apps/dynatrace.services/explorer-new/services-new"))
	fmt.Printf("    %s\n", termLink("Open Distributed Tracing", appsURL+"/ui/apps/dynatrace.distributedtracing/explorer"))
}
