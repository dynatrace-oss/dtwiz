package installer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/fatih/color"
)

// DetectedProcess describes a running process relevant to a runtime.
type DetectedProcess struct {
	PID     int
	Command string // full command line
	CWD     string // working directory of the process
}

// ScannedProject describes a project directory that contains one or more marker files.
type ScannedProject struct {
	Path        string   // absolute path to the project directory
	Markers     []string // which indicator files were found
	RunningPIDs []int    // PIDs of processes running from this project
}

// commonNoiseDirs is the shared set of directory names that are never useful
// project roots: OS-managed macOS home folders, VCS metadata, build artifacts,
// and language-specific cache/dependency trees.
var commonNoiseDirs = map[string]bool{
	// macOS home top-level folders
	"Library": true, "Applications": true, "System": true,
	"Movies": true, "Music": true, "Pictures": true, "Public": true,
	// VCS
	".git": true, ".svn": true,
	// dependency trees / build artifacts
	"node_modules": true, "vendor": true,
	"venv": true, ".venv": true, "__pycache__": true,
	"dist": true, "build": true, "target": true, "out": true,
}

// scanProjectDirs scans common locations for directories containing any of the
// specified marker files. Directories whose name appears in excludeNames are
// skipped, as are all names in commonNoiseDirs.
// Strategy:
//  1. Check CWD and its immediate subdirectories.
//  2. Walk up to 2 ancestor directories from CWD; for each level, scan all
//     sibling directories. Stop early once projects are found at a given level.
func scanProjectDirs(markers []string, excludeNames []string) []ScannedProject {
	excludeSet := make(map[string]bool, len(excludeNames))
	for _, name := range excludeNames {
		excludeSet[name] = true
	}

	skipDir := func(name string) bool {
		return strings.HasPrefix(name, ".") || excludeSet[name] || commonNoiseDirs[name]
	}

	var projects []ScannedProject
	seen := make(map[string]bool)

	checkDir := func(dir string) {
		if skipDir(filepath.Base(dir)) {
			return
		}
		// Resolve symlinks and normalize to lowercase for case-insensitive
		// filesystems (macOS APFS).
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			resolved = dir
		}
		key := strings.ToLower(resolved)
		if seen[key] {
			return
		}
		seen[key] = true
		var found []string
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				found = append(found, marker)
			}
		}
		if len(found) > 0 {
			logger.Debug("project dir matched", "path", dir, "markers", strings.Join(found, ","))
			projects = append(projects, ScannedProject{Path: dir, Markers: found})
		} else {
			logger.Debug("project dir scanned, no markers", "path", dir, "looking_for", strings.Join(markers, ","))
		}
	}

	// scanSiblings scans all non-noise subdirectories of dir.
	// If a subdirectory has no markers itself (e.g. a monorepo root like
	// terra-sample-apps/), its children are also checked one level deeper.
	// Returns the number of new projects found.
	scanSiblings := func(dir string) int {
		before := len(projects)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() || skipDir(e.Name()) {
				continue
			}
			child := filepath.Join(dir, e.Name())
			beforeChild := len(projects)
			checkDir(child)
			if len(projects) == beforeChild {
				// No markers found — descend one level deeper
				subEntries, _ := os.ReadDir(child)
				for _, se := range subEntries {
					if se.IsDir() && !skipDir(se.Name()) {
						checkDir(filepath.Join(child, se.Name()))
					}
				}
			}
		}
		return len(projects) - before
	}

	cwd, err := os.Getwd()
	if err != nil {
		return projects
	}

	// Step 1: CWD itself and its immediate children.
	checkDir(cwd)
	scanSiblings(cwd)

	// Steps 2–3: walk up to 2 ancestor levels; stop early if projects are found.
	dir := cwd
	for range 2 {
		parent := filepath.Dir(dir)
		if parent == dir {
			break // filesystem root
		}
		if scanSiblings(parent) > 0 {
			break
		}
		dir = parent
	}

	return projects
}

// detectProcesses finds running processes whose command line contains filterTerm.
// excludeTerms is a list of substrings that disqualify a matching line.
// On Unix, uses `ps ax`; returns nil on any error (best-effort).
func detectProcesses(filterTerm string, excludeTerms []string) []DetectedProcess {
	out, err := exec.Command("ps", "ax", "-o", "pid=,command=").Output()
	if err != nil {
		logger.Warn("ps command failed", "filter", filterTerm, "err", err)
		return nil
	}
	logger.Debug("scanning processes", "filter", filterTerm)

	var procs []DetectedProcess
	myPID := os.Getpid()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid == myPID {
			continue
		}
		cmd := strings.TrimSpace(parts[1])
		if !strings.Contains(cmd, filterTerm) {
			continue
		}
		skip := false
		for _, ex := range excludeTerms {
			if strings.Contains(cmd, ex) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		procs = append(procs, DetectedProcess{PID: pid, Command: cmd, CWD: getProcessCWD(pid)})
	}
	logger.Debug("process scan complete", "filter", filterTerm, "matched", len(procs))
	return procs
}

// getProcessCWD returns the current working directory of a process using lsof.
// Returns "" if the CWD cannot be determined (best-effort).
func getProcessCWD(pid int) string {
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strconv.Itoa(pid), "-Fn").Output()
	if err != nil {
		logger.Warn("lsof cwd lookup failed", "pid", pid, "err", err)
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}

// processMatchPIDs returns PIDs from procs whose CWD starts with dirPath or
// whose command line contains dirPath (case-insensitive).
func processMatchPIDs(dirPath string, procs []DetectedProcess) []int {
	dirLower := strings.ToLower(dirPath)
	var pids []int
	for _, p := range procs {
		cwdLower := strings.ToLower(p.CWD)
		cmdLower := strings.ToLower(p.Command)
		if strings.HasPrefix(cwdLower, dirLower) || strings.Contains(cmdLower, dirLower) {
			pids = append(pids, p.PID)
		}
	}
	return pids
}

// generateBaseOtelEnvVars returns the OTEL_* environment variables common to
// all runtimes exporting to Dynatrace. Headers use URL-encoded values.
func generateBaseOtelEnvVars(apiURL, token, serviceName string) map[string]string {
	return map[string]string{
		"OTEL_SERVICE_NAME":                                   serviceName,
		"OTEL_EXPORTER_OTLP_ENDPOINT":                        strings.TrimRight(apiURL, "/") + "/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_HEADERS":                         "Authorization=Api-Token%20" + token,
		"OTEL_EXPORTER_OTLP_PROTOCOL":                        "http/protobuf",
		"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE":  "delta",
		"OTEL_TRACES_EXPORTER":                               "otlp",
		"OTEL_METRICS_EXPORTER":                              "otlp",
		"OTEL_LOGS_EXPORTER":                                 "otlp",
	}
}

func serviceNameFromPath(projectPath string) string {
	base := filepath.Base(projectPath)
	if base == "" || base == "." || base == "/" {
		return "my-service"
	}
	return base
}

func matchProcessesToProjects(projects []ScannedProject, procs []DetectedProcess) {
	for i := range projects {
		projects[i].RunningPIDs = processMatchPIDs(projects[i].Path, procs)
	}
}

// promptProjectSelection prints a numbered list of ScannedProjects and prompts
// the user to pick one. Returns a pointer to the selected project, or nil if
// the user presses Enter to skip or enters an invalid number.
func promptProjectSelection(label string, projects []ScannedProject) *ScannedProject {
	header := color.New(color.FgMagenta)
	fmt.Println()
	header.Printf("  %s projects on this machine:\n", label)
	fmt.Println("  " + strings.Repeat("─", 50))
	for i, proj := range projects {
		line := fmt.Sprintf("  [%d]  %s  (%s)", i+1, proj.Path, strings.Join(proj.Markers, ", "))
		if len(proj.RunningPIDs) > 0 {
			pidStrs := make([]string, len(proj.RunningPIDs))
			for j, pid := range proj.RunningPIDs {
				pidStrs[j] = strconv.Itoa(pid)
			}
			line += fmt.Sprintf("  ← PIDs: %s", strings.Join(pidStrs, ", "))
		}
		fmt.Println(line)
	}
	fmt.Println()
	fmt.Printf("  Select a project to instrument [1-%d] or press Enter to skip: ", len(projects))
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil
	}
	num, err := strconv.Atoi(answer)
	if err != nil || num < 1 || num > len(projects) {
		fmt.Println("  Invalid selection, skipping instrumentation.")
		return nil
	}
	return &projects[num-1]
}

func GenerateEnvExportScript(envVars map[string]string) string {
	var sb strings.Builder
	sb.WriteString("# Dynatrace OpenTelemetry auto-instrumentation environment variables\n")
	for k, v := range envVars {
		sb.WriteString(fmt.Sprintf("export %s=%q\n", k, v))
	}
	return sb.String()
}

// envVarsToSlice converts an env var map to KEY=VALUE slice for exec.Cmd.Env.
func envVarsToSlice(envVars map[string]string) []string {
	out := make([]string, 0, len(envVars))
	for k, v := range envVars {
		out = append(out, k+"="+v)
	}
	return out
}

// stopProcesses sends SIGINT to the given PIDs and waits for them to exit.
func stopProcesses(pids []int) {
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := proc.Signal(os.Interrupt); err != nil {
			fmt.Printf("    Warning: could not stop PID %d: %v\n", pid, err)
			continue
		}
		// Wait for the process to actually exit so ports are released.
		_, _ = proc.Wait()
		fmt.Printf("    Stopped PID %d\n", pid)
	}
}

// detectListeningPort uses lsof to find the TCP port a process is listening on.
// Returns the port string or "" if none found.
func detectListeningPort(pid int) string {
	out, err := exec.Command("lsof", "-a", "-i", "TCP", "-sTCP:LISTEN", "-p", strconv.Itoa(pid), "-Fn", "-P").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		// lsof -Fn outputs lines like "n*:8000" or "n[::]:5000".
		if !strings.HasPrefix(line, "n") {
			continue
		}
		if idx := strings.LastIndex(line, ":"); idx >= 0 {
			port := line[idx+1:]
			// Skip the OTLP exporter port — we want the app port.
			if port != "4317" && port != "4318" {
				return port
			}
		}
	}
	return ""
}

// dqlResponse is the minimal structure for a Dynatrace DQL query response.
type dqlResponse struct {
	Result struct {
		Records []map[string]interface{} `json:"records"`
	} `json:"result"`
}

// waitForServices polls Dynatrace via DQL to check if the started services
// appear as smartscape SERVICE entities. Prints a link when found.
func waitForServices(envURL, platformToken string, serviceNames []string) {
	if len(serviceNames) == 0 || platformToken == "" {
		return
	}

	appsURL := AppsURL(envURL)
	apiURL := appsURL + "/platform/storage/query/v1/query:execute"
	logger.Debug("waiting for services in Dynatrace", "services", strings.Join(serviceNames, ","), "url", apiURL)

	conditions := make([]string, len(serviceNames))
	for i, name := range serviceNames {
		conditions[i] = fmt.Sprintf("name == \"%s\"", name)
	}
	dql := fmt.Sprintf("smartscapeNodes SERVICE | filter %s", strings.Join(conditions, " or "))

	remaining := make(map[string]bool, len(serviceNames))
	for _, name := range serviceNames {
		remaining[name] = true
	}

	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Println()
			if len(remaining) > 0 {
				names := make([]string, 0, len(remaining))
				for n := range remaining {
					names = append(names, n)
				}
				fmt.Printf("  Timed out waiting for: %s\n", strings.Join(names, ", "))
				fmt.Println("  Services may take a few more minutes to appear in Dynatrace.")
			}
			return
		case <-ticker.C:
			logger.Debug("polling smartscape for services", "remaining", len(remaining))
			found := querySmartscapeServices(apiURL, platformToken, dql)
			for _, name := range found {
				if remaining[name] {
					delete(remaining, name)
					fmt.Printf("  ✓ \"%s\" appeared in Dynatrace → %s\n", name, appsURL+"/ui/apps/my.getting.started.dieter/")
				}
			}
			if len(remaining) == 0 {
				fmt.Println()
				fmt.Println("  All services are reporting to Dynatrace.")
				return
			}
		}
	}
}

// querySmartscapeServices executes a DQL query and returns the entity names found.
func querySmartscapeServices(apiURL, platformToken, dql string) []string {
	payload := map[string]interface{}{
		"query":                      dql,
		"requestTimeoutMilliseconds": 10000,
		"maxResultRecords":           100,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+platformToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("smartscape DQL request failed", "err", err)
		return nil
	}
	defer resp.Body.Close()

	logger.Debug("smartscape DQL response", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	var result dqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	var names []string
	for _, rec := range result.Result.Records {
		if name, ok := rec["name"].(string); ok {
			names = append(names, name)
		}
	}
	logger.Debug("smartscape DQL found services", "count", len(names), "names", strings.Join(names, ","))
	return names
}
