package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

func generateOtelPythonEnvVars(apiURL, token, serviceName string) map[string]string {
	envVars := generateBaseOtelEnvVars(apiURL, token, serviceName)
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

func buildPythonInstrumentationPlan(proj ScannedProject, apiURL, token, envURL, platformToken string) *PythonInstrumentationPlan {
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
	return DetectPythonPlanFromPath("", apiURL, token)
}

func DetectPythonPlanFromPath(projectPath, apiURL, token string) *PythonInstrumentationPlan {
	if _, err := detectPython(); err != nil {
		return nil
	}

	if projectPath != "" {
		projects := []ScannedProject{{Path: projectPath}}
		processes := detectPythonProcesses()
		matchProcessesToProjects(projects, processes)
		return buildPythonInstrumentationPlan(projects[0], apiURL, token, "", "")
	}

	projects, processes := runInParallel(detectPythonProjects, detectPythonProcesses)
	matchProcessesToProjects(projects, processes)

	if len(projects) == 0 {
		logger.Debug("no Python projects detected — no Python source files or running processes found")
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

func (p *PythonInstrumentationPlan) Execute() {
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
			cmd = exec.Command(otelInstrument, pythonBin, ep)
		} else {
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
		return
	}

}

func InstallOtelPython(envURL, token, platformToken, serviceName, projectPath string, dryRun bool) error {
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err != nil {
			return fmt.Errorf("project path not found: %s", projectPath)
		}
	}
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

	plan := DetectPythonPlanFromPath(projectPath, apiURL, token)
	if plan == nil {
		printManualInstructions(envVars)
		return nil
	}

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
