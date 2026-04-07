package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/fatih/color"
)

// detectPython finds a usable Python 3 executable on the current PATH,
// preferring python3 over python.
// PythonProcess describes a detected running Python process.
type PythonProcess struct {
	PID     int
	Command string // full command line
	CWD     string // working directory of the process
}

// detectPython finds a usable Python 3 interpreter on the current PATH,
// preferring python3 over python when both are available.
func detectPython() (string, error) {
	for _, name := range []string{"python3", "python"} {
		logger.Debug("checking python interpreter on PATH", "candidate", name)
		path, err := exec.LookPath(name)
		if err != nil {
			logger.Debug("python candidate not found", "name", name)
			logger.Debug("python interpreter not found on PATH", "candidate", name, "error", err)
			continue
		}
		// Verify it's actually Python 3.
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			logger.Warn("python version check failed", "path", path, "err", err)
			continue
		}
		version := strings.TrimSpace(string(out))
		if strings.HasPrefix(version, "Python 3") {
			logger.Debug("python found", "path", path, "version", version)
			logger.Debug("python interpreter version probe failed", "candidate", name, "path", path, "error", err)
			continue
		}
		version := strings.TrimSpace(string(out))
		logger.Debug("python interpreter version probe succeeded", "candidate", name, "path", path, "version", version)
		if strings.HasPrefix(version, "Python 3") {
			logger.Debug("selected python interpreter", "candidate", name, "path", path)
			return path, nil
		}
		logger.Debug("python candidate is not Python 3", "path", path, "version", version)
	}
	logger.Debug("no usable python 3 interpreter found on PATH")
	return "", fmt.Errorf("Python 3 interpreter not found — install Python 3 and ensure either `python3` or `python` is in PATH")
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
	logger.Debug("running pip install", "cmd", pip.name, "args", strings.Join(args, " "))
	out, err := cmd.CombinedOutput()
// validatePythonPrerequisites checks that a Python 3 interpreter, pip, and venv are available.
func validatePythonPrerequisites() error {
	pythonBin, err := detectPython()
	if err != nil {
		return err
	}
	return nil
}

// runOtelBootstrap runs `opentelemetry-bootstrap -a install` to automatically
// install instrumentation libraries for all packages found in the environment.
// Output is suppressed unless the command fails.
func runOtelBootstrap(pythonPath string) error {
	cmd := exec.Command(pythonPath, "-m", "opentelemetry.instrumentation.bootstrap", "-a", "install")
	logger.Debug("running opentelemetry-bootstrap", "python", pythonPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		return fmt.Errorf("opentelemetry-bootstrap failed: %w", err)
	logger.Debug("validating python prerequisites", "python", pythonBin)
	if out, err := exec.Command(pythonBin, "-m", "pip", "--version").CombinedOutput(); err != nil {
		logger.Debug("python pip check failed", "python", pythonBin, "output", strings.TrimSpace(string(out)), "error", err)
		return fmt.Errorf("pip is not available for the detected Python 3 interpreter (%s): %w\n    %s", pythonBin, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("python pip check succeeded", "python", pythonBin)
	if out, err := exec.Command(pythonBin, "-m", "venv", "--help").CombinedOutput(); err != nil {
		logger.Debug("python venv check failed", "python", pythonBin, "output", strings.TrimSpace(string(out)), "error", err)
		return fmt.Errorf("venv module is not available for the detected Python 3 interpreter (%s) — on Debian/Ubuntu run: apt install python3-venv: %w\n    %s", pythonBin, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("python venv check succeeded", "python", pythonBin)
	return nil
}

// generateOtelPythonEnvVars returns the OTEL_* environment variables required
// for auto-instrumentation to export to Dynatrace.
func generateOtelPythonEnvVars(apiURL, token, serviceName string) map[string]string {
	envVars := generateBaseOtelEnvVars(apiURL, token, serviceName)
	envVars["OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED"] = "true"
	return envVars
}

// detectPythonProcesses finds running Python processes (excluding the current
// process and common system Python processes).
func detectPythonProcesses() []DetectedProcess {
	return detectProcesses("python", []string{"pip ", "setup.py"})
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

func detectPythonProjects() []ScannedProject {
	return scanProjectDirs(pythonProjectMarkers, nil)
}

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

// PythonInstrumentationPlan captures all the user's choices for Python
// auto-instrumentation so that detection/prompting and execution can happen
// at different times (e.g. choices upfront, execution after collector install).
type PythonInstrumentationPlan struct {
	Project       ScannedProject
	Entrypoints   []string
	NeedsVenv     bool
	EnvVars       map[string]string
	EnvURL        string
	PlatformToken string
}

func (p *PythonInstrumentationPlan) Runtime() string { return "Python" }

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
	logger.Debug("selected python project for instrumentation", "project", proj.Path)
	entrypoints := detectPythonEntrypoints(proj.Path)
	if len(entrypoints) == 0 {
		fmt.Printf("  Skipping %s — no Python entrypoint found.\n", proj.Path)
		fmt.Println("    Looked for: pyproject.toml [project.scripts], or common files (main.py, app.py, run.py, server.py, manage.py, wsgi.py, asgi.py).")
		fmt.Println("    Add one of these files and re-run dtwiz.")
		return nil
	}

	needsVenv := !isVenvHealthy(proj.Path)
	logger.Debug("python project venv evaluation complete", "project", proj.Path, "needs_venv", needsVenv, "entrypoints", entrypoints)

	// Generate env vars using the API URL derived from what the caller provides.
	envVars := generateOtelPythonEnvVars(apiURL, token, "my-service")

	return &PythonInstrumentationPlan{
		Project:       proj,
		Entrypoints:   entrypoints,
		NeedsVenv:     needsVenv,
		EnvVars:       envVars,
		EnvURL:        envURL,
		PlatformToken: platformToken,
	}
}

func DetectPythonPlan(apiURL, token string) *PythonInstrumentationPlan {
	if _, err := detectPython(); err != nil {
		logger.Debug("python not found on PATH", "skipping Python instrumentation")
		return nil
	}

	projects, processes := runInParallel(detectPythonProjects, detectPythonProcesses)
	matchProcessesToProjects(projects, processes)

	if len(projects) == 0 {
		logger.Debug("no Python projects detected", "skipping Python instrumentation")
		return nil
	}

	sel := promptProjectSelection("Python", projects)
	if sel == nil {
		return nil
	}

	return buildPythonInstrumentationPlan(*sel, apiURL, token, "", "")
}

// installProjectDeps installs the project's own dependencies using the
// appropriate file: requirements.txt, Pipfile, pyproject.toml, or setup.py.
// Returns the description of what was installed (for logging), or "" if nothing found.
func installProjectDeps(pip *pipCommand, projectPath string) (string, error) {
	// requirements.txt — most common.
	reqFile := filepath.Join(projectPath, "requirements.txt")
	if _, err := os.Stat(reqFile); err == nil {
		args := append(append([]string{}, pip.args...), "install", "-r", reqFile)
		full := pip.name + " " + strings.Join(args, " ")
		fmt.Printf("\n    %s\n", full)
		cmd := exec.Command(pip.name, args...)
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			os.Stdout.Write(out)
			return "", fmt.Errorf("pip install -r requirements.txt failed: %w\n    command: %s", err, full)
		}
		return "requirements.txt", nil
	}

	// pyproject.toml — pip install .
	pyproject := filepath.Join(projectPath, "pyproject.toml")
	if _, err := os.Stat(pyproject); err == nil {
		args := append(append([]string{}, pip.args...), "install", ".")
		full := pip.name + " " + strings.Join(args, " ")
		fmt.Printf("\n    %s\n", full)
		cmd := exec.Command(pip.name, args...)
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			os.Stdout.Write(out)
			return "", fmt.Errorf("pip install . (pyproject.toml) failed: %w\n    command: %s", err, full)
		}
		return "pyproject.toml", nil
	}

	// setup.py — pip install .
	setupPy := filepath.Join(projectPath, "setup.py")
	if _, err := os.Stat(setupPy); err == nil {
		args := append(append([]string{}, pip.args...), "install", ".")
		full := pip.name + " " + strings.Join(args, " ")
		fmt.Printf("\n    %s\n", full)
		cmd := exec.Command(pip.name, args...)
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			os.Stdout.Write(out)
			return "", fmt.Errorf("pip install . (setup.py) failed: %w\n    command: %s", err, full)
		}
		return "setup.py", nil
	}

	return "", nil
}

// projectDepsDescription returns a human-readable description of how project
// dependencies would be installed, or "" if no supported file is found.
func projectDepsDescription(projectPath string) string {
	if _, err := os.Stat(filepath.Join(projectPath, "requirements.txt")); err == nil {
		return "pip install -r requirements.txt"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "pyproject.toml")); err == nil {
		return "pip install . (pyproject.toml)"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "setup.py")); err == nil {
		return "pip install . (setup.py)"
	}
	return ""
}

// PrintPlanSteps prints the Python instrumentation steps for inclusion in a
// combined plan preview.
func (p *PythonInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project: %s\n", p.Project.Path)
	if len(p.Project.RunningProcessIDs) > 0 {
		pidStrs := make([]string, len(p.Project.RunningProcessIDs))
		for i, pid := range p.Project.RunningProcessIDs {
			pidStrs[i] = strconv.Itoa(pid)
		}
		fmt.Printf("     Stop running processes (PIDs: %s)\n", strings.Join(pidStrs, ", "))
	}
	if p.NeedsVenv {
		if detectProjectVenvDir(p.Project.Path) != "" {
			fmt.Println("     Recreate virtualenv (.venv) — existing venv is from a different environment")
		} else {
			fmt.Println("     Create virtualenv (.venv)")
		}
	}
	if desc := projectDepsDescription(p.Project.Path); desc != "" {
		fmt.Printf("     %s\n", desc)
	}
	fmt.Printf("     pip install %s\n", strings.Join(otelPythonPackages, " "))
	fmt.Println("     opentelemetry-bootstrap -a install")
	for _, ep := range p.Entrypoints {
		svcName := serviceNameFromEntrypoint(p.Project.Path, ep)
		fmt.Printf("     opentelemetry-instrument python %s  (service: %s)\n", ep, svcName)
	}
}

func confirmRecreateVirtualenv(venvDir string) (bool, error) {
	prompt := fmt.Sprintf(
		"  Existing virtualenv at %s is not usable.\n  A working virtualenv is required to install Python dependencies, add instrumentation packages, and start OTLP ingest reliably.\n  Remove it and recreate it?",
		venvDir,
	)
	return confirmProceed(prompt)
}

func removeStaleVirtualenv(venvDir string) (bool, error) {
	confirmed, err := confirmRecreateVirtualenv(venvDir)
	if err != nil {
		return false, err
	}
	if !confirmed {
		return false, nil
	}
	if err := os.RemoveAll(venvDir); err != nil {
		return false, fmt.Errorf("remove stale virtualenv %s: %w", venvDir, err)
	}
	return true, nil
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

	if len(proj.RunningProcessIDs) > 0 {
		fmt.Print("  Stopping running processes... ")
		stopProcesses(proj.RunningProcessIDs)
		fmt.Println("done.")
	}

	// Create venv if needed (no venv exists, or the existing one is unhealthy
	// because it was created on a different machine / with a removed Python).
	if p.NeedsVenv {
		if venvDir := detectProjectVenvDir(proj.Path); venvDir != "" {
			removed, err := removeStaleVirtualenv(venvDir)
			if err != nil {
				fmt.Println("failed.")
				fmt.Printf("    %v\n", err)
				return
			}
			if !removed {
				fmt.Println("  Cancelled: Python auto-instrumentation needs a working virtualenv to install packages and start OTLP ingest reliably.")
				return
			}
		}
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
		pythonBin = resolveVenvBinary(proj.Path, "python")
		if pythonBin == "" {
			pythonBin = "python3"
		}
	}

	// Install project dependencies before OTel packages so bootstrap can detect them.
	fmt.Print("  Installing project dependencies... ")
	installed, err := installProjectDeps(venvPip, proj.Path)
	if err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return
	}
	if installed != "" {
		fmt.Printf("done (%s).\n", installed)
	} else {
		fmt.Println("skipped (no requirements.txt, pyproject.toml, or setup.py found).")
	}

	fmt.Print("  Installing OTel packages... ")
	if err := installPackages(venvPip, otelPythonPackages); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return
	}
	fmt.Println("done.")

	// resolve now that the otel packages are installed
	otelInstrument = resolveVenvBinary(proj.Path, "opentelemetry-instrument")

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

	fmt.Print("  Verifying framework instrumentations... ")
	if err := ensureFrameworkInstrumentations(venvPython, venvPip); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return
	}
	fmt.Println("done.")

	// Launch each entrypoint.
	fmt.Println()
	var procs []*ManagedProcess
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

		cmd := exec.Command(venvPython, otelInstrument, pythonBin, ep)
		cmd.Dir = proj.Path
		cmd.Env = append(os.Environ(), formatEnvVars(epEnvVars)...)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			logFile.Close()
		cmd.Env = append(os.Environ(), envVarsToSlice(epEnvVars)...)

		mp, err := StartManagedProcess(svcName, logName, cmd, logFile)
		if err != nil {
			fmt.Printf("    Failed to start %s: %v\n", ep, err)
			continue
		}
		procs = append(procs, mp)
	}

	if len(startedPIDs) > 0 {
		time.Sleep(2 * time.Second)
		fmt.Println()
		for i, pid := range startedPIDs {
			port := detectProcessListeningPort(pid)
			line := fmt.Sprintf("  %s (PID %d)", startedServices[i], pid)
			if port != "" {
				line += fmt.Sprintf(" → http://localhost:%s", port)
			}
			line += fmt.Sprintf("  [log: %s]", startedLogs[i])
			fmt.Println(line)
		}
	startedServices, startedPIDs := PrintProcessSummary(procs, 2*time.Second)
	_ = startedPIDs

	if len(startedServices) == 0 {
		fmt.Println()
		fmt.Println("  No services are running — check the logs above for errors.")
		return
	}

	fmt.Println()
	fmt.Println("  Waiting for traffic — send requests to your services to generate traces and metrics.")
	waitForServices(p.EnvURL, p.PlatformToken, startedServices, false)
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
	dql := fmt.Sprintf("smartscapeNodes SERVICE, from: now()-1m | filter %s", strings.Join(conditions, " or "))
	fmt.Printf("  Querying Dynatrace for services with:\n    %s\n", dql)

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
			logger.Debug("python entrypoint found", "file", name)
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

// detectProjectVenvDir returns the first supported virtualenv directory found
// in the project, or "" if none exists.
func detectProjectVenvDir(projectPath string) string {
	for _, venvName := range []string{".venv", "venv", "env", ".env"} {
		venvDir := filepath.Join(projectPath, venvName)
		info, err := os.Stat(venvDir)
		if err == nil && info.IsDir() {
			logger.Debug("found project virtualenv directory", "project", projectPath, "venv_dir", venvDir)
			return venvDir
		}
	}
	logger.Debug("no project virtualenv directory found", "project", projectPath)
	return ""
}

// detectProjectPip returns a pipCommand that invokes pip via `python -m pip`
// using the virtualenv's own Python binary. This avoids executing pip scripts
// directly, which can fail with ENOENT when a venv's pip shebang points to a
// Python interpreter that no longer exists at that path (common on macOS with
// Homebrew or after Python upgrades).
func detectProjectPip(projectPath string) *pipCommand {
	for _, venvName := range []string{".venv", "venv", "env", ".env"} {
		// Unix: prefer python, fall back to python3.
		for _, pyName := range []string{"python", "python3"} {
			pyPath := filepath.Join(projectPath, venvName, "bin", pyName)
			if _, err := os.Stat(pyPath); err == nil {
				logger.Debug("resolved project pip command", "project", projectPath, "python", pyPath, "venv", venvName)
				return &pipCommand{name: pyPath, args: []string{"-m", "pip"}}
			}
		}
		// Windows: prefer python.exe, fall back to python3.exe.
		for _, pyName := range []string{"python.exe", "python3.exe"} {
			pyPath := filepath.Join(projectPath, venvName, "Scripts", pyName)
			if _, err := os.Stat(pyPath); err == nil {
				logger.Debug("resolved project pip command", "project", projectPath, "python", pyPath, "venv", venvName)
				return &pipCommand{name: pyPath, args: []string{"-m", "pip"}}
			}
		}
	}
	logger.Debug("could not resolve project pip command", "project", projectPath)
	return nil
}

// isVenvHealthy returns true if the project has a virtualenv whose Python
// binary is actually executable. A venv can exist on disk but be unhealthy when
// it was created on a different machine or with a Python version that has since
// been removed (the venv Python is typically a symlink to the system Python
// that created it — if that binary is gone the whole venv is broken).
func isVenvHealthy(projectPath string) bool {
	pip := detectProjectPip(projectPath)
	if pip == nil {
		logger.Debug("project virtualenv is not healthy", "project", projectPath, "reason", "no_python_binary_found")
		return false
	}
	// Run `python --version` — the lightest possible smoke-test.
	out, err := exec.Command(pip.name, "--version").CombinedOutput()
	if err != nil {
		logger.Debug("project virtualenv health check failed", "project", projectPath, "python", pip.name, "output", strings.TrimSpace(string(out)), "error", err)
		return false
	}
	logger.Debug("project virtualenv health check succeeded", "project", projectPath, "python", pip.name, "output", strings.TrimSpace(string(out)))
	return true
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
	if err := validatePythonPrerequisites(); err != nil {
		return err
	}

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
		for _, line := range formatPrintableEnvVars(envVars) {
			fmt.Printf("    %s\n", line)
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
