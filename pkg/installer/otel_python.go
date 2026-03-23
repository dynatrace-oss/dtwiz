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

	"github.com/fatih/color"
)

// PythonProcess describes a detected running Python process.
type PythonProcess struct {
	PID     int
	Command string // full command line
	CWD     string // working directory of the process
}

// detectPython finds a usable Python 3 executable on the current PATH,
// preferring python3 over python.
func detectPython() (string, error) {
	for _, name := range []string{"python3", "python"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		// Verify it's actually Python 3.
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(string(out)), "Python 3") {
			return path, nil
		}
	}
	return "", fmt.Errorf("Python 3 not found — install Python 3 and ensure it is in PATH")
}

// pipCommand holds the resolved pip executable and arguments.
type pipCommand struct {
	name string
	args []string
}

// otelPythonPackages is the list of OpenTelemetry packages to install for
// auto-instrumentation, following the Dynatrace documentation.
var otelPythonPackages = []string{
	"opentelemetry-distro",
	"opentelemetry-exporter-otlp",
}

// installPackages installs the given pip packages using the resolved pip command.
// Output is suppressed unless the command fails.
func installPackages(pip *pipCommand, packages []string) error {
	args := append(append([]string{}, pip.args...), append([]string{"install"}, packages...)...)
	cmd := exec.Command(pip.name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		return fmt.Errorf("pip install failed: %w", err)
	}
	return nil
}

// runOtelBootstrap runs `opentelemetry-bootstrap -a install` to automatically
// install instrumentation libraries for all packages found in the environment.
// Output is suppressed unless the command fails.
func runOtelBootstrap(pythonPath string) error {
	cmd := exec.Command(pythonPath, "-m", "opentelemetry.instrumentation.bootstrap", "-a", "install")
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		return fmt.Errorf("opentelemetry-bootstrap failed: %w", err)
	}
	return nil
}

// generateOtelPythonEnvVars returns the OTEL_* environment variables required
// for auto-instrumentation to export to Dynatrace.
func generateOtelPythonEnvVars(apiURL, token, serviceName string) map[string]string {
	return map[string]string{
		"OTEL_SERVICE_NAME":                              serviceName,
		"OTEL_EXPORTER_OTLP_ENDPOINT":                   strings.TrimRight(apiURL, "/") + "/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_HEADERS":                    "Authorization=Api-Token%20" + token,
		"OTEL_EXPORTER_OTLP_PROTOCOL":                   "http/protobuf",
		"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE": "delta",
		"OTEL_TRACES_EXPORTER":                          "otlp",
		"OTEL_METRICS_EXPORTER":                         "otlp",
		"OTEL_LOGS_EXPORTER":                            "otlp",
		"OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED": "true",
	}
}

// GenerateEnvExportScript returns a shell `export` script for the given env vars.
func GenerateEnvExportScript(envVars map[string]string) string {
	var sb strings.Builder
	sb.WriteString("# Dynatrace OpenTelemetry auto-instrumentation environment variables\n")
	for k, v := range envVars {
		sb.WriteString(fmt.Sprintf("export %s=%q\n", k, v))
	}
	return sb.String()
}

// detectPythonProcesses finds running Python processes (excluding the current
// process and common system Python processes).
func detectPythonProcesses() []PythonProcess {
	// Use ps to find python processes with full command line.
	out, err := exec.Command("ps", "ax", "-o", "pid=,command=").Output()
	if err != nil {
		return nil
	}

	var procs []PythonProcess
	myPID := os.Getpid()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split into PID and command.
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid == myPID {
			continue
		}
		cmd := strings.TrimSpace(parts[1])
		// Match python processes but skip system/pip/setup processes.
		if !strings.Contains(cmd, "python") {
			continue
		}
		if strings.Contains(cmd, "pip ") || strings.Contains(cmd, "setup.py") ||
			strings.Contains(cmd, "/bin/dtwiz") {
			continue
		}
		procs = append(procs, PythonProcess{PID: pid, Command: cmd, CWD: getProcessCWD(pid)})
	}
	return procs
}

// getProcessCWD returns the current working directory of a process using lsof.
func getProcessCWD(pid int) string {
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strconv.Itoa(pid), "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}

// PythonProject describes a detected Python project directory.
type PythonProject struct {
	Path        string   // absolute path to the project directory
	Markers     []string // which indicator files were found (e.g. "pyproject.toml", "requirements.txt")
	RunningPIDs []int    // PIDs of Python processes running from this project
}

// pythonProjectMarkers are the files that indicate a Python project root.
var pythonProjectMarkers = []string{
	"pyproject.toml",
	"setup.py",
	"setup.cfg",
	"requirements.txt",
	"Pipfile",
	"poetry.lock",
	"manage.py",
}

// detectPythonProjects scans common locations for Python project directories.
// Looks in the current working directory and one level of subdirectories, plus
// common project locations under $HOME.
func detectPythonProjects() []PythonProject {
	var projects []PythonProject
	seen := make(map[string]bool)

	checkDir := func(dir string) {
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
		var markers []string
		for _, marker := range pythonProjectMarkers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				markers = append(markers, marker)
			}
		}
		if len(markers) > 0 {
			projects = append(projects, PythonProject{Path: dir, Markers: markers})
		}
	}

	// Check CWD and immediate subdirectories.
	if cwd, err := os.Getwd(); err == nil {
		checkDir(cwd)
		entries, _ := os.ReadDir(cwd)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				checkDir(filepath.Join(cwd, e.Name()))
			}
		}
	}

	// Check common home-directory project locations (two levels deep).
	if home, err := os.UserHomeDir(); err == nil {
		for _, base := range []string{"Code", "code", "projects", "src", "dev"} {
			dir := filepath.Join(home, base)
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				sub := filepath.Join(dir, e.Name())
				checkDir(sub)
				// Also check one level deeper (e.g. ~/Code/data-generators/orderschnitzel).
				subEntries, err := os.ReadDir(sub)
				if err != nil {
					continue
				}
				for _, se := range subEntries {
					if se.IsDir() && !strings.HasPrefix(se.Name(), ".") {
						checkDir(filepath.Join(sub, se.Name()))
					}
				}
			}
		}
	}

	return projects
}

// matchProcessesToProjects associates detected Python processes with their
// project directories by checking if the process command line contains the
// project path.
func matchProcessesToProjects(projects []PythonProject, procs []PythonProcess) {
	for i := range projects {
		projLower := strings.ToLower(projects[i].Path)
		for _, p := range procs {
			cwdLower := strings.ToLower(p.CWD)
			cmdLower := strings.ToLower(p.Command)
			if strings.HasPrefix(cwdLower, projLower) || strings.Contains(cmdLower, projLower) {
				projects[i].RunningPIDs = append(projects[i].RunningPIDs, p.PID)
			}
		}
	}
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

// printManualInstructions prints the manual setup instructions when no project
// was selected for automatic instrumentation.
func printManualInstructions(envVars map[string]string) {
	fmt.Println()
	fmt.Println("  To instrument a Python application manually:")
	fmt.Println()
	fmt.Printf("    pip install %s\n", strings.Join(otelPythonPackages, " "))
	fmt.Println("    opentelemetry-bootstrap -a install")
	fmt.Println()
	fmt.Print(GenerateEnvExportScript(envVars))
	fmt.Println()
	fmt.Println("  Then run your application with:")
	fmt.Println("    opentelemetry-instrument python your_app.py")
}

// envVarsToSlice converts an env var map to KEY=VALUE slice for exec.Cmd.Env.
func envVarsToSlice(envVars map[string]string) []string {
	out := make([]string, 0, len(envVars))
	for k, v := range envVars {
		out = append(out, k+"="+v)
	}
	return out
}

// PythonInstrumentationPlan captures all the user's choices for Python
// auto-instrumentation so that detection/prompting and execution can happen
// at different times (e.g. choices upfront, execution after collector install).
type PythonInstrumentationPlan struct {
	Project     PythonProject
	Entrypoints []string
	NeedsVenv   bool
	EnvVars     map[string]string
	EnvURL      string
	PlatformToken string
}

// DetectPythonPlan scans for Python projects, shows them to the user, and
// returns a plan if the user selects one. Returns nil if the user skips or
// no projects are found.
func DetectPythonPlan(apiURL, token string) *PythonInstrumentationPlan {
	if _, err := detectPython(); err != nil {
		return nil
	}

	projects := detectPythonProjects()
	procs := detectPythonProcesses()
	matchProcessesToProjects(projects, procs)

	if len(projects) == 0 {
		return nil
	}

	otelHeader := color.New(color.FgMagenta)
	otelMuted := color.New()

	fmt.Println()
	otelHeader.Println("  Python projects on this machine:")
	otelMuted.Println("  " + strings.Repeat("─", 50))
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

	proj := projects[num-1]
	entrypoints := detectPythonEntrypoints(proj.Path)
	if len(entrypoints) == 0 {
		fmt.Print("  No entrypoint detected. Enter the Python file to run (e.g. app.py): ")
		ep, _ := reader.ReadString('\n')
		ep = strings.TrimSpace(ep)
		if ep == "" {
			return nil
		}
		entrypoints = []string{ep}
	}

	needsVenv := detectProjectPip(proj.Path) == nil

	// Generate env vars using the API URL derived from what the caller provides.
	envVars := generateOtelPythonEnvVars(apiURL, token, "my-service")

	return &PythonInstrumentationPlan{
		Project:     proj,
		Entrypoints: entrypoints,
		NeedsVenv:   needsVenv,
		EnvVars:     envVars,
	}
}

// PrintPlanSteps prints the Python instrumentation steps for inclusion in a
// combined plan preview.
func (p *PythonInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project: %s\n", p.Project.Path)
	if len(p.Project.RunningPIDs) > 0 {
		pidStrs := make([]string, len(p.Project.RunningPIDs))
		for i, pid := range p.Project.RunningPIDs {
			pidStrs[i] = strconv.Itoa(pid)
		}
		fmt.Printf("     Stop running processes (PIDs: %s)\n", strings.Join(pidStrs, ", "))
	}
	if p.NeedsVenv {
		fmt.Println("     Create virtualenv (.venv)")
	}
	fmt.Printf("     pip install %s\n", strings.Join(otelPythonPackages, " "))
	fmt.Println("     opentelemetry-bootstrap -a install")
	for _, ep := range p.Entrypoints {
		svcName := serviceNameFromEntrypoint(p.Project.Path, ep)
		fmt.Printf("     opentelemetry-instrument python %s  (service: %s)\n", ep, svcName)
	}
}

// Execute runs the Python instrumentation plan (no prompts — assumes already confirmed).
func (p *PythonInstrumentationPlan) Execute() {
	proj := p.Project
	envVars := p.EnvVars

	venvPip := detectProjectPip(proj.Path)
	otelInstrument := resolveVenvBinary(proj.Path, "opentelemetry-instrument")
	pythonBin := resolveVenvBinary(proj.Path, "python")
	if pythonBin == "" {
		pythonBin = "python3"
	}

	// Stop running processes.
	if len(proj.RunningPIDs) > 0 {
		fmt.Print("  Stopping running processes... ")
		stopProcesses(proj.RunningPIDs)
		fmt.Println("done.")
	}

	// Create venv if needed.
	if p.NeedsVenv {
		fmt.Print("  Creating virtualenv... ")
		pythonPath, err := detectPython()
		if err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			return
		}
		venvDir := filepath.Join(proj.Path, ".venv")
		cmd := exec.Command(pythonPath, "-m", "venv", venvDir)
		cmd.Dir = proj.Path
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("failed.")
			os.Stdout.Write(out)
			return
		}
		fmt.Println("done.")
		venvPip = detectProjectPip(proj.Path)
		if venvPip == nil {
			fmt.Println("    Could not find pip in new virtualenv.")
			return
		}
		otelInstrument = resolveVenvBinary(proj.Path, "opentelemetry-instrument")
		pythonBin = resolveVenvBinary(proj.Path, "python")
		if pythonBin == "" {
			pythonBin = "python3"
		}
	}

	fmt.Print("  Installing OTel packages... ")
	if err := installPackages(venvPip, otelPythonPackages); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return
	}
	fmt.Println("done.")

	fmt.Print("  Running opentelemetry-bootstrap... ")
	venvPython := resolveVenvBinary(proj.Path, "python")
	if venvPython == "python" {
		venvPython = resolveVenvBinary(proj.Path, "python3")
	}
	if err := runOtelBootstrap(venvPython); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return
	}
	fmt.Println("done.")

	// Launch each entrypoint.
	fmt.Println()
	var startedServices []string
	var startedPIDs []int
	var startedLogs []string
	for _, ep := range p.Entrypoints {
		svcName := serviceNameFromEntrypoint(proj.Path, ep)
		epEnvVars := make(map[string]string, len(envVars))
		for k, v := range envVars {
			epEnvVars[k] = v
		}
		epEnvVars["OTEL_SERVICE_NAME"] = svcName

		logName := svcName + ".log"
		logPath := filepath.Join(proj.Path, logName)
		logFile, err := os.Create(logPath)
		if err != nil {
			fmt.Printf("    Failed to create log file %s: %v\n", logPath, err)
			continue
		}

		cmd := exec.Command(otelInstrument, pythonBin, ep)
		cmd.Dir = proj.Path
		cmd.Env = append(os.Environ(), envVarsToSlice(epEnvVars)...)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			logFile.Close()
			fmt.Printf("    Failed to start %s: %v\n", ep, err)
			continue
		}
		startedServices = append(startedServices, svcName)
		startedPIDs = append(startedPIDs, cmd.Process.Pid)
		startedLogs = append(startedLogs, logName)
	}

	// Wait for ports then print a combined summary.
	if len(startedPIDs) > 0 {
		time.Sleep(2 * time.Second)
		fmt.Println()
		for i, pid := range startedPIDs {
			port := detectListeningPort(pid)
			line := fmt.Sprintf("  %s (PID %d)", startedServices[i], pid)
			if port != "" {
				line += fmt.Sprintf(" → http://localhost:%s", port)
			}
			line += fmt.Sprintf("  [log: %s]", startedLogs[i])
			fmt.Println(line)
		}
	}

	// Poll Dynatrace for the services to appear.
	fmt.Println()
	fmt.Println("  Waiting for traffic — send requests to your services to generate traces and metrics.")
	waitForServices(p.EnvURL, p.PlatformToken, startedServices)
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

	// Build DQL query for all service names.
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
		"query":                       dql,
		"requestTimeoutMilliseconds":  10000,
		"maxResultRecords":            100,
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
		return nil
	}
	defer resp.Body.Close()

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
	return names
}

// commonEntrypoints are filenames commonly used as Python project entrypoints,
// checked in priority order.
var commonEntrypoints = []string{
	"main.py",
	"app.py",
	"run.py",
	"server.py",
	"manage.py",
	"wsgi.py",
	"asgi.py",
}

// serviceNameFromEntrypoint derives a human-readable OTEL_SERVICE_NAME from an
// entrypoint path relative to the project root.
//
// Examples:
//
//	"app.py"                in project "orderschnitzel" → "orderschnitzel"
//	"s-frontend/app.py"     in project "orderschnitzel" → "orderschnitzel-s-frontend"
//	"services/api/main.py"  in project "myapp"          → "myapp-api"
func serviceNameFromEntrypoint(projectPath, entrypoint string) string {
	projectName := filepath.Base(projectPath)

	dir := filepath.Dir(entrypoint)
	if dir == "." || dir == "" {
		// Entrypoint is in the project root — use project name directly.
		return projectName
	}

	// Use the immediate parent folder of the entrypoint as the service qualifier.
	// e.g. "s-frontend/app.py" → "s-frontend", "services/api/main.py" → "api"
	servicePart := filepath.Base(dir)
	return projectName + "-" + servicePart
}

// detectPythonEntrypoints finds Python entrypoint files in a project.
// Checks pyproject.toml scripts, common filenames in the project root, and
// common filenames in immediate subdirectories (for multi-service projects).
// Returns paths relative to the project root.
func detectPythonEntrypoints(projectPath string) []string {
	var entrypoints []string

	// Try pyproject.toml [project.scripts] or [tool.poetry.scripts].
	pyproject := filepath.Join(projectPath, "pyproject.toml")
	if data, err := os.ReadFile(pyproject); err == nil {
		if ep := parseEntrypointFromPyproject(string(data)); ep != "" {
			entrypoints = append(entrypoints, ep)
		}
	}
	if len(entrypoints) > 0 {
		return entrypoints
	}

	// Check common entrypoint filenames in the project root.
	for _, name := range commonEntrypoints {
		if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
			entrypoints = append(entrypoints, name)
		}
	}
	if len(entrypoints) > 0 {
		return entrypoints
	}

	// Check immediate subdirectories (e.g. s-frontend/app.py, s-order/app.py).
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "__pycache__" ||
			e.Name() == "node_modules" {
			continue
		}
		subDir := filepath.Join(projectPath, e.Name())
		for _, name := range commonEntrypoints {
			if _, err := os.Stat(filepath.Join(subDir, name)); err == nil {
				entrypoints = append(entrypoints, filepath.Join(e.Name(), name))
			}
		}
	}
	return entrypoints
}

// parseEntrypointFromPyproject extracts a script entrypoint from pyproject.toml content.
// Looks for patterns like `module:func` under [project.scripts] and converts to module path.
func parseEntrypointFromPyproject(content string) string {
	// Simple line-based scan for `name = "module:func"` or `name = "module.submod:func"`.
	inScripts := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inScripts = trimmed == "[project.scripts]" || trimmed == "[tool.poetry.scripts]"
			continue
		}
		if !inScripts {
			continue
		}
		// Parse `name = "module:func"`.
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		if colonIdx := strings.Index(val, ":"); colonIdx > 0 {
			// Convert module path to file: "myapp.main:run" → "myapp/main.py"
			modPath := val[:colonIdx]
			return strings.ReplaceAll(modPath, ".", "/") + ".py"
		}
	}
	return ""
}

// resolveVenvBinary finds a binary in the project's virtualenv bin directory.
// Returns the absolute path if found, otherwise returns the name for PATH lookup.
func resolveVenvBinary(projectPath, name string) string {
	for _, venvName := range []string{".venv", "venv", "env", ".env"} {
		binPath := filepath.Join(projectPath, venvName, "bin", name)
		if _, err := os.Stat(binPath); err == nil {
			return binPath
		}
		// Windows.
		binPath = filepath.Join(projectPath, venvName, "Scripts", name+".exe")
		if _, err := os.Stat(binPath); err == nil {
			return binPath
		}
	}
	return name
}

// detectProjectPip looks for a pip executable inside common virtualenv
// directories of a project.
func detectProjectPip(projectPath string) *pipCommand {
	for _, venvName := range []string{".venv", "venv", "env", ".env"} {
		pipPath := filepath.Join(projectPath, venvName, "bin", "pip")
		if _, err := os.Stat(pipPath); err == nil {
			return &pipCommand{name: pipPath}
		}
		// Windows layout.
		pipPath = filepath.Join(projectPath, venvName, "Scripts", "pip.exe")
		if _, err := os.Stat(pipPath); err == nil {
			return &pipCommand{name: pipPath}
		}
	}
	return nil
}

// InstallOtelPython sets up OpenTelemetry auto-instrumentation for Python
// applications. It detects Python projects on the machine and installs
// packages into the selected project's virtualenv.
//
// Parameters:
//   - envURL:        Dynatrace environment URL
//   - token:          API token (Ingest scope)
//   - platformToken:  Platform token (dt0s16.*) for DQL queries (optional)
//   - serviceName:    OTEL_SERVICE_NAME value (defaults to "my-service" if empty)
//   - dryRun:         when true, only print what would be done
func InstallOtelPython(envURL, token, platformToken, serviceName string, dryRun bool) error {
	apiURL := APIURL(envURL)

	if serviceName == "" {
		serviceName = "my-service"
	}

	envVars := generateOtelPythonEnvVars(apiURL, token, serviceName)

	if dryRun {
		fmt.Println("[dry-run] Would set up OpenTelemetry Python auto-instrumentation")
		fmt.Printf("  API URL:      %s\n", apiURL)
		fmt.Printf("  Service name: %s\n", serviceName)
		fmt.Println("  Packages to install (in project virtualenv):")
		fmt.Printf("    pip install %s\n", strings.Join(otelPythonPackages, " "))
		fmt.Println("    opentelemetry-bootstrap -a install")
		fmt.Println()
		fmt.Println("  Environment variables:")
		for k, v := range envVars {
			fmt.Printf("    %s=%s\n", k, v)
		}
		return nil
	}

	cyan := color.New(color.FgMagenta)
	sep := strings.Repeat("─", 60)

	fmt.Println()
	cyan.Println("  Dynatrace Python Auto-Instrumentation")
	fmt.Println("  " + sep)

	plan := DetectPythonPlan(apiURL, token)
	if plan == nil {
		printManualInstructions(envVars)
		return nil
	}

	// Show the plan.
	fmt.Println()
	cyan.Println("  Steps:")
	plan.PrintPlanSteps()

	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	plan.EnvURL = envURL
	plan.PlatformToken = platformToken
	plan.EnvVars = envVars

	fmt.Printf("\n  ── Python auto-instrumentation ──\n\n")
	plan.Execute()

	return nil
}
