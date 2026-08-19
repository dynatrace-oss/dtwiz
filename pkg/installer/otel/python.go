package otel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

func generateOtelPythonEnvVars(collectorEndpoint, serviceName string) map[string]string {
	envVars := generateBaseOtelEnvVars(collectorEndpoint, serviceName)
	envVars["OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED"] = "true"
	return envVars
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

type PythonInstrumentationPlan struct {
	Project       ScannedProject
	Entrypoints   []string
	NeedsVenv     bool
	EnvVars       map[string]string
	EnvURL        string
	PlatformToken string
}

func (p *PythonInstrumentationPlan) Runtime() string { return "Python" }

func buildPythonInstrumentationPlan(proj ScannedProject, collectorEndpoint, envURL, platformToken string) *PythonInstrumentationPlan {
	entrypoints := detectPythonEntrypoints(proj.Path)
	if len(entrypoints) == 0 {
		fmt.Printf("  Skipping %s — no Python entrypoint found.\n", proj.Path)
		fmt.Println("    Looked for: pyproject.toml [project.scripts], or common files (main.py, app.py, run.py, server.py, manage.py, wsgi.py, asgi.py).")
		fmt.Println("    Add one of these files and re-run dtwiz.")
		return nil
	}

	needsVenv := !isVenvHealthy(proj.Path)
	logger.Debug("python project venv evaluation complete", "project", proj.Path, "needs_venv", needsVenv, "entrypoints", entrypoints)

	svcName := projectServiceName(proj.Path)
	envVars := generateOtelPythonEnvVars(collectorEndpoint, svcName)

	return &PythonInstrumentationPlan{
		Project:       proj,
		Entrypoints:   entrypoints,
		NeedsVenv:     needsVenv,
		EnvVars:       envVars,
		EnvURL:        envURL,
		PlatformToken: platformToken,
	}
}

func DetectPythonPlan(collectorEndpoint string) *PythonInstrumentationPlan {
	return DetectPythonPlanFromPath("", collectorEndpoint)
}

func DetectPythonPlanFromPath(projectPath, collectorEndpoint string) *PythonInstrumentationPlan {
	if _, err := DetectPython(); err != nil {
		return nil
	}
	return detectPythonPlanWithConfirmedRuntime(projectPath, collectorEndpoint)
}

// detectPythonPlanWithConfirmedRuntime builds the plan assuming Python is already confirmed usable.
// Call this when DetectPython() or validatePythonPrerequisites() has already run in the same invocation.
func detectPythonPlanWithConfirmedRuntime(projectPath, collectorEndpoint string) *PythonInstrumentationPlan {
	if projectPath != "" {
		projects := []ScannedProject{{Path: projectPath}}
		processes := detectPythonProcesses()
		matchProcessesToProjects(projects, processes)
		return buildPythonInstrumentationPlan(projects[0], collectorEndpoint, "", "")
	}

	projects, processes := runInParallel(
		func() []ScannedProject { return detectPythonProjects(defaultScanRoots()) },
		detectPythonProcesses,
	)
	matchProcessesToProjects(projects, processes)

	if len(projects) == 0 {
		logger.Debug("no Python projects detected — no Python source files or running processes found")
		return nil
	}

	sel := promptProjectSelection("Python", projects)
	if sel == nil {
		return nil
	}

	return buildPythonInstrumentationPlan(*sel, collectorEndpoint, "", "")
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

func (p *PythonInstrumentationPlan) Execute() error {
	proj := p.Project
	envVars := p.EnvVars

	venvPip := detectProjectPip(proj.Path)
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
		if venvDir := detectProjectVenvDir(proj.Path); venvDir != "" {
			removed, err := removeStaleVirtualenv(venvDir)
			if err != nil {
				fmt.Println("failed.")
				fmt.Printf("    %v\n", err)
				return fmt.Errorf("removing stale virtualenv: %w", err)
			}
			if !removed {
				fmt.Println("  Cancelled: Python auto-instrumentation needs a working virtualenv to install packages and start OTLP ingest reliably.")
				return fmt.Errorf("virtualenv removal cancelled")
			}
		}
		fmt.Print("  Creating virtualenv... ")
		pythonPath, err := DetectPython()
		if err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			return err
		}
		venvDir := filepath.Join(proj.Path, ".venv")
		logger.Debug("creating virtualenv", "python", pythonPath, "venv_dir", venvDir)
		cmd := exec.Command(pythonPath, "-m", "venv", venvDir)
		cmd.Dir = proj.Path
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("failed.")
			os.Stdout.Write(out)
			return fmt.Errorf("creating virtualenv: %w", err)
		}
		fmt.Println("done.")
		venvPip = detectProjectPip(proj.Path)
		if venvPip == nil {
			fmt.Println("    Could not find pip in new virtualenv.")
			return fmt.Errorf("pip not found in new virtualenv")
		}
		pythonBin = resolveVenvBinary(proj.Path, "python")
		if pythonBin == "" {
			pythonBin = "python3"
		}
	}

	fmt.Print("  Installing project dependencies... ")
	installed, err := installProjectDeps(venvPip, proj.Path)
	if err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return fmt.Errorf("installing project dependencies: %w", err)
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
		return fmt.Errorf("installing OTel packages: %w", err)
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
		return fmt.Errorf("running opentelemetry-bootstrap: %w", err)
	}
	fmt.Println("done.")

	fmt.Print("  Verifying framework instrumentations... ")
	if err := ensureFrameworkInstrumentations(venvPython, venvPip); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return fmt.Errorf("verifying framework instrumentations: %w", err)
	}
	fmt.Println("done.")

	otelInstrument := resolveVenvBinary(proj.Path, "opentelemetry-instrument")

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

		// On Unix/macOS, otelInstrument is a Python script whose shebang may point
		// to a stale path after venv recreation. Invoke it via venvPython so Python
		// reads the script content directly, bypassing the shebang entirely.
		// On Windows, pip installs a Portable Executable .exe wrapper that must be called directly.
		// When not found in the venv (bare name), call directly and rely on PATH.
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" || !filepath.IsAbs(otelInstrument) {
			logger.Debug("launching instrumented python process", "instrument", otelInstrument, "python", pythonBin, "entrypoint", ep)
			cmd = exec.Command(otelInstrument, pythonBin, ep)
		} else {
			logger.Debug("launching instrumented python process", "venv_python", venvPython, "instrument", otelInstrument, "python", pythonBin, "entrypoint", ep)
			cmd = exec.Command(venvPython, otelInstrument, pythonBin, ep)
		}
		cmd.Dir = proj.Path
		cmd.Env = append(os.Environ(), formatEnvVars(epEnvVars)...)

		mp, err := StartManagedProcess(svcName, logName, ep, cmd, logFile)
		if err != nil {
			fmt.Printf("    Failed to start %s: %v\n", ep, err)
			continue
		}
		procs = append(procs, mp)
	}

	startedServices, _ := PrintProcessSummary(procs, processSettleDelay)

	if len(startedServices) == 0 {
		fmt.Println()
		fmt.Println("  No services are running — check the logs above for errors.")
		return fmt.Errorf("no services started — all processes failed to start")
	}
	if len(startedServices) < len(procs) {
		return fmt.Errorf("%d of %d service(s) failed to start — check the logs above for errors", len(procs)-len(startedServices), len(procs))
	}

	return nil
}

func InstallOtelPython(envURL, token, platformToken, serviceName, projectPath string, dryRun bool) error {
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err != nil {
			return fmt.Errorf("project path not found: %s", projectPath)
		}
	}
	if _, err := validatePythonPrerequisites(); err != nil {
		return err
	}

	collectorEndpoint := fmt.Sprintf("http://localhost:%d", otlpHTTPPortFromConfig(findExistingCollectorConfig()))
	if serviceName == "" {
		serviceName = "my-service"
	}

	envVars := generateOtelPythonEnvVars(collectorEndpoint, serviceName)

	if dryRun {
		fmt.Println("[dry-run] Would set up OpenTelemetry Python auto-instrumentation")
		fmt.Printf("  Collector endpoint: %s\n", collectorEndpoint)
		fmt.Printf("  Service name:       %s\n", serviceName)
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

	sep := strings.Repeat("─", 60)

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Python Auto-Instrumentation")
	fmt.Println("  " + sep)

	plan := detectPythonPlanWithConfirmedRuntime(projectPath, collectorEndpoint)
	if plan == nil {
		printManualInstructions(envVars)
		return installer.ErrInstallCancelled
	}

	fmt.Println()
	display.ColorMessage.Println("  Steps:")
	plan.PrintPlanSteps()

	ok, err := installer.ConfirmProceed("  Proceed with installation?")
	if err != nil {
		return err
	}
	if !ok {
		return installer.ErrInstallCancelled
	}

	plan.EnvURL = envURL
	plan.PlatformToken = platformToken
	plan.EnvVars = envVars

	fmt.Printf("\n  ── Python auto-instrumentation ──\n\n")
	return plan.Execute()
}
