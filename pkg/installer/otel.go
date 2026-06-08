package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type InstrumentationPlan interface {
	Runtime() string
	PrintPlanSteps()
	Execute()
}

type runtimeInfo struct {
	name    string
	binName string
	enabled bool
	detect  func() []detectedProject
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

func detectPythonRuntimeProjects() []detectedProject {
	return detectMatchedProjects("Python", detectPythonProjects, detectPythonProcesses)
}

func detectJavaRuntimeProjects() []detectedProject {
	return detectMatchedProjects("Java", detectJavaProjects, detectJavaProcesses)
}

func detectNodeRuntimeProjects() []detectedProject {
	return detectMatchedProjects("Node.js", detectNodeProjects, detectNodeProcesses)
}

func detectGoRuntimeProjects() []detectedProject {
	projects := detectGoProjects()
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

func detectAllProjects(runtimes []runtimeInfo) []detectedProject {
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
		if rt.binName == "python3" || rt.binName == "python" {
			_, err := detectPython()
			available = err == nil
		} else {
			_, err := exec.LookPath(rt.binName)
			available = err == nil
		}
		if !available {
			fmt.Printf("  Skipping %s instrumentation — '%s' not found on PATH.\n", rt.name, rt.binName)
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
			results[idx] = result{projects: rt.detect()}
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
	fmt.Printf("  [%d]  Skip\n", len(projects)+1)
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

	cp, err := prepareCollectorPlan(envURL, token)
	if err != nil {
		return err
	}

	runtimes := detectAvailableRuntimes()

	var plan InstrumentationPlan
	if projectPath != "" {
		runtime := inferRuntimeFromPath(projectPath)
		if runtime == "" {
			return fmt.Errorf("could not detect runtime from project path: %s", projectPath)
		}
		// Verify the runtime binary is actually available before proceeding.
		switch runtime {
		case "Python":
			if _, err := detectPython(); err != nil {
				return fmt.Errorf("Python runtime not available: %w", err)
			}
		case "Java":
			if _, err := exec.LookPath("java"); err != nil {
				return fmt.Errorf("Java runtime not available: 'java' not found on PATH")
			}
		case "Node.js":
			if _, err := exec.LookPath("node"); err != nil {
				return fmt.Errorf("Node.js runtime not available: 'node' not found on PATH")
			}
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
		projects := detectAllProjects(runtimes)
		if len(projects) > 0 {
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
				again, err := confirmProceed("  Select another project?")
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

	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return ErrInstallCancelled
	}
	fmt.Println()

	if err := cp.execute(envURL, platformToken, plan != nil); err != nil {
		return err
	}

	if plan != nil {
		fmt.Printf("\n  ── %s auto-instrumentation ──\n\n", plan.Runtime())
		plan.Execute()
	}

	return nil
}
