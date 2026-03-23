package installer

import (
	"archive/tar"
	"archive/zip"
	"bufio"
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
)

//go:embed otel.tmpl
var otelConfigTemplateText string

// otelConfigData holds the values substituted into otel.tmpl.
type otelConfigData struct {
	Endpoint   string
	AuthHeader string
}

// otelCollectorBinaryName returns the expected binary name inside the release archive.
func otelCollectorBinaryName() string {
	if runtime.GOOS == "windows" {
		return "dynatrace-otel-collector.exe"
	}
	return "dynatrace-otel-collector"
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
func sendOtelVerificationLog(body string) error {
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

	resp, err := http.Post("http://127.0.0.1:4318/v1/logs", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sending OTLP log: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OTLP endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// waitForLogInDynatrace queries the Dynatrace Grail DQL API directly until a
// log record containing searchTerm appears, or until the timeout elapses.
//
// The DQL endpoint lives on the .apps. URL variant:
//
//	POST https://<env>.apps.<domain>/platform/storage/query/v1/query:execute
func waitForLogInDynatrace(envURL, token, searchTerm string, timeout time.Duration) error {
	appsBase := strings.TrimRight(toAppsURL(envURL), "/")
	queryURL := appsBase + "/platform/storage/query/v1/query:execute"

	dqlQuery := fmt.Sprintf(
		`fetch logs, from: now()-10m | filter contains(content, "%s") | limit 1`,
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

				if resp.StatusCode/100 == 2 {
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
				} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
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
				} else if resp.StatusCode/100 != 2 {
					lastErr = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
				}
			}
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

// toAppsURL converts a classic Dynatrace environment URL to its apps-platform
// equivalent, which is required for UI deep-links and Grail DQL queries.
// e.g. https://fxz0998d.dev.dynatracelabs.com → https://fxz0998d.dev.apps.dynatracelabs.com
// If the URL already contains ".apps." it is returned unchanged.
func toAppsURL(envURL string) string {
	if strings.Contains(envURL, ".apps.") {
		return envURL
	}
	// Insert ".apps." before the well-known SaaS domain suffixes.
	for _, suffix := range []string{".dynatracelabs.com", ".dynatrace.com"} {
		if idx := strings.Index(envURL, suffix); idx != -1 {
			return envURL[:idx] + ".apps" + envURL[idx:]
		}
	}
	return envURL // unknown domain — return as-is
}

// buildOtelLogsUIURL constructs the Dynatrace Logs UI deep-link pre-filtered
// to show records containing searchTerm, using the intent-based URL pattern.
func buildOtelLogsUIURL(envURL, searchTerm string) string {
	base := strings.TrimRight(toAppsURL(envURL), "/")
	fragment := fmt.Sprintf(
		`{"dt.query":"fetch logs","dt.segments":[],"showDqlEditor":false,"dt.queryConfig":{},"facetsCollapse":false,"filterFieldQuery":"content = *%s*"}`,
		searchTerm,
	)
	encoded := strings.ReplaceAll(url.QueryEscape(fragment), "+", "%20")
	return base + "/ui/apps/dynatrace.logs/intent/view_query#" + encoded
}

// waitForOtelCollectorReady polls TCP port 4318 until the collector accepts
// connections or the timeout elapses. crashed is closed when the process dies
// early so the probe can abort immediately.
func waitForOtelCollectorReady(timeout time.Duration, crashed <-chan error) error {
	deadline := time.Now().Add(timeout)
	for {
		// Try IPv4 loopback explicitly — avoids macOS resolving localhost→[::1]
		// while the collector only binds 0.0.0.0 (IPv4).
		conn, err := net.DialTimeout("tcp", "127.0.0.1:4318", time.Second)
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
			return fmt.Errorf("collector did not open port 4318 within %s: %w", timeout, err)
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
func verifyOtelInstall(envURL, platformToken, apiToken string, crashed <-chan error) error {
	// Prefer platform token; fall back to API token.
	dqlToken := platformToken
	if dqlToken == "" {
		dqlToken = apiToken
	}

	hostname, _ := os.Hostname()
	// Unique search token: hostname + unix seconds — short and searchable.
	uniqueID := fmt.Sprintf("dtwiz-%s-%d", strings.ReplaceAll(hostname, ".", "-"), time.Now().Unix())

	body := fmt.Sprintf(
		"OpenTelemetry Collector Successfully installed with dtwiz [host: %s, os: %s/%s, id: %s]",
		hostname, runtime.GOOS, runtime.GOARCH, uniqueID,
	)

	fmt.Println()
	fmt.Printf("  Waiting for collector to be ready...")
	if err := waitForOtelCollectorReady(30*time.Second, crashed); err != nil {
		return fmt.Errorf("collector not ready: %w", err)
	}
	fmt.Println(" ✓")

	fmt.Printf("  Sending verification log to collector...\n")
	if err := sendOtelVerificationLog(body); err != nil {
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

	if err := waitForLogInDynatrace(envURL, dqlToken, uniqueID, 2*time.Minute); err != nil {
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
func generateOtelConfig(apiURL, token string) (string, error) {
	tmpl, err := template.New("otel").Parse(otelConfigTemplateText)
	if err != nil {
		return "", fmt.Errorf("parsing otel template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, otelConfigData{
		Endpoint:   strings.TrimRight(apiURL, "/"),
		AuthHeader: AuthHeader(token),
	}); err != nil {
		return "", fmt.Errorf("rendering otel template: %w", err)
	}
	return buf.String(), nil
}

// printFileBox prints file content inside a box-drawing frame so it is
// visually distinct from action/command lines in the plan output.
func printFileBox(path, content string) {
	label := filepath.Base(path)
	// Build the top border: ┌── filename ──────…
	minWidth := 50
	top := fmt.Sprintf("     ┌── %s ", label)
	if pad := minWidth - len(top); pad > 0 {
		top += strings.Repeat("─", pad)
	}
	fmt.Println(top)
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		fmt.Printf("     │ %s\n", line)
	}
	bottom := "     └" + strings.Repeat("─", minWidth-len("     └"))
	fmt.Println(bottom)
}

// findRunningOtelCollectors returns the PIDs of all running dynatrace-otel-collector
// processes (there may be more than one if a previous kill was incomplete).
func findRunningOtelCollectors() []int {
	if runtime.GOOS == "windows" {
		return findRunningOtelCollectorsWindows()
	}
	// Unix: use -f to match the full command line, catching processes started
	// via an absolute path or through a wrapper (e.g. go run).
	out, err := exec.Command("pgrep", "-f", "dynatrace-otel-collector").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, s := range strings.Fields(strings.TrimSpace(string(out))) {
		pid, err := strconv.Atoi(s)
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// findRunningOtelCollectorsWindows searches for all known OTel Collector
// process names on Windows, matching the same set the analyzer detects.
func findRunningOtelCollectorsWindows() []int {
	processNames := []string{
		"dynatrace-otel-collector",
		"otelcol",
		"otelcol-contrib",
	}
	seen := map[int]bool{}
	var pids []int

	for _, name := range processNames {
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			"Get-Process -Name '"+name+"' -ErrorAction SilentlyContinue | ForEach-Object { $_.Id }").Output()
		if err != nil {
			continue
		}
		for _, s := range strings.Fields(strings.TrimSpace(string(out))) {
			pid, err := strconv.Atoi(s)
			if err == nil && !seen[pid] {
				seen[pid] = true
				pids = append(pids, pid)
			}
		}
	}

	// Fall back to command-line search for custom-named builds.
	if len(pids) == 0 {
		for _, pattern := range []string{"otel-collector", "otelcol"} {
			out, err := exec.Command("powershell", "-NoProfile", "-Command",
				"Get-CimInstance Win32_Process | Where-Object { $_.Name -match '"+pattern+"' } | ForEach-Object { $_.ProcessId }").Output()
			if err != nil {
				continue
			}
			for _, s := range strings.Fields(strings.TrimSpace(string(out))) {
				pid, err := strconv.Atoi(s)
				if err == nil && !seen[pid] {
					seen[pid] = true
					pids = append(pids, pid)
				}
			}
		}
	}

	return pids
}

// startOtelCollector starts the collector as a background process.
// It waits briefly to detect immediate startup failures; if the process is
// still running after the check it is detached (the parent does not Wait on it).
// The returned channel receives the exit error (or nil) if the process later dies.
func startOtelCollector(binaryPath, configPath string) (<-chan error, error) {
	cmd := exec.Command(binaryPath, "--config", configPath)
	cmd.Stdout = os.Stdout

	// Pipe stderr through a filter — the collector writes structured logs to
	// stderr in tab-separated format: "<timestamp>\t<level>\t<component>\t<msg>".
	// Suppress info-level lines; forward warn/error/fatal and anything unrecognised.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting OTel Collector: %w", err)
	}

	// Drain stderr in the background, only printing non-info lines.
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(line, "\tinfo\t") {
				fmt.Fprintln(os.Stderr, line)
			}
		}
	}()

	pid := cmd.Process.Pid
	fmt.Printf("  Dynatrace OTel Collector started (PID %d).\n", pid)

	// Monitor the process; send its exit status on the channel.
	crashed := make(chan error, 1)
	go func() {
		crashed <- cmd.Wait()
	}()

	// Give it a moment to fail fast on obvious misconfigurations.
	select {
	case err := <-crashed:
		if err != nil {
			return nil, fmt.Errorf("OTel Collector exited immediately: %w", err)
		}
		fmt.Println("  Collector exited.")
		close(crashed)
		return crashed, nil
	case <-time.After(3 * time.Second):
		fmt.Printf("  Collector is running in the background (PID %d). Detaching...\n", pid)
		_ = cmd.Process.Release()
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
}

func prepareCollectorPlan(envURL, token, ingestToken string) (*collectorPlan, error) {
	apiURL := APIURL(envURL)
	collectorToken := ingestToken
	if collectorToken == "" {
		collectorToken = token
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	installDir := filepath.Join(cwd, "opentelemetry")
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
		configPreview:  strings.ReplaceAll(configContent, collectorToken, "<redacted>"),
	}, nil
}

func (cp *collectorPlan) printPlanSteps() {
	fmt.Printf("     Directory : %s\n", cp.installDir)
	fmt.Printf("     Binary    : %s\n", cp.binaryPath)
	fmt.Println()
	printFileBox(cp.configPath, cp.configPreview)
}

func (cp *collectorPlan) printDryRun(ingestToken string) {
	assetName, _ := otelPlatformAssetName("latest")
	fmt.Println("[dry-run] Would install Dynatrace OpenTelemetry Collector")
	fmt.Printf("  Install dir:  %s\n", cp.installDir)
	fmt.Printf("  Binary:       %s\n", cp.binaryPath)
	fmt.Printf("  Config:       %s\n", cp.configPath)
	fmt.Printf("  Asset:        %s\n", assetName)
	fmt.Printf("  Ingest token: %s\n", func() string {
		if ingestToken != "" {
			return "(from --access-token)"
		}
		return "(from token)"
	}())
	fmt.Println()
	printFileBox(cp.configPath, cp.configPreview)
}

// execute downloads, writes config, and starts the collector.
// When skipVerification is true the test-log round-trip is skipped
// (useful when auto-instrumentation will generate real traffic next).
func (cp *collectorPlan) execute(envURL, platformToken string, skipVerification bool) error {
	if err := os.MkdirAll(cp.installDir, 0o755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	binaryPath, err := downloadOtelCollector(cp.installDir)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cp.configPath, []byte(cp.configContent), 0o600); err != nil {
		return fmt.Errorf("writing OTel Collector config: %w", err)
	}
	fmt.Printf("  Config written to: %s\n", cp.configPath)

	if pids := findRunningOtelCollectors(); len(pids) > 0 {
		fmt.Printf("\n  Dynatrace OTel Collector already running (PIDs: %v).\n", pids)
		fmt.Print("  Kill them and start the new one? [Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "" && answer != "y" && answer != "yes" {
			return fmt.Errorf("aborted: collector already running (PIDs: %v)", pids)
		}
		for _, pid := range pids {
			proc, err := os.FindProcess(pid)
			if err != nil {
				fmt.Printf("  Warning: could not find process %d: %v\n", pid, err)
				continue
			}
			if err := proc.Kill(); err != nil {
				fmt.Printf("  Warning: could not kill process %d: %v\n", pid, err)
				continue
			}
			fmt.Printf("  Stopped collector (PID %d).\n", pid)
		}
	}

	crashed, err := startOtelCollector(binaryPath, cp.configPath)
	if err != nil {
		return err
	}

	if skipVerification {
		fmt.Println("  Collector started — skipping verification (app instrumentation will follow).")
		return nil
	}

	if err := verifyOtelInstall(envURL, platformToken, cp.collectorToken, crashed); err != nil {
		fmt.Printf("\n  Warning: log verification failed: %v\n", err)
		fmt.Println("  The collector may still be working — check the Dynatrace UI.")
	}

	return nil
}

// InstallOtelCollectorOnly installs the Dynatrace OTel Collector without
// runtime instrumentation.
func InstallOtelCollectorOnly(envURL, token, ingestToken, platformToken string, dryRun bool) error {
	fmt.Println()
	fmt.Println("  This installer will set up the Dynatrace OpenTelemetry Collector.")
	fmt.Println()

	cp, err := prepareCollectorPlan(envURL, token, ingestToken)
	if err != nil {
		return err
	}

	if dryRun {
		cp.printDryRun(ingestToken)
		return nil
	}

	fmt.Println()
	fmt.Println("  ── Plan ──")
	fmt.Println()
	cp.printPlanSteps()

	fmt.Println()
	fmt.Print("  Apply? [Y/n]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return fmt.Errorf("installation aborted")
	}
	fmt.Println()

	return cp.execute(envURL, platformToken, false)
}
