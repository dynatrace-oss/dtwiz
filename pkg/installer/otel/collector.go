package otel

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

//go:embed otel.tmpl
var otelConfigTemplateText string

// otelConfigData holds the values substituted into otel.tmpl.
type otelConfigData struct {
	Endpoint        string
	AuthHeader      string
	MetricsPort     int
	GRPCPort        int
	HTTPPort        int
	HostMonitoring  bool
	IncludeJournald bool
	HealthCheckPort int
}

// findFreePort returns the lowest port >= startPort on which localhost is not
// already listening.  Falls back to startPort if no free port is found within
// 100 attempts (avoids an infinite loop on pathological systems).
func findFreePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		addr := fmt.Sprintf("localhost:%d", port)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		l.Close()
		return port
	}
	return startPort
}

// otelCollectorBinaryName returns the expected binary name inside the release archive.
func otelCollectorBinaryName() string {
	if runtime.GOOS == "windows" {
		return "dynatrace-otel-collector.exe"
	}
	return "dynatrace-otel-collector"
}

// otelCollectorInstallDir returns the directory where the OTel Collector is installed.
// Uses the user's home directory on all platforms to avoid permission issues.
func otelCollectorInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting user home directory: %w", err)
	}
	return filepath.Join(home, "opentelemetry"), nil
}

// otelLatestReleaseVersion resolves the latest release tag (e.g. "v0.44.0")
// for the Dynatrace OTel Collector by following the /releases/latest redirect
// on github.com. This avoids the GitHub REST API entirely, sidestepping the
// 60 req/hour unauthenticated rate limit that causes 403 responses.
func otelLatestReleaseVersion(ctx context.Context) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow — we want the Location header
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://github.com/Dynatrace/dynatrace-otel-collector/releases/latest", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching latest release redirect: %w", err)
	}
	resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("GitHub releases/latest returned no redirect (status %d)", resp.StatusCode)
	}

	// Location is e.g. https://github.com/.../releases/tag/v0.44.0
	tag := loc[strings.LastIndex(loc, "/")+1:]
	if tag == "" || !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("unexpected redirect location: %s", loc)
	}
	logger.Debug("resolved latest OTel Collector release", "location", loc, "tag", tag)
	return tag, nil
}

// otelPlatformAssetName returns the versioned GitHub release asset filename for
// the current OS/architecture combination.
// Asset naming: dynatrace-otel-collector_{version}_{OS}_{arch}[.tar.gz|.zip]
// e.g. dynatrace-otel-collector_0.44.0_Darwin_arm64.tar.gz
func otelPlatformAssetName(version string) (string, error) {
	// Strip leading 'v' from tag (v0.44.0 → 0.44.0).
	ver := strings.TrimPrefix(version, "v")

	var osName, archName string
	switch runtime.GOOS {
	case "linux":
		osName = "Linux"
	case "darwin":
		osName = "Darwin"
	case "windows":
		osName = "Windows"
	default:
		return "", fmt.Errorf("unsupported OS for OTel Collector: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture for OTel Collector: %s", runtime.GOARCH)
	}

	if runtime.GOOS == "windows" {
		return fmt.Sprintf("dynatrace-otel-collector_%s_%s_%s.zip", ver, osName, archName), nil
	}
	return fmt.Sprintf("dynatrace-otel-collector_%s_%s_%s.tar.gz", ver, osName, archName), nil
}

// otelReleaseURL returns the download URL for a specific versioned release asset.
func otelReleaseURL(version, assetName string) string {
	return fmt.Sprintf(
		"https://github.com/Dynatrace/dynatrace-otel-collector/releases/download/%s/%s",
		version, assetName,
	)
}

// downloadOtelCollector downloads and extracts the OTel Collector binary to
// the specified destination path.
func downloadOtelCollector(destDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Printf("  Resolving latest Dynatrace OTel Collector release...\n")
	version, err := otelLatestReleaseVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving latest release version: %w", err)
	}

	assetName, err := otelPlatformAssetName(version)
	if err != nil {
		return "", err
	}

	downloadURL := otelReleaseURL(version, assetName)
	logger.Debug("downloading OTel Collector", "version", version, "asset", assetName, "url", downloadURL)
	fmt.Printf("  Downloading Dynatrace OTel Collector %s from GitHub...\n", version)
	fmt.Printf("  URL: %s\n", downloadURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("building download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading OTel Collector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OTel Collector download returned status %d", resp.StatusCode)
	}

	// Save archive to temp file.
	tmpArchive, err := os.CreateTemp("", "dt-otel-collector-*")
	if err != nil {
		return "", fmt.Errorf("creating temp archive file: %w", err)
	}
	tmpArchiveName := tmpArchive.Name()
	defer os.Remove(tmpArchiveName)
	logger.Debug("archive temp file created", "path", tmpArchiveName)

	if _, err := io.Copy(tmpArchive, resp.Body); err != nil {
		tmpArchive.Close()
		return "", fmt.Errorf("writing archive to disk: %w", err)
	}
	tmpArchive.Close()

	// Extract binary from archive.
	binaryName := otelCollectorBinaryName()
	destPath := filepath.Join(destDir, binaryName)

	if strings.HasSuffix(assetName, ".zip") {
		if err := extractFromZip(tmpArchiveName, binaryName, destPath); err != nil {
			return "", fmt.Errorf("extracting from zip: %w", err)
		}
	} else {
		if err := extractFromTarGz(tmpArchiveName, binaryName, destPath); err != nil {
			return "", fmt.Errorf("extracting from tar.gz: %w", err)
		}
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0o755); err != nil {
			return "", fmt.Errorf("setting OTel Collector executable bit: %w", err)
		}
	}

	// On macOS, unsigned binaries downloaded from the internet are silently
	// killed by the system before they can produce any output.  Strip all
	// extended attributes (incl. quarantine) and apply an ad-hoc signature so
	// the OS allows the binary to run.
	if runtime.GOOS == "darwin" {
		if err := macOSPrepBinary(destPath); err != nil {
			return "", err
		}
	}

	return destPath, nil
}

// macOSPrepBinary removes quarantine/extended attributes and applies an ad-hoc
// code signature so macOS allows the binary to execute.
func macOSPrepBinary(binaryPath string) error {
	fmt.Println("  Preparing binary for macOS (removing quarantine, applying ad-hoc signature)...")

	if out, err := exec.Command("xattr", "-cr", binaryPath).CombinedOutput(); err != nil {
		return fmt.Errorf("xattr -cr failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	if _, err := exec.LookPath("codesign"); err == nil {
		if out, err := exec.Command("codesign", "--force", "--deep", "--sign", "-", binaryPath).CombinedOutput(); err != nil {
			// Non-fatal: log the warning but continue — the binary may still work.
			fmt.Printf("  Warning: ad-hoc codesign failed (may still work): %v\n%s\n",
				err, strings.TrimSpace(string(out)))
		} else {
			fmt.Println("  Ad-hoc signature applied.")
		}
	}
	return nil
}

// extractFromTarGz extracts a single file by name from a .tar.gz archive.
func extractFromTarGz(archivePath, targetName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if filepath.Base(hdr.Name) == targetName {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		}
	}
	return fmt.Errorf("binary %q not found in archive", targetName)
}

// extractFromZip extracts a single file by name from a .zip archive.
func extractFromZip(archivePath, targetName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == targetName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		}
	}
	return fmt.Errorf("binary %q not found in zip archive", targetName)
}

// sendOtelVerificationLog sends a single OTLP log record to the local collector
// (HTTP on 4318) with the given body text and returns the unique install ID
// embedded in the message so the caller can search for it.
// Retries on connection reset/refused: the TCP port can accept connections before
// the HTTP handler is fully initialized, causing a RST on the first request.
func sendOtelVerificationLog(body string, httpPort int) error {
	hostname, _ := os.Hostname()

	payload := map[string]interface{}{
		"resourceLogs": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]interface{}{
						{"key": "service.name", "value": map[string]string{"stringValue": "dtwiz"}},
						{"key": "host.name", "value": map[string]string{"stringValue": hostname}},
						{"key": "os.type", "value": map[string]string{"stringValue": runtime.GOOS}},
						{"key": "host.arch", "value": map[string]string{"stringValue": runtime.GOARCH}},
					},
				},
				"scopeLogs": []map[string]interface{}{
					{
						"scope": map[string]string{"name": "dtwiz.installer"},
						"logRecords": []map[string]interface{}{
							{
								"timeUnixNano":   fmt.Sprintf("%d", time.Now().UnixNano()),
								"severityText":   "INFO",
								"severityNumber": 9,
								"body":           map[string]string{"stringValue": body},
								"attributes": []map[string]interface{}{
									{"key": "dtwiz.version", "value": map[string]string{"stringValue": "1.0"}},
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling OTLP payload: %w", err)
	}

	const maxAttempts = 5
	for attempt := range maxAttempts {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/v1/logs", httpPort), "application/json", bytes.NewReader(data))
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "connection reset") || strings.Contains(errStr, "connection refused") {
				logger.Debug("sendOtelVerificationLog: transient error, retrying", "attempt", attempt+1, "err", err)
				continue
			}
			return fmt.Errorf("sending OTLP log: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("OTLP endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil
	}
	return fmt.Errorf("sending OTLP log: collector not ready after %d attempts", maxAttempts)
}

// waitForLogInDynatraceFn is overridable in tests to avoid real DQL polling.
var waitForLogInDynatraceFn = waitForLogInDynatrace

// waitForLogInDynatrace queries the Dynatrace Grail DQL API directly until a
// log record containing searchTerm appears, or until the timeout elapses.
//
// The DQL endpoint lives on the .apps. URL variant:
//
//	POST https://<env>.apps.<domain>/platform/storage/query/v1/query:execute
func waitForLogInDynatrace(envURL, token, searchTerm string, timeout time.Duration) error {
	appsBase := strings.TrimRight(installer.AppsURL(envURL), "/")
	queryURL := appsBase + "/platform/storage/query/v1/query:execute"

	dqlQuery := fmt.Sprintf(
		`fetch logs, from: now()-1m | filter contains(content, "%s") | limit 1`,
		searchTerm,
	)

	deadline := time.Now().Add(timeout)
	var lastErr string
	for {
		payload, _ := json.Marshal(map[string]interface{}{
			"query":                      dqlQuery,
			"requestTimeoutMilliseconds": 8000,
			"maxResultRecords":           1,
		})

		req, err := http.NewRequest(http.MethodPost, queryURL, bytes.NewReader(payload))
		if err != nil {
			lastErr = err.Error()
		} else {
			req.Header.Set("Content-Type", "application/json")
			// The Grail DQL endpoint always requires Bearer auth, regardless
			// of token type (dt0c01.*, dt0s16.*, OAuth).
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lastErr = err.Error()
			} else {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				switch {
				case resp.StatusCode/100 == 2:
					// Parse JSON to check for actual log records, matching the
					// Python implementation: data["result"]["records"] non-empty.
					var data struct {
						Result struct {
							Records []json.RawMessage `json:"records"`
						} `json:"result"`
					}
					if json.Unmarshal(body, &data) == nil && len(data.Result.Records) > 0 {
						return nil
					}
					// 2xx but no records yet — continue polling.
					lastErr = ""
				case resp.StatusCode == 401 || resp.StatusCode == 403:
					// Show token prefix so the user can verify they passed the right one.
					tokenHint := token
					if len(tokenHint) > 20 {
						tokenHint = tokenHint[:20] + "..."
					}
					return fmt.Errorf(
						"DQL query returned %d — the token may lack the required scopes\n\n"+
							"  Ensure the token has scope: storage:logs:read\n"+
							"  Token used: %s\n"+
							"  Endpoint:   %s\n"+
							"  Response:   %s",
						resp.StatusCode, tokenHint, queryURL, strings.TrimSpace(string(body)),
					)
				default:
					lastErr = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
				}
			}
		}

		if lastErr != "" {
			logger.Warn("DQL poll error", "lastErr", lastErr)
		} else {
			logger.Debug("DQL poll tick")
		}
		if time.Now().After(deadline) {
			if lastErr != "" {
				return fmt.Errorf("timed out waiting for log to appear in Dynatrace\n\n  Last error: %s", lastErr)
			}
			return fmt.Errorf("timed out waiting for log to appear in Dynatrace")
		}
		fmt.Print(".")
		time.Sleep(5 * time.Second)
	}
}

// buildOtelLogsUIURL constructs the Dynatrace Logs UI deep-link pre-filtered
// to show records containing searchTerm, using the intent-based URL pattern.
func buildOtelLogsUIURL(envURL, searchTerm string) string {
	base := strings.TrimRight(installer.AppsURL(envURL), "/")
	fragment := fmt.Sprintf(
		`{"dt.query":"fetch logs","dt.segments":[],"showDqlEditor":false,"dt.queryConfig":{},"facetsCollapse":false,"filterFieldQuery":"content = *%s*"}`,
		searchTerm,
	)
	encoded := strings.ReplaceAll(url.QueryEscape(fragment), "+", "%20")
	return base + "/ui/apps/dynatrace.logs/intent/view_query#" + encoded
}

// otlpHTTPPortFromConfig reads the collector config at configPath and returns
// the HTTP OTLP receiver port. Falls back to 4318 when the file cannot be
// read or the endpoint field is absent.
func otlpHTTPPortFromConfig(configPath string) int {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 4318
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return 4318
	}
	root := doc.Content[0]
	receivers := nodeMappingGet(root, "receivers")
	otlp := nodeMappingGet(receivers, "otlp")
	protocols := nodeMappingGet(otlp, "protocols")
	httpProto := nodeMappingGet(protocols, "http")
	endpoint := nodeMappingGet(httpProto, "endpoint")
	if endpoint == nil {
		return 4318
	}
	_, portStr, err := net.SplitHostPort(endpoint.Value)
	if err != nil {
		return 4318
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return 4318
	}
	return port
}

// waitForOtelCollectorReady polls the collector's OTLP HTTP port until it
// accepts connections or the timeout elapses. crashed is closed when the
// process dies early so the probe can abort immediately.
func waitForOtelCollectorReady(timeout time.Duration, httpPort int, crashed <-chan error) error {
	addr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	deadline := time.Now().Add(timeout)
	for {
		// Try IPv4 loopback explicitly — avoids macOS resolving localhost→[::1]
		// while the collector only binds 0.0.0.0 (IPv4).
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		logger.Debug("waiting for collector port", "port", httpPort, "err", err)
		select {
		case crashErr := <-crashed:
			if crashErr != nil {
				return fmt.Errorf("collector process exited unexpectedly: %w", crashErr)
			}
			return fmt.Errorf("collector process exited unexpectedly")
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("collector did not open port %d within %s: %w", httpPort, timeout, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// verifyOtelInstall sends a verification log through the running collector,
// waits for it to arrive in Dynatrace via DQL query, then prints the UI link.
//
// For the DQL query, platformToken is preferred; if empty, apiToken is used
// as a fallback (matching the Python: platform_token or api_token). The Grail
// DQL endpoint always requires Bearer auth. If neither token is set,
// verification is skipped with a manual-check link.
func verifyOtelInstall(envURL, platformToken, apiToken string, httpPort int, crashed <-chan error) error {
	// Prefer platform token; fall back to API token.
	dqlToken := platformToken
	if dqlToken == "" {
		dqlToken = apiToken
	}

	hostname, _ := os.Hostname()
	// Unique search token: hostname + unix seconds — short and searchable.
	uniqueID := fmt.Sprintf("dtwiz-%s-%d", strings.ReplaceAll(hostname, ".", "-"), time.Now().Unix())
	logger.Debug("verification ID generated", "uniqueID", uniqueID)

	body := fmt.Sprintf(
		"OpenTelemetry Collector Successfully installed with dtwiz [host: %s, os: %s/%s, id: %s]",
		hostname, runtime.GOOS, runtime.GOARCH, uniqueID,
	)

	fmt.Println()
	fmt.Printf("  Waiting for collector to be ready...")
	if err := waitForOtelCollectorReady(30*time.Second, httpPort, crashed); err != nil {
		return fmt.Errorf("collector not ready: %w", err)
	}
	fmt.Println(" ✓")

	fmt.Printf("  Sending verification log to collector...\n")
	if err := sendOtelVerificationLog(body, httpPort); err != nil {
		return fmt.Errorf("sending verification log: %w", err)
	}
	if dqlToken == "" {
		fmt.Println("  Skipping DQL log verification (no token available).")
		fmt.Println()
		logsURL := buildOtelLogsUIURL(envURL, uniqueID)
		fmt.Println("  Check manually:", termLink("Open in Dynatrace Logs", logsURL))
		return nil
	}

	fmt.Printf("  Log sent. Waiting for it to appear in Dynatrace")

	if err := waitForLogInDynatraceFn(envURL, dqlToken, uniqueID, 2*time.Minute); err != nil {
		return err
	}

	fmt.Println(" ✓")
	fmt.Println()
	logsURL := buildOtelLogsUIURL(envURL, uniqueID)
	fmt.Println("  🎉 View the logline:", termLink("Open in Dynatrace Logs", logsURL))
	return nil
}

// termSupportsOSC8 reports whether the current terminal likely supports OSC 8
// hyperlinks. VS Code, iTerm2, WezTerm, and Windows Terminal do; macOS
// Terminal.app (Apple_Terminal) and plain xterm do not.
func termSupportsOSC8() bool {
	switch os.Getenv("TERM_PROGRAM") {
	case "vscode", "iTerm.app", "WezTerm", "Hyper":
		return true
	}
	// Windows Terminal sets WT_SESSION (not TERM_PROGRAM).
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	// GNOME Terminal / VTE-based terminals.
	if os.Getenv("VTE_VERSION") != "" {
		return true
	}
	return false
}

// termLink returns a clickable terminal hyperlink when the terminal supports
// OSC 8, otherwise returns "label: url" so the user can copy-paste the URL.
func termLink(label, url string) string {
	if termSupportsOSC8() {
		// Format: ESC]8;;URL ESC\ label ESC]8;; ESC\
		return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, label)
	}
	return fmt.Sprintf("%s:\n    %s", label, url)
}

// generateOtelConfig renders otel.tmpl and returns a collector configuration YAML string.
// It probes for free ports starting at the canonical defaults so multiple collectors can
// run on the same host without conflicting.
// When the Experimental feature flag is enabled, the combined host+app config is rendered;
// otherwise the app-only config is rendered (identical to the pre-host-monitoring output).
func generateOtelConfig(apiURL, token string) (string, error) {
	tmpl, err := template.New("otel").Parse(otelConfigTemplateText)
	if err != nil {
		return "", fmt.Errorf("parsing otel template: %w", err)
	}
	grpcPort := findFreePort(4317)
	httpPort := findFreePort(4318)
	if httpPort == grpcPort {
		httpPort = findFreePort(grpcPort + 1)
	}
	metricsPort := findFreePort(8888)
	if metricsPort == grpcPort || metricsPort == httpPort {
		metricsPort = findFreePort(httpPort + 1)
	}

	data := otelConfigData{
		Endpoint:    strings.TrimRight(apiURL, "/"),
		AuthHeader:  installer.AuthHeader(token),
		MetricsPort: metricsPort,
		GRPCPort:    grpcPort,
		HTTPPort:    httpPort,
	}

	if featureflags.IsEnabled(featureflags.Experimental) {
		healthCheckPort := findFreePort(13133)
		for healthCheckPort == grpcPort || healthCheckPort == httpPort || healthCheckPort == metricsPort {
			healthCheckPort = findFreePort(healthCheckPort + 1)
		}
		data.HostMonitoring = true
		data.IncludeJournald = runtime.GOOS == "linux"
		data.HealthCheckPort = healthCheckPort
		logger.Debug("otel config ports", "grpc", grpcPort, "http", httpPort, "metrics", metricsPort, "health_check", healthCheckPort)
	} else {
		logger.Debug("otel config ports", "grpc", grpcPort, "http", httpPort, "metrics", metricsPort)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering otel template: %w", err)
	}

	rendered := buf.String()
	var parsed any
	if err := yaml.Unmarshal([]byte(rendered), &parsed); err != nil {
		return "", fmt.Errorf("rendered otel config is not valid YAML: %w", err)
	}

	return rendered, nil
}

// printConfigPreview prints the OTel Collector config preview.
// Default: head (up to OTLP receiver endpoints) + "..." + pipelines section.
// With --debug: full config.
func (cp *collectorPlan) printConfigPreview(sep string) {
	const headLines = 20

	label := filepath.Base(cp.configPath)
	lines := strings.Split(strings.TrimRight(cp.configPreview, "\n"), "\n")

	fmt.Println()
	fmt.Printf("  %s\n", sep)
	display.ColorMessage.Printf("  %s:\n", label)
	fmt.Printf("  %s\n", sep)

	if logger.IsDebug() {
		for _, line := range lines {
			fmt.Printf("    %s\n", line)
		}
	} else {
		headEnd := configHeadEnd(lines, headLines)
		pipeStart := pipelinesSectionStart(lines)
		for _, line := range lines[:headEnd] {
			fmt.Printf("    %s\n", line)
		}
		const truncateThreshold = 30
		switch {
		case pipeStart > headEnd && pipeStart-headEnd > truncateThreshold:
			fmt.Printf("    # ... (%d lines) — run with --debug to see full %s\n", pipeStart-headEnd, label)
			for _, line := range lines[pipeStart:] {
				fmt.Printf("    %s\n", line)
			}
		case pipeStart > headEnd:
			for _, line := range lines[headEnd:] {
				fmt.Printf("    %s\n", line)
			}
		case len(lines)-headEnd > truncateThreshold:
			fmt.Printf("    # ... (%d more lines) — run with --debug to see full %s\n", len(lines)-headEnd, label)
		case len(lines) > headEnd:
			for _, line := range lines[headEnd:] {
				fmt.Printf("    %s\n", line)
			}
		}
	}

	fmt.Printf("  %s\n", sep)
}

// configHeadEnd returns the index at which the head section should stop.
// It cuts just before the first hostmetrics receiver block so that scraper
// details are hidden in the ellipsis. Falls back to min(headLines, len(lines)).
func configHeadEnd(lines []string, headLines int) int {
	limit := min(headLines, len(lines))
	for i := range limit {
		trimmed := strings.TrimLeft(lines[i], " ")
		if strings.HasPrefix(trimmed, "host_metrics/") || strings.HasPrefix(trimmed, "hostmetrics/") {
			return i
		}
	}
	return limit
}

// pipelinesSectionStart returns the index of the "  pipelines:" line, or -1 if not found.
func pipelinesSectionStart(lines []string) int {
	for i, line := range lines {
		if strings.TrimRight(line, " ") == "  pipelines:" {
			return i
		}
	}
	return -1
}

// runningCollector holds info about a detected running OTel Collector process.
type runningCollector struct {
	pid  int
	path string
}

// findRunningOtelCollectors returns info about all running dynatrace-otel-collector
// processes (there may be more than one if a previous kill was incomplete).
// Platform-specific detection is in otel_collector_windows.go (//go:build windows)
// and otel_collector_other.go (//go:build !windows).

// formatPIDs formats a slice of PIDs as a comma-separated string.
func formatPIDs(procs []runningCollector) string {
	s := make([]string, len(procs))
	for i, p := range procs {
		s[i] = strconv.Itoa(p.pid)
	}
	return strings.Join(s, ", ")
}

// startOtelCollector starts the collector as a background process.
// It waits briefly to detect immediate startup failures; if the process is
// still running after the check, a goroutine continues to monitor it via cmd.Wait().
// The returned channel receives the exit error (or nil) if the process later dies.
func startOtelCollector(binaryPath, configPath string) (<-chan error, error) {
	logPath := filepath.Join(filepath.Dir(configPath), "dynatrace-otel-collector.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("creating collector log file: %w", err)
	}

	cmd := exec.Command(binaryPath, "--config", configPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("starting OTel Collector: %w", err)
	}

	pid := cmd.Process.Pid
	fmt.Printf("  %s started (PID %d).\n", filepath.Base(binaryPath), pid)
	fmt.Printf("  Collector log: %s\n", logPath)

	// Monitor the process; send its exit status on the channel.
	crashed := make(chan error, 1)
	go func() {
		defer logFile.Close()
		crashed <- cmd.Wait()
	}()

	// Give it a moment to fail fast on obvious misconfigurations.
	select {
	case err := <-crashed:
		if err != nil {
			return nil, fmt.Errorf("OTel Collector exited immediately: %w", err)
		}
		return nil, fmt.Errorf("OTel Collector exited immediately with no error (check %s for details)", logPath)
	case <-time.After(3 * time.Second):
		fmt.Printf("  %s is running in the background (PID %d). Detaching...\n", filepath.Base(binaryPath), pid)
	}

	fmt.Println("  OpenTelemetry Collector running.")
	return crashed, nil
}

// collectorPlan holds all pre-computed state for a collector install so we can
// show a preview before touching disk.
type collectorPlan struct {
	apiURL         string
	collectorToken string
	installDir     string
	configPath     string
	binaryPath     string
	configContent  string
	configPreview  string
	runningPIDs    []runningCollector
}

func prepareCollectorPlan(envURL, token string) (*collectorPlan, error) {
	apiURL := installer.APIURL(envURL)
	collectorToken := token
	installDir, err := otelCollectorInstallDir()
	if err != nil {
		return nil, err
	}
	configContent, err := generateOtelConfig(apiURL, collectorToken)
	if err != nil {
		return nil, fmt.Errorf("generating OTel Collector config: %w", err)
	}
	return &collectorPlan{
		apiURL:         apiURL,
		collectorToken: collectorToken,
		installDir:     installDir,
		configPath:     filepath.Join(installDir, "config.yaml"),
		binaryPath:     filepath.Join(installDir, otelCollectorBinaryName()),
		configContent:  configContent,
		configPreview:  installer.MaskSecret(configContent, collectorToken),
		runningPIDs:    findRunningOtelCollectors(),
	}, nil
}

func (cp *collectorPlan) printDryRun() {
	sep := strings.Repeat("─", 60)

	fmt.Println("[dry-run] Would install Dynatrace OpenTelemetry Collector")
	fmt.Printf("  Install dir:  %s\n", cp.installDir)
	fmt.Printf("  Binary:       %s\n", cp.binaryPath)
	fmt.Printf("  Config:       %s\n", cp.configPath)
	assetName, _ := otelPlatformAssetName("latest")
	fmt.Printf("  Asset:        %s\n", assetName)
	fmt.Printf("  Ingest token: (configured)\n")

	cp.printConfigPreview(sep)
}

// execute downloads, writes config, and starts the collector.
// When skipVerification is true the test-log round-trip is skipped
// (useful when auto-instrumentation will generate real traffic next).
func (cp *collectorPlan) execute(envURL, platformToken string, skipVerification bool) error {
	if err := os.MkdirAll(cp.installDir, 0o755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	// Stop any running collectors first so file locks are released before
	// the download overwrites the binary (critical on Windows).
	if procs := cp.runningPIDs; len(procs) > 0 {
		fmt.Printf("  Stopping existing collector (PIDs: %s)...\n", formatPIDs(procs))
		for _, rc := range procs {
			proc, err := os.FindProcess(rc.pid)
			if err != nil {
				fmt.Printf("  Warning: could not find process %d: %v\n", rc.pid, err)
				continue
			}
			if err := installer.KillAndWaitProcess(proc); err != nil {
				fmt.Printf("  Warning: could not kill process %d: %v\n", rc.pid, err)
				continue
			}
			fmt.Printf("  Stopped collector (PID %d).\n", rc.pid)
		}
	}

	binaryPath, err := downloadOtelCollector(cp.installDir)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cp.configPath, []byte(cp.configContent), 0o600); err != nil {
		return fmt.Errorf("writing OTel Collector config: %w", err)
	}
	fmt.Printf("  Config written to: %s\n", cp.configPath)

	crashed, err := startOtelCollector(binaryPath, cp.configPath)
	if err != nil {
		return err
	}

	if skipVerification {
		fmt.Println("  Collector started — skipping verification (app instrumentation will follow).")
		return nil
	}

	if err := verifyOtelInstall(envURL, platformToken, cp.collectorToken, otlpHTTPPortFromConfig(cp.configPath), crashed); err != nil {
		fmt.Printf("\n  Warning: log verification failed: %v\n", err)
		fmt.Println("  The collector may still be working — check the Dynatrace UI.")
	}

	return nil
}

// InstallOtelCollectorOnly installs the Dynatrace OTel Collector without
// runtime instrumentation.
func InstallOtelCollectorOnly(envURL, token, platformToken string, dryRun bool) error {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace OpenTelemetry Installation")
	fmt.Println()

	cp, err := prepareCollectorPlan(envURL, token)
	if err != nil {
		return err
	}

	if dryRun {
		cp.printDryRun()
		return nil
	}

	sep := strings.Repeat("─", 60)

	fmt.Printf("  Directory: %s\n", cp.installDir)
	fmt.Printf("  Binary:    %s\n", cp.binaryPath)
	if len(cp.runningPIDs) > 0 {
		for _, rc := range cp.runningPIDs {
			if rc.path != "" {
				fmt.Printf("  Running:  Existing OTel Collector PID %d at %s (will be stopped)\n", rc.pid, rc.path)
			} else {
				fmt.Printf("  Running:  Existing OTel Collector PID %d (will be stopped)\n", rc.pid)
			}
		}
	}

	cp.printConfigPreview(sep)

	fmt.Println()
	ok, err := installer.ConfirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	return cp.execute(envURL, platformToken, false)
}
