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
func detectPython() (string, error) {
	for _, name := range []string{"python3", "python"} {
		path, err := exec.LookPath(name)
		if err != nil {
			logger.Debug("python candidate not found", "name", name)
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
			return path, nil
		}
		logger.Debug("python candidate is not Python 3", "path", path, "version", version)
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
	logger.Debug("running pip install", "cmd", pip.name, "args", strings.Join(args, " "))
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
	logger.Debug("running opentelemetry-bootstrap", "python", pythonPath)
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

func buildPythonInstrumentationPlan(proj ScannedProject, apiURL, token, envURL, platformToken string) *PythonInstrumentationPlan {
	entrypoints := detectPythonEntrypoints(proj.Path)
	if len(entrypoints) == 0 {
		fmt.Printf("  Skipping %s — no Python entrypoint found.\n", proj.Path)
		fmt.Println("    Looked for: pyproject.toml [project.scripts], or common files (main.py, app.py, run.py, server.py, manage.py, wsgi.py, asgi.py).")
		fmt.Println("    Add one of these files and re-run dtwiz.")
		return nil
	}

	needsVenv := detectProjectPip(proj.Path) == nil
	svcName := projectServiceName(proj.Path)
	envVars := generateOtelPythonEnvVars(apiURL, token, svcName)

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
		fmt.Println("     Create virtualenv (.venv)")
	}
	fmt.Printf("     pip install %s\n", strings.Join(otelPythonPackages, " "))
	fmt.Println("     opentelemetry-bootstrap -a install")
	for _, ep := range p.Entrypoints {
		svcName := serviceNameFromEntrypoint(p.Project.Path, ep)
		fmt.Printf("     opentelemetry-instrument python %s  (service: %s)\n", ep, svcName)
	}
}

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
		cmd.Env = append(os.Environ(), formatEnvVars(epEnvVars)...)
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
	}

	fmt.Println()
	fmt.Println("  Waiting for traffic — send requests to your services to generate traces and metrics.")
	waitForServices(p.EnvURL, p.PlatformToken, startedServices)
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
