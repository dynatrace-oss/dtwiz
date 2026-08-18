package otel

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// connectedService is an app process tied to the collector — via TCP connection
// to its OTLP ports or by exporting to the same Dynatrace tenant.
type connectedService struct {
	pid               int
	name              string   // short display name (binary basename)
	command           string   // full command line (whitespace-joined; may misparse space-containing args)
	cmdline           []string // null-delimited argv from /proc/<pid>/cmdline on Linux; nil elsewhere
	workDir           string   // working directory at detection time
	collectorPort     string   // OTLP receiver port this service sends to (e.g. "4317" or "4318")
	listenPorts       []string // TCP ports this process itself listens on (e.g. ["8080", "8001"])
	exportsTo         string   // OTLP export endpoint from the process env, when tenant-matched
	env               []string // full environment ("KEY=VAL"), captured for faithful relaunch
	collectorEndpoint string   // local OTLP HTTP endpoint to route through on restart (e.g. "http://localhost:4320")
}

// receiverPortsFromConfig parses the collector YAML config and returns the
// port numbers configured for the OTLP gRPC and HTTP receivers.  Falls back
// to the standard OTLP defaults (4317 and 4318) when the config cannot be
// parsed or the endpoint fields are absent.
func receiverPortsFromConfig(data []byte) []string {
	defaults := []string{"4317", "4318"}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return defaults
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return defaults
	}
	root := doc.Content[0]

	protocols := nodeGet(root, "receivers", "otlp", "protocols")
	if protocols == nil {
		return defaults
	}

	var ports []string
	for _, proto := range []string{"grpc", "http"} {
		endpointNode := nodeGet(protocols, proto, "endpoint")
		if endpointNode == nil {
			continue
		}
		addr := endpointNode.Value
		if idx := strings.LastIndex(addr, ":"); idx >= 0 {
			if port := addr[idx+1:]; port != "" {
				ports = append(ports, port)
				logger.Debug("receiver port from config", "proto", proto, "port", port)
			}
		}
	}
	if len(ports) == 0 {
		return defaults
	}
	return ports
}

// detectConnectedServices returns app processes associated with the collector:
// those with an active TCP connection to its OTLP ports, OTel-instrumented
// processes exporting to the same tenant (apps that never connect locally),
// and — one level deeper — services connected to any of those apps on their
// own listening ports (so that restarting App A also restarts clients of App A).
// excludePIDs and dtwiz itself are filtered out; results are deduplicated.
func detectConnectedServices(configData []byte, excludePIDs map[int]bool) []connectedService {
	ports := receiverPortsFromConfig(configData)
	tenants := collectorTenantsFromConfig(configData)

	if excludePIDs == nil {
		excludePIDs = map[int]bool{}
	}
	excludePIDs[os.Getpid()] = true

	var result []connectedService
	seen := map[int]bool{}
	add := func(svc connectedService) {
		if excludePIDs[svc.pid] || seen[svc.pid] {
			return
		}
		seen[svc.pid] = true
		result = append(result, svc)
	}

	for _, svc := range detectServicesOnPorts(ports) {
		add(svc)
	}
	for _, svc := range detectInstrumentedServices(tenants, ports) {
		add(svc)
	}

	// Cascade: find services connected to the detected apps on their own
	// listening ports (e.g. App B → App A → collector).  One level only —
	// deeper chains are uncommon and would risk pulling in unrelated processes.
	var cascadePorts []string
	cascadePortSet := map[string]bool{}
	for _, svc := range result {
		for _, p := range svc.listenPorts {
			if !cascadePortSet[p] {
				cascadePortSet[p] = true
				cascadePorts = append(cascadePorts, p)
			}
		}
	}
	if len(cascadePorts) > 0 {
		for _, svc := range detectServicesOnPorts(cascadePorts) {
			add(svc)
		}
	}

	return result
}

// isDynatraceEndpoint reports whether endpoint points to a Dynatrace-hosted
// URL (SaaS or managed). Only these endpoints yield meaningful tenant IDs;
// non-Dynatrace exporters (Jaeger, Tempo, etc.) must be excluded so their
// first DNS label never contaminates the tenant set.
func isDynatraceEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range []string{".dynatrace.com", ".dynatracelabs.com"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	// Managed deployment: path contains /e/<tenantId>
	return strings.Contains(u.Path, "/e/")
}

// collectorTenantsFromConfig returns tenant IDs extracted from every configured exporter endpoint.
func collectorTenantsFromConfig(data []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	exporters := nodeMappingGet(doc.Content[0], "exporters")
	if exporters == nil {
		return nil
	}

	seen := map[string]bool{}
	var tenants []string
	// exporters is a mapping of <name> -> <exporter config>.
	for i := 0; i+1 < len(exporters.Content); i += 2 {
		endpoint := nodeMappingGet(exporters.Content[i+1], "endpoint")
		if endpoint == nil || endpoint.Value == "" {
			continue
		}
		if !isDynatraceEndpoint(endpoint.Value) {
			continue
		}
		tenant := installer.ExtractTenantID(endpoint.Value)
		if tenant != "" && !seen[tenant] {
			seen[tenant] = true
			tenants = append(tenants, tenant)
			logger.Debug("collector export tenant", "endpoint", endpoint.Value, "tenant", tenant)
		}
	}
	return tenants
}

// otlpEnvKeys are the env vars (priority order) that reveal an OTLP endpoint.
// DT_ENVIRONMENT is excluded — dtwiz and its children inherit it, causing false matches.
var otlpEnvKeys = []string{
	"OTEL_EXPORTER_OTLP_ENDPOINT=",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=",
	"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=",
	"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=",
}

// otlpEndpointFromEnv returns the OTLP export endpoint from a ps -eww command+env string, or "".
func otlpEndpointFromEnv(cmdEnv string) string {
	fields := strings.Fields(cmdEnv)
	for _, key := range otlpEnvKeys {
		for _, tok := range fields {
			if strings.HasPrefix(tok, key) {
				if v := strings.TrimPrefix(tok, key); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// endpointMatchesCollector reports whether an endpoint targets the local collector ports or its export tenant.
func endpointMatchesCollector(endpoint string, tenantSet, portSet map[string]bool) bool {
	if host, port := hostPort(endpoint); isLoopback(host) && port != "" && portSet[port] {
		return true
	}
	if tenant := installer.ExtractTenantID(endpoint); tenant != "" && tenantSet[tenant] {
		return true
	}
	return false
}

// hostPort returns the host and port of a URL or host:port string.
func hostPort(endpoint string) (host, port string) {
	s := endpoint
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func isLoopback(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return false
}

// otlpSignalEndpointKeys are per-signal overrides dropped on reconciliation to prevent stale-tenant routing.
var otlpSignalEndpointKeys = []string{
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
	"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
}

// reconcileExportEnv updates OTEL_EXPORTER_OTLP_ENDPOINT and the Authorization
// header to match DT_ENVIRONMENT + DT_PLATFORM_TOKEN, preventing stale OTLP
// vars from routing to an old tenant.  No-op when DT_ENVIRONMENT is absent,
// already consistent, or the endpoint is loopback (collector handles routing).
func reconcileExportEnv(env []string) (out []string) {
	dtEnv := envGet(env, "DT_ENVIRONMENT")
	if dtEnv == "" {
		return env
	}

	current := envGet(env, "OTEL_EXPORTER_OTLP_ENDPOINT")
	if current != "" {
		if host, _ := hostPort(current); isLoopback(host) {
			return env // routed via a local collector — leave as-is
		}
	}

	target := strings.TrimRight(installer.APIURL(dtEnv), "/") + "/api/v2/otlp"
	tokenMatches := true
	if tok := envGet(env, "DT_PLATFORM_TOKEN"); tok != "" {
		tokenMatches = headerHasToken(envGet(env, "OTEL_EXPORTER_OTLP_HEADERS"), tok)
	}
	if current == target && tokenMatches {
		return env // already consistent
	}

	out = envSet(env, "OTEL_EXPORTER_OTLP_ENDPOINT", target)
	out = envRemove(out, otlpSignalEndpointKeys...)
	if tok := envGet(env, "DT_PLATFORM_TOKEN"); tok != "" {
		hdr := rebuildAuthHeader(envGet(env, "OTEL_EXPORTER_OTLP_HEADERS"), tok)
		out = envSet(out, "OTEL_EXPORTER_OTLP_HEADERS", hdr)
	}
	return out
}

// retargetEnvToCollector returns env with OTEL_EXPORTER_OTLP_ENDPOINT set to
// the local collector HTTP endpoint and per-signal endpoint overrides removed.
// No-op when the current endpoint is already loopback (already collector-routed).
func retargetEnvToCollector(env []string, endpoint string) (out []string, changed bool) {
	if current := envGet(env, "OTEL_EXPORTER_OTLP_ENDPOINT"); current != "" {
		if host, _ := hostPort(current); isLoopback(host) {
			return env, false
		}
	}
	out = envSet(env, "OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	out = envRemove(out, otlpSignalEndpointKeys...)
	return out, true
}

// envGet returns the value of key in a "KEY=VAL" environment slice, or "".
func envGet(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}

// envSet replaces or appends key=val in a "KEY=VAL" environment slice.
func envSet(env []string, key, val string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			out = append(out, prefix+val)
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, prefix+val)
	}
	return out
}

// envRemove drops the given keys from a "KEY=VAL" environment slice.
func envRemove(env []string, keys ...string) []string {
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		if eq := strings.IndexByte(e, '='); eq > 0 && drop[e[:eq]] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// rebuildAuthHeader replaces the Authorization token in an OTLP headers string,
// preserving the auth scheme (Api-Token/Bearer) and other headers; adds one if absent.
func rebuildAuthHeader(existing, token string) string {
	parts := strings.Split(existing, ",")
	found := false
	for i, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 && strings.EqualFold(strings.TrimSpace(kv[0]), "Authorization") {
			parts[i] = "Authorization=" + authScheme(kv[1]) + "%20" + token
			found = true
		}
	}
	if !found {
		if strings.TrimSpace(existing) == "" {
			return "Authorization=Api-Token%20" + token
		}
		parts = append(parts, "Authorization=Api-Token%20"+token)
	}
	return strings.Join(parts, ",")
}

// authScheme returns the scheme prefix ("Api-Token", "Bearer", etc.) from an Authorization header value.
func authScheme(headerVal string) string {
	for _, sep := range []string{"%20", " "} {
		if i := strings.Index(headerVal, sep); i > 0 {
			return headerVal[:i]
		}
	}
	return "Api-Token"
}

// headerHasToken reports whether the Authorization header already carries token.
func headerHasToken(headerVal, token string) bool {
	return token != "" && strings.Contains(headerVal, token)
}

// stripEnvSuffix removes the env block ps -eww appends after arguments, returning
// only the command.  Also prevents env var secrets from appearing in output.
func stripEnvSuffix(cmdEnv string) string {
	fields := strings.Fields(cmdEnv)
	for i, tok := range fields {
		if eq := strings.IndexByte(tok, '='); eq > 0 && isEnvKey(tok[:eq]) {
			return strings.Join(fields[:i], " ")
		}
	}
	return cmdEnv
}

// envSuffix returns the env block from a ps -eww command+env string as "KEY=VAL"
// strings for exec.Cmd.Env.  Space-containing values may split incorrectly.
func envSuffix(cmdEnv string) []string {
	fields := strings.Fields(cmdEnv)
	for i, tok := range fields {
		if eq := strings.IndexByte(tok, '='); eq > 0 && isEnvKey(tok[:eq]) {
			return fields[i:]
		}
	}
	return nil
}

// isEnvKey reports whether s is a valid UPPER_SNAKE env variable name.
func isEnvKey(s string) bool {
	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return s != ""
}

// serviceDisplayName returns a short human-readable label for a process given
// its full command line.
func serviceDisplayName(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "(unknown)"
	}
	name := filepath.Base(fields[0])
	// For interpreter-based services (node, python, java), append the first
	// non-flag argument so the display name is more informative.
	if isInterpreter(name) && len(fields) > 1 {
		for _, arg := range fields[1:] {
			if !strings.HasPrefix(arg, "-") {
				return name + " " + scriptLabel(arg)
			}
		}
	}
	return name
}

// genericScriptNames are ambiguous entrypoint filenames; parent dir is prepended for context (e.g. "delivery/app.py").
var genericScriptNames = map[string]bool{
	"app.py": true, "main.py": true, "__main__.py": true, "server.py": true, "run.py": true,
	"index.js": true, "server.js": true, "main.js": true, "app.js": true,
	"main.go": true, "index.ts": true, "server.ts": true,
}

// scriptLabel returns a script display label, prepending the parent dir for generic filenames.
func scriptLabel(arg string) string {
	base := filepath.Base(arg)
	if genericScriptNames[base] {
		if parent := filepath.Base(filepath.Dir(arg)); parent != "." && parent != "/" && parent != "" {
			return parent + "/" + base
		}
	}
	return base
}

func isInterpreter(name string) bool {
	lower := strings.ToLower(name)
	for _, interp := range []string{"node", "python", "python3", "java", "ruby", "perl"} {
		if lower == interp || strings.HasPrefix(lower, interp+".") {
			return true
		}
	}
	return false
}

// printConnectedServices prints the list of detected connected services.
func printConnectedServices(svcs []connectedService) {
	display.ColorBold.Printf("  Connected services (%d):\n", len(svcs))
	for _, svc := range svcs {
		fmt.Printf("    • PID %-6d  %s\n", svc.pid, display.ColorDefault.Sprint(svc.name))
		if len(svc.listenPorts) > 0 {
			display.ColorMuted.Printf("              listening on: %s\n", strings.Join(svc.listenPorts, ", "))
		}
		if svc.exportsTo != "" {
			display.ColorMuted.Printf("              exports to: %s\n", svc.exportsTo)
		}
	}
}

// waitForListenPorts polls detectListenPorts up to maxAttempts times at interval
// until the process has bound at least one port, then returns the result.
func waitForListenPorts(pid, maxAttempts int, interval time.Duration) []string {
	for i := 0; i < maxAttempts; i++ {
		if ports := detectListenPorts(pid); len(ports) > 0 {
			return ports
		}
		if i < maxAttempts-1 {
			time.Sleep(interval)
		}
	}
	return nil
}

// restartConnectedServices stops each service (SIGTERM→SIGKILL) and relaunches
// it detached with its original command, workdir, and reconciled environment.
func restartConnectedServices(svcs []connectedService) {
	if len(svcs) == 0 {
		return
	}

	display.ColorBold.Printf("  Restarting %d connected service(s):\n", len(svcs))
	var relaunchFailed bool
	for _, svc := range svcs {
		fmt.Printf("    • PID %-6d  %s  ", svc.pid, svc.name)

		if err := stopService(svc.pid); err != nil {
			fmt.Println(display.ColorError.Sprint("could not stop: " + err.Error()))
			logger.Debug("stopService failed", "pid", svc.pid, "name", svc.name, "err", err)
			continue
		}

		// Route through local collector when requested; otherwise reconcile the
		// export tenant with DT_ENVIRONMENT so a stale endpoint is corrected.
		// On Windows the process environment cannot be read, so fall back to the
		// current process environment as the base — this gives the relaunched
		// service a complete environment (PATH, etc.) and lets retargetEnvToCollector
		// correctly override OTEL_EXPORTER_OTLP_ENDPOINT to the local collector.
		envToUse := svc.env
		if envToUse == nil {
			envToUse = os.Environ()
		}
		if svc.collectorEndpoint != "" {
			envToUse, _ = retargetEnvToCollector(envToUse, svc.collectorEndpoint)
		}
		newEnv := reconcileExportEnv(envToUse)
		svc.env = newEnv

		newPID, err := relaunchService(svc)
		if err != nil {
			relaunchFailed = true
			fmt.Println(display.ColorError.Sprint("stopped, but restart failed: " + err.Error()))
			logger.Debug("relaunchService failed", "pid", svc.pid, "name", svc.name, "err", err)
			continue
		}

		// Detect ports the new process is actually listening on.
		newPorts := waitForListenPorts(newPID, 5, 200*time.Millisecond)
		portLabel := ""
		if len(newPorts) > 0 {
			portLabel = display.ColorMuted.Sprintf(" (ports: %s)", strings.Join(newPorts, ", "))
		}
		fmt.Println(display.ColorOK.Sprintf("restarted (PID %d)", newPID) + portLabel)
	}

	if relaunchFailed {
		fmt.Println()
		display.ColorDefault.Println("  Some services could not be restarted automatically — start them manually.")
	}
}
