package otel

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// isElevatedFn reports whether the process has elevated privileges. Overridable in tests.
var isElevatedFn = installer.IsElevated

type InstrumentationPlan interface {
	Runtime() string
	PrintPlanSteps()
	Execute() error
}

type runtimeInfo struct {
	name    string
	binName string
	enabled bool
	detect  func(roots []string) []detectedProject
}

type detectedProject struct {
	ScannedProject
	Runtime    string
	ModuleName string
}

func detectAvailableRuntimes() []runtimeInfo {
	allEnabled := featureflags.IsEnabled(featureflags.AllRuntimes)
	return []runtimeInfo{
		{name: "Python", binName: "python3", enabled: true, detect: detectPythonRuntimeProjects},
		{name: "Java", binName: "java", enabled: true, detect: detectJavaRuntimeProjects},
		{name: "Node.js", binName: "node", enabled: true, detect: detectNodeRuntimeProjects},
		{name: "Go", binName: "go", enabled: allEnabled, detect: detectGoRuntimeProjects},
	}
}

func detectedProjectsFromScan(runtime string, projects []ScannedProject) []detectedProject {
	detected := make([]detectedProject, 0, len(projects))
	for _, project := range projects {
		detected = append(detected, detectedProject{ScannedProject: project, Runtime: runtime})
	}
	return detected
}

func detectMatchedProjects(runtime string, projectFn func() []ScannedProject, processFn func() []DetectedProcess) []detectedProject {
	projects, processes := runInParallel(projectFn, processFn)
	matchProcessesToProjects(projects, processes)
	return detectedProjectsFromScan(runtime, projects)
}

func detectPythonRuntimeProjects(roots []string) []detectedProject {
	return detectMatchedProjects("Python", func() []ScannedProject { return detectPythonProjects(roots) }, detectPythonProcesses)
}

func detectJavaRuntimeProjects(roots []string) []detectedProject {
	return detectMatchedProjects("Java", func() []ScannedProject { return detectJavaProjects(roots) }, detectJavaProcesses)
}

func detectNodeRuntimeProjects(roots []string) []detectedProject {
	return detectMatchedProjects("Node.js", func() []ScannedProject { return detectNodeProjects(roots) }, detectNodeProcesses)
}

func detectGoRuntimeProjects(roots []string) []detectedProject {
	projects := detectGoProjects(roots)
	detected := make([]detectedProject, 0, len(projects))
	for _, project := range projects {
		detected = append(detected, detectedProject{
			ScannedProject: project.ScannedProject,
			Runtime:        "Go",
			ModuleName:     project.ModuleName,
		})
	}
	return detected
}

// detectAllProjects filters runtimes to those with a usable binary and scans
// for projects in parallel across all enabled runtimes.
func detectAllProjects(runtimes []runtimeInfo, roots []string) []detectedProject {
	type result struct {
		projects []detectedProject
	}

	active := make([]runtimeInfo, 0, len(runtimes))
	for _, rt := range runtimes {
		if !rt.enabled {
			logger.Debug("skipping runtime (disabled)", "runtime", rt.name)
			continue
		}
		// Python's binary may exist as a non-functional stub even when no real
		// interpreter is installed, so run it to verify it's usable.
		var available bool
		var stubOnly bool
		var binPath string
		if rt.binName == "python3" || rt.binName == "python" {
			_, err := DetectPython()
			available = err == nil
			if !available {
				var lookErr error
				binPath, lookErr = exec.LookPath(rt.binName)
				stubOnly = lookErr == nil // binary found but not usable
			}
		} else {
			_, err := exec.LookPath(rt.binName)
			available = err == nil
		}
		if !available {
			if stubOnly {
				if isWindowsStorePythonStub(binPath) {
					fmt.Printf("  Skipping %s instrumentation — '%s' is a Windows Store stub, not a real interpreter.\n  Install Python 3 from https://python.org or disable it in Settings → Apps → Advanced app settings → App execution aliases.\n", rt.name, rt.binName)
				} else {
					fmt.Printf("  Skipping %s instrumentation — '%s' found on PATH but not usable.\n", rt.name, rt.binName)
				}
			} else {
				fmt.Printf("  Skipping %s instrumentation — '%s' not found on PATH.\n", rt.name, rt.binName)
			}
			continue
		}
		active = append(active, rt)
	}

	results := make([]result, len(active))
	var wg sync.WaitGroup
	for i, rt := range active {
		wg.Add(1)
		go func(idx int, rt runtimeInfo) {
			defer wg.Done()
			results[idx] = result{projects: rt.detect(roots)}
		}(i, rt)
	}
	wg.Wait()

	var all []detectedProject
	for _, r := range results {
		all = append(all, r.projects...)
	}
	return all
}

func printProjectList(projects []detectedProject) {
	for i, p := range projects {
		line := fmt.Sprintf("  [%d]  %s  %s  (%s)", i+1, p.Runtime, p.Path, strings.Join(p.Markers, ", "))
		if len(p.RunningProcessIDs) > 0 {
			pidStrs := make([]string, len(p.RunningProcessIDs))
			for j, pid := range p.RunningProcessIDs {
				pidStrs[j] = strconv.Itoa(pid)
			}
			line += fmt.Sprintf("  ← %d processes (PIDs: %s)",
				len(p.RunningProcessIDs),
				strings.Join(pidStrs, ", "))
		}
		if p.ModuleName != "" {
			line += fmt.Sprintf("  (module: %s)", p.ModuleName)
		}
		fmt.Println(line)
	}
	fmt.Printf("  [%d]  ", len(projects)+1)
	display.ColorMessage.Println("Skip — If skipped, you need to send application signals to the OpenTelemetry Collector yourself so they show up in Dynatrace.")
}

func selectProject(projects []detectedProject) (detectedProject, bool) {
	fmt.Println()
	fmt.Printf("  Select a project to instrument [1-%d]: ", len(projects)+1)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return detectedProject{}, false
	}
	num, err := strconv.Atoi(answer)
	if err != nil || num < 1 || num > len(projects)+1 {
		fmt.Println("  Invalid selection, skipping instrumentation.")
		return detectedProject{}, false
	}
	if num == len(projects)+1 {
		return detectedProject{}, false
	}
	return projects[num-1], true
}

func createRuntimePlan(proj detectedProject, apiURL, token, envURL, platformToken string) InstrumentationPlan {
	svcName := projectServiceName(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	switch proj.Runtime {
	case "Python":
		plan := buildPythonInstrumentationPlan(proj.ScannedProject, apiURL, token, envURL, platformToken)
		if plan == nil {
			return nil
		}
		return plan
	case "Java":
		return buildJavaInstrumentationPlan(proj, apiURL, token, envURL)
	case "Node.js":
		plan := buildNodeInstrumentationPlan(proj.ScannedProject, apiURL, token)
		if plan == nil {
			return nil
		}
		return plan
	case "Go":
		goProj := GoProject{
			ScannedProject: proj.ScannedProject,
			ModuleName:     proj.ModuleName,
		}
		return &GoInstrumentationPlan{
			Project: goProj,
			EnvVars: envVars,
		}
	}
	return nil
}

// inferRuntimeFromPath returns "Java", "Node.js", "Python", or "" based on which
// marker files or directories are present directly inside path.
func inferRuntimeFromPath(path string) string {
	hasFile := func(name string) bool {
		_, err := os.Stat(filepath.Join(path, name))
		return err == nil
	}
	for _, m := range javaProjectMarkers {
		if hasFile(m) {
			return "Java"
		}
	}
	for _, m := range nodeProjectMarkers {
		if hasFile(m) {
			return "Node.js"
		}
	}
	for _, m := range pythonProjectMarkers {
		if hasFile(m) {
			return "Python"
		}
	}
	return ""
}

func InstallOtelCollector(envURL, token, platformToken string, dryRun bool) error {
	return InstallOtelCollectorWithProject(envURL, token, platformToken, "", dryRun)
}

func InstallOtelCollectorWithProject(envURL, token, platformToken, projectPath string, dryRun bool) error {
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err != nil {
			return fmt.Errorf("project path not found: %s", projectPath)
		}
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace OpenTelemetry Installation")
	fmt.Println()

	if featureflags.IsEnabled(featureflags.Experimental) {
		fmt.Println("  This will enable OpenTelemetry service and host monitoring.")
		switch runtime.GOOS {
		case "linux":
			if !isElevatedFn() {
				fmt.Println("  Note: full host metrics and logs require elevated privileges (root or systemd-journal group).")
				fmt.Println("        process.disk.io is dropped without privileged access; system.processes.created is Linux-only.")
			}
		case "windows":
			if !isElevatedFn() {
				fmt.Println("  Note: some per-process metrics require Administrator or Debug privilege;")
				fmt.Println("        without it, metrics for services and other users' processes are skipped.")
			}
		case "darwin":
			fmt.Println("  Note: system.processes.created and process.disk.io are unavailable on macOS")
			fmt.Println("        regardless of privilege level and will not appear in Dynatrace.")
		}
	} else {
		supportsLinks := display.StdoutSupportsHyperlinks()
		// ℹ️ (U+2139 + U+FE0F) width is reliable on macOS and Windows (always 2 cols)
		// but inconsistent across Linux terminals — some render it as 1-wide.
		// Fall back to ASCII (i) on Linux and non-hyperlink terminals to guarantee
		// box alignment. Both options pad to 4 visual columns.
		icon := "(i) " // 3-wide ASCII + 1 space
		if supportsLinks && runtime.GOOS != "linux" {
			icon = "ℹ️  " // 2-wide emoji + 2 spaces (macOS/Windows only)
		}
		const hmURL = "https://docs.dynatrace.com/docs/observe/infrastructure-observability/extensions/opentelemetry-host-monitoring"
		fmt.Println("  ┌────────────────────────────────────────────────────────────────┐")
		fmt.Printf("  │ %sThis will enable OpenTelemetry service monitoring.         │\n", icon)
		fmt.Println("  │                                                                │")
		fmt.Println("  │ If you also want to activate host monitoring, follow the       │")
		if supportsLinks {
			fmt.Printf("  │ %s instructions.                    │\n", display.Hyperlink("OpenTelemetry Host Monitoring", hmURL))
		} else {
			fmt.Println("  │ OpenTelemetry Host Monitoring instructions.                    │")
		}
		fmt.Println("  └────────────────────────────────────────────────────────────────┘")
		if !supportsLinks {
			fmt.Printf("    OpenTelemetry Host Monitoring: %s\n", hmURL)
		}
	}
	fmt.Println()

	runtimes := detectAvailableRuntimes()

	cp, err := prepareCollectorPlan(envURL, token)
	if err != nil {
		return err
	}

	var plan InstrumentationPlan
	if projectPath != "" {
		runtime := inferRuntimeFromPath(projectPath)
		if runtime == "" {
			return fmt.Errorf("could not detect runtime from project path: %s", projectPath)
		}
		projects := []ScannedProject{{Path: projectPath}}
		switch runtime {
		case "Java":
			matchProcessesToProjects(projects, detectJavaProcesses())
		case "Node.js":
			matchProcessesToProjects(projects, detectNodeProcesses())
		default:
			matchProcessesToProjects(projects, detectPythonProcesses())
		}
		proj := detectedProject{ScannedProject: projects[0], Runtime: runtime}
		plan = createRuntimePlan(proj, cp.apiURL, token, envURL, platformToken)
	} else {
		roots, err := selectScanRoots()
		if err != nil {
			return err
		}
		projects := detectAllProjects(runtimes, roots)
		if len(projects) == 0 {
			fmt.Println("  No projects detected.")
			cont, err := installer.ConfirmProceed("  Continue installation?")
			if err != nil {
				return fmt.Errorf("reading confirmation: %w", err)
			}
			if !cont {
				return installer.ErrInstallCancelled
			}
		} else {
			for {
				display.ColorMessage.Println("  Detected projects:")
				display.PrintSectionDivider()
				printProjectList(projects)

				selected, ok := selectProject(projects)
				if !ok {
					break
				}
				plan = createRuntimePlan(selected, cp.apiURL, token, envURL, platformToken)
				if plan != nil {
					break
				}
				// Project can't be auto-instrumented; ask if the user wants to try another.
				again, err := installer.ConfirmProceed("  Select another project?")
				if err != nil || !again {
					break
				}
			}
		}
	}
	fmt.Println()

	if plan != nil {
		display.ColorMessage.Println("  This will install the OTel Collector and auto-instrument your application.")
	}
	fmt.Println()

	display.ColorMessage.Println("  1) OTel Collector")
	fmt.Printf("     Directory: %s\n", cp.installDir)
	fmt.Printf("     Binary:    %s\n", cp.binaryPath)
	if len(cp.runningPIDs) > 0 {
		for _, rc := range cp.runningPIDs {
			if rc.path != "" {
				fmt.Printf("     Running:  Existing OTel Collector PID %d at %s (will be stopped)\n", rc.pid, rc.path)
			} else {
				fmt.Printf("     Running:  Existing OTel Collector PID %d (will be stopped)\n", rc.pid)
			}
		}
	}

	sep := strings.Repeat("─", 60)
	cp.printConfigPreview(sep)

	if plan != nil {
		fmt.Println()
		display.ColorMessage.Printf("  2) %s auto-instrumentation\n", plan.Runtime())
		plan.PrintPlanSteps()
	}

	fmt.Println()

	if dryRun {
		display.PrintStatusLine("dry-run", "no changes made", display.ColorMuted)
		return nil
	}

	ok, err := installer.ConfirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	if err := cp.execute(envURL, platformToken, plan != nil); err != nil {
		return err
	}

	if plan != nil {
		fmt.Printf("\n  ── %s auto-instrumentation ──\n\n", plan.Runtime())
		if err := plan.Execute(); err != nil {
			return err
		}
	}

	return nil
}
