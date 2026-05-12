package installer

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	},
}

var otelJavaAgentURL = "https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar"

var javaProjectMarkers = []string{
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
	"gradlew",
	"gradlew.bat",
	"mvnw.cmd",
	".mvn",
}

func detectJavaProjects() []ScannedProject {
	return scanProjectDirs(javaProjectMarkers, nil)
}

func detectJavaProcesses() []DetectedProcess {
	processes := detectProcesses("java", nil)
	logger.Debug("detected java processes", "count", len(processes))
	return processes
}

// scanJavaProjects scans for Java projects, enriches process data, and matches
// running processes to their projects. Returns the list of detected projects.
func scanJavaProjects() []ScannedProject {
	projects, processes := runInParallel(detectJavaProjects, detectJavaProcesses)
	processes = enrichProcessesWithJPS(processes)
	matchProcessesToProjects(projects, processes)
	return projects
}

func javaAgentPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory for agent path: %w", err)
	}
	return filepath.Join(homeDir, ".opentelemetry", "java", "opentelemetry-javaagent.jar"), nil
}

func downloadJavaAgent() (string, error) {
	destPath, err := javaAgentPath()
	if err != nil {
		return "", err
	}
	logger.Debug("downloading java agent", "url", otelJavaAgentURL, "dest", destPath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", fmt.Errorf("creating agent directory: %w", err)
	}

	resp, err := httpClient.Get(otelJavaAgentURL) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("downloading agent from %s: %w", otelJavaAgentURL, err)
	}
	defer resp.Body.Close()
	logger.Debug("java agent download response", "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading agent from %s: HTTP %d", otelJavaAgentURL, resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("creating agent file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("writing agent file: %w", err)
	}

	display.PrintStatusLine("download", "OpenTelemetry Java agent... done.", display.ColorOK)
	return destPath, nil
}

func displayInstrumentedCmd(ep JavaEntrypoint, agentPath string) string {
	if strings.HasPrefix(ep.Command, "java ") {
		return "java -javaagent:" + agentPath + " " + strings.TrimPrefix(ep.Command, "java ")
	}
	return `JAVA_TOOL_OPTIONS="-javaagent:` + agentPath + `" ` + ep.Command
}

func buildInstrumentedCmd(ep JavaEntrypoint, agentPath, projectPath string, envVars map[string]string) *exec.Cmd {
	var cmd *exec.Cmd

	if strings.HasPrefix(ep.Command, "java ") {
		// "java -jar /path/to/app.jar" → exec java with -javaagent inserted.
		rest := strings.TrimPrefix(ep.Command, "java ")
		args := append([]string{"-javaagent:" + agentPath}, strings.Fields(rest)...)
		cmd = exec.Command("java", args...)
	} else {
		// Wrapper commands: pass JAVA_TOOL_OPTIONS via env.
		fields := strings.Fields(ep.Command)
		if len(fields) == 0 {
			cmd = exec.Command(ep.Command)
		} else if strings.HasSuffix(fields[0], ".cmd") || strings.HasSuffix(fields[0], ".bat") {
			// Windows wrapper: must be invoked via cmd /c.
			cmd = exec.Command("cmd", append([]string{"/c", fields[0]}, fields[1:]...)...)
		} else {
			cmd = exec.Command(fields[0], fields[1:]...)
		}
		finalEnvVars := make(map[string]string, len(envVars)+1)
		for k, v := range envVars {
			finalEnvVars[k] = v
		}
		finalEnvVars["JAVA_TOOL_OPTIONS"] = "-javaagent:" + agentPath
		cmd.Env = append(os.Environ(), formatEnvVars(finalEnvVars)...)
		cmd.Dir = projectPath
		return cmd
	}

	cmd.Dir = projectPath
	cmd.Env = append(os.Environ(), formatEnvVars(envVars)...)
	return cmd
}

type SubModulePlan struct {
	Name          string
	Path          string
	LaunchCommand string
	EnvVars       map[string]string
}

type JavaInstrumentationPlan struct {
	Project           ScannedProject
	EnvVars           map[string]string
	EnvURL            string
	Token             string
	EntrypointCommand string
	BuildCommand      string
	SubModules        []SubModulePlan
}

func (p *JavaInstrumentationPlan) Runtime() string { return "Java" }

// DetectJavaPlan builds a JavaInstrumentationPlan by scanning for projects and prompting
// the user to select one. Returns nil if no project is selected.
func DetectJavaPlan(envURL, token string) *JavaInstrumentationPlan {
	if _, err := exec.LookPath("java"); err != nil {
		logger.Debug("java not found on PATH, skipping Java instrumentation")
		return nil
	}

	projects := scanJavaProjects()
	if len(projects) == 0 {
		logger.Debug("no Java projects detected, skipping Java instrumentation")
		return nil
	}

	sel := promptProjectSelection("Java", projects)
	if sel == nil {
		return nil
	}

	proj := detectedProject{ScannedProject: *sel, Runtime: "Java"}
	plan := buildJavaInstrumentationPlan(proj, APIURL(envURL), token, envURL)
	if jp, ok := plan.(*JavaInstrumentationPlan); ok {
		return jp
	}
	return nil
}

func (p *JavaInstrumentationPlan) PrintPlanSteps() {
	agentPath, err := javaAgentPath()
	if err != nil {
		agentPath = "opentelemetry-javaagent.jar"
	}
	fmt.Printf("     Project:    %s\n", p.Project.Path)
	if p.EntrypointCommand != "" {
		ep := JavaEntrypoint{Command: p.EntrypointCommand}
		fmt.Printf("     Launch:     %s\n", displayInstrumentedCmd(ep, agentPath))
	} else {
		fmt.Printf("     Launch:     (entrypoint will be detected at execution time)\n")
	}
	fmt.Printf("     Agent JAR:  %s\n", otelJavaAgentURL)
	for _, line := range formatPrintableEnvVars(p.EnvVars) {
		fmt.Printf("     %s\n", line)
	}
}

func buildJavaInstrumentationPlan(proj detectedProject, apiURL, token, envURL string) InstrumentationPlan {
	if mm := detectMultiModule(proj.Path); mm != nil {
		return buildMultiModulePlan(mm, proj.ScannedProject, apiURL, token, envURL)
	}
	svcName := projectServiceName(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)
	entrypoints := detectJavaEntrypoints(proj.Path)
	var entrypointCmd string
	if len(entrypoints) > 0 {
		ep := promptEntrypointSelection(entrypoints)
		if ep != nil {
			entrypointCmd = ep.Command
		}
	}
	return &JavaInstrumentationPlan{
		Project:           proj.ScannedProject,
		EnvVars:           envVars,
		EnvURL:            envURL,
		Token:             token,
		EntrypointCommand: entrypointCmd,
	}
}

func (p *JavaInstrumentationPlan) Execute() {
	if len(p.SubModules) > 0 {
		if len(p.Project.RunningProcessIDs) > 0 {
			fmt.Print("  Stopping running processes... ")
			stopProcesses(p.Project.RunningProcessIDs)
			fmt.Println("done.")
		}
		if err := p.executeMultiModule(); err != nil {
			logger.Debug("multi-module execution failed", "error", err)
		}
		return
	}

	agentPath, err := downloadJavaAgent()
	if err != nil {
		display.PrintStatusLine("error", fmt.Sprintf("failed to download agent: %v", err), display.ColorError)
		return
	}

	if len(p.Project.RunningProcessIDs) > 0 {
		logger.Debug("stopping running java processes", "pids", p.Project.RunningProcessIDs)
		stopProcesses(p.Project.RunningProcessIDs)
	}

	var ep *JavaEntrypoint
	if p.EntrypointCommand != "" {
		e := JavaEntrypoint{Command: p.EntrypointCommand}
		ep = &e
	} else {
		entrypoints := detectJavaEntrypoints(p.Project.Path)
		if len(entrypoints) == 0 {
			logger.Debug("no entrypoints found at execute time", "project", p.Project.Path)
			display.PrintStatusLine("error", "no runnable entrypoint detected — build the project first", display.ColorError)
			return
		}
		ep = promptEntrypointSelection(entrypoints)
		if ep == nil {
			return
		}
	}

	svcName := projectServiceName(p.Project.Path)
	displayCmd := displayInstrumentedCmd(*ep, agentPath)

	logPath := filepath.Join(p.Project.Path, svcName+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		display.PrintStatusLine("error", fmt.Sprintf("failed to create log file: %v", err), display.ColorError)
		return
	}

	logger.Debug("launching instrumented java process", "cmd", displayCmd, "dir", p.Project.Path)
	cmd := buildInstrumentedCmd(*ep, agentPath, p.Project.Path, p.EnvVars)

	proc, err := StartManagedProcess(svcName, svcName+".log", "", cmd, logFile)
	if err != nil {
		display.PrintStatusLine("error", fmt.Sprintf("failed to start process: %v", err), display.ColorError)
		return
	}
	proc.portDetector = func(pid int) string { return detectJavaListeningPort(pid, p.Project.Path) }

	aliveNames, _ := PrintProcessSummary([]*ManagedProcess{proc}, processSettleDelay)
	if len(aliveNames) == 0 {
		display.PrintStatusLine("error", "No services are running — check the logs above for errors.", display.ColorError)
		return
	}

	updateOtelCollectorIfPresent(p.EnvURL, p.Token, false)
}

// InstallOtelJava is the main entry point for the `dtwiz install otel-java` command.
func InstallOtelJava(envURL, token, serviceName, projectPath string, dryRun bool) error {
	if _, err := validateJavaPrerequisites(); err != nil {
		return err
	}

	apiURL := APIURL(envURL)

	if serviceName == "" {
		serviceName = "my-service"
	}

	var envVars map[string]string

	var proj ScannedProject
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err != nil {
			return fmt.Errorf("project path not found: %s", projectPath)
		}
		proj = ScannedProject{Path: projectPath}
	} else {
		projects := scanJavaProjects()
		logger.Debug("detected java projects", "count", len(projects))
		if len(projects) == 0 {
			display.PrintStatusLine("error", "no Java projects detected", display.ColorError)
			return fmt.Errorf("no Java projects detected")
		}

		sel := promptProjectSelection("Java", projects)
		if sel == nil {
			logger.Debug("user skipped project selection")
			return nil
		}
		proj = *sel
	}

	if serviceName == "my-service" {
		serviceName = projectServiceName(proj.Path)
	}
	logger.Debug("java instrumentation target", "project", proj.Path, "service", serviceName)
	envVars = generateBaseOtelEnvVars(apiURL, token, serviceName)

	if mm := detectMultiModule(proj.Path); mm != nil {
		logger.Debug("multi-module project detected", "tool", mm.BuildTool)
		plan := buildMultiModulePlan(mm, proj, apiURL, token, envURL)

		fmt.Println()
		display.Header("Java multi-module instrumentation plan")
		fmt.Printf("  Project:       %s\n", proj.Path)
		if plan.BuildCommand != "" {
			fmt.Printf("  Build command: %s\n", plan.BuildCommand)
		}
		fmt.Println()
		fmt.Println("  Modules:")
		for _, sub := range plan.SubModules {
			fmt.Printf("    [%s]  %s\n", sub.Name, sub.LaunchCommand)
		}
		fmt.Println()

		if dryRun {
			display.PrintStatusLine("dry-run", "no changes made", display.ColorMuted)
			return nil
		}

		ok, err := confirmProceed("  Proceed with multi-module instrumentation?")
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		if !ok {
			fmt.Println("  Installation cancelled.")
			return nil
		}

		if len(proj.RunningProcessIDs) > 0 {
			logger.Debug("stopping running java processes", "pids", proj.RunningProcessIDs)
			stopProcesses(proj.RunningProcessIDs)
		}

		if err := plan.executeMultiModule(); err != nil {
			return err
		}
		return nil
	}

	entrypoints := detectJavaEntrypoints(proj.Path)
	logger.Debug("detected java entrypoints", "count", len(entrypoints), "project", proj.Path)
	if len(entrypoints) == 0 {
		if dryRun {
			display.PrintStatusLine("dry-run", "no runnable entrypoint detected — build the project first", display.ColorMuted)
			return nil
		}
		if err := attemptSingleModuleBuild(proj.Path); err != nil {
			display.PrintStatusLine("error", "no runnable entrypoint detected — build the project first", display.ColorError)
			return err
		}
		entrypoints = detectJavaEntrypoints(proj.Path)
		logger.Debug("detected java entrypoints after build", "count", len(entrypoints), "project", proj.Path)
		if len(entrypoints) == 0 {
			display.PrintStatusLine("error", "no runnable entrypoint detected — build the project first", display.ColorError)
			return fmt.Errorf("no runnable entrypoint detected")
		}
	}

	ep := promptEntrypointSelection(entrypoints)
	if ep == nil {
		logger.Debug("user skipped entrypoint selection")
		return nil
	}

	agentPath, err := javaAgentPath()
	if err != nil {
		return fmt.Errorf("resolving Java agent path: %w", err)
	}
	launchDisplay := displayInstrumentedCmd(*ep, agentPath)

	fmt.Println()
	display.Header("Java instrumentation plan")
	fmt.Printf("  Project:    %s\n", proj.Path)
	fmt.Printf("  Launch:     %s\n", launchDisplay)
	fmt.Printf("  Agent JAR:  %s\n", otelJavaAgentURL)
	fmt.Println()
	fmt.Println("  Environment variables:")
	for _, line := range formatPrintableEnvVars(envVars) {
		fmt.Printf("    %s\n", line)
	}
	var pidStr string
	if len(proj.RunningProcessIDs) > 0 {
		strs := make([]string, len(proj.RunningProcessIDs))
		for i, pid := range proj.RunningProcessIDs {
			strs[i] = fmt.Sprintf("%d", pid)
		}
		pidStr = strings.Join(strs, ", ")
		fmt.Printf("  PIDs to stop: %s\n", pidStr)
	}
	fmt.Println()

	if dryRun {
		logger.Debug("dry-run mode, skipping execution")
		display.PrintStatusLine("dry-run", "no changes made", display.ColorMuted)
		return nil
	}

	confirmText := "  Proceed with installation?"
	if pidStr != "" {
		confirmText = fmt.Sprintf("  Stop PID(s) %s and proceed with installation?", pidStr)
	}
	ok, err := confirmProceed(confirmText)
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return nil
	}

	agentPath, err = downloadJavaAgent()
	if err != nil {
		return fmt.Errorf("downloading Java agent: %w", err)
	}

	stopProcesses(proj.RunningProcessIDs)

	logPath := filepath.Join(proj.Path, serviceName+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}

	launchDisplay = displayInstrumentedCmd(*ep, agentPath)
	logger.Debug("launching instrumented java process", "cmd", launchDisplay, "dir", proj.Path)
	cmd := buildInstrumentedCmd(*ep, agentPath, proj.Path, envVars)

	proc, err := StartManagedProcess(serviceName, serviceName+".log", "", cmd, logFile)
	if err != nil {
		return fmt.Errorf("starting instrumented process: %w", err)
	}
	proc.portDetector = func(pid int) string { return detectJavaListeningPort(pid, proj.Path) }

	aliveNames, _ := PrintProcessSummary([]*ManagedProcess{proc}, processSettleDelay)
	if len(aliveNames) == 0 {
		display.PrintStatusLine("error", "No services are running — check the logs above for errors.", display.ColorError)
		return nil
	}

	updateOtelCollectorIfPresent(envURL, token, dryRun)

	return nil
}
