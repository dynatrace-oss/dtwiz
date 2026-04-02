package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/fatih/color"
)

// InstrumentationPlan is the interface that all runtime instrumentation plans
// must implement. It allows the orchestrator to work with any runtime
// uniformly.
type InstrumentationPlan interface {
	Runtime() string
	PrintPlanSteps()
	Execute()
}

// runtimeInfo describes a supported runtime for the multi-runtime scanner.
type runtimeInfo struct {
	name       string // display name (e.g. "Python", "Java", "Node.js", "Go")
	binName    string // binary to check on PATH (e.g. "python3", "java", "node", "go")
	enabled bool // if false, this runtime is skipped unless DTWIZ_ALL_RUNTIMES is set
}

// detectedProject is a project found during the multi-runtime scan.
// It embeds ScannedProject and adds the runtime name and optional extra
// metadata (e.g. Go module name).
type detectedProject struct {
	ScannedProject
	Runtime    string
	ModuleName string // only set for Go projects
}

func allRuntimesEnabled() bool {
	v := os.Getenv("DTWIZ_ALL_RUNTIMES")
	return v == "true" || v == "1"
}

func detectAvailableRuntimes() []runtimeInfo {
	allEnabled := allRuntimesEnabled()
	return []runtimeInfo{
		{name: "Python", binName: "python3", enabled: true},
		{name: "Java", binName: "java", enabled: allEnabled},
		{name: "Node.js", binName: "node", enabled: allEnabled},
		{name: "Go", binName: "go", enabled: allEnabled},
	}
}

func detectAllProjects(runtimes []runtimeInfo) []detectedProject {
	type result struct {
		index    int
		projects []detectedProject
	}

	active := make([]runtimeInfo, 0, len(runtimes))
	for _, rt := range runtimes {
		if !rt.enabled {
			continue
		}
		if _, err := exec.LookPath(rt.binName); err != nil {
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
			var detected []detectedProject
			switch rt.name {
			case "Python":
				projects := detectPythonProjects()
				procs := detectPythonProcesses()
				matchProcessesToProjects(projects, procs)
				for _, p := range projects {
					detected = append(detected, detectedProject{ScannedProject: p, Runtime: "Python"})
				}
			case "Java":
				projects := detectJavaProjects()
				procs := detectJavaProcesses()
				matchProcessesToProjects(projects, procs)
				for _, p := range projects {
					detected = append(detected, detectedProject{ScannedProject: p, Runtime: "Java"})
				}
			case "Node.js":
				projects := detectNodeProjects()
				procs := detectNodeProcesses()
				matchProcessesToProjects(projects, procs)
				for _, p := range projects {
					detected = append(detected, detectedProject{ScannedProject: p, Runtime: "Node.js"})
				}
			case "Go":
				projects := detectGoProjects()
				for _, p := range projects {
					detected = append(detected, detectedProject{
						ScannedProject: p.ScannedProject,
						Runtime:        "Go",
						ModuleName:     p.ModuleName,
					})
				}
			}
			results[idx] = result{index: idx, projects: detected}
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
		if len(p.RunningPIDs) > 0 {
			pidStrs := make([]string, len(p.RunningPIDs))
			for j, pid := range p.RunningPIDs {
				pidStrs[j] = strconv.Itoa(pid)
			}
			line += fmt.Sprintf("  ← PIDs: %s", strings.Join(pidStrs, ", "))
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
	svcName := serviceNameFromPath(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	switch proj.Runtime {
	case "Python":
		entrypoints := detectPythonEntrypoints(proj.Path)
		if len(entrypoints) == 0 {
			fmt.Printf("  Skipping %s — no Python entrypoint found.\n", proj.Path)
			fmt.Println("    Looked for: pyproject.toml [project.scripts], or common files (main.py, app.py, run.py, server.py, manage.py, wsgi.py, asgi.py).")
			fmt.Println("    Add one of these files and re-run dtwiz.")
			return nil
		}
		needsVenv := detectProjectPip(proj.Path) == nil
		pyEnvVars := generateOtelPythonEnvVars(apiURL, token, svcName)
		return &PythonInstrumentationPlan{
			Project:       proj.ScannedProject,
			Entrypoints:   entrypoints,
			NeedsVenv:     needsVenv,
			EnvVars:       pyEnvVars,
			EnvURL:        envURL,
			PlatformToken: platformToken,
		}
	case "Java":
		return &JavaInstrumentationPlan{
			Project: proj.ScannedProject,
			EnvVars: envVars,
		}
	case "Node.js":
		entrypoints := detectNodeEntrypoints(proj.Path)
		return &NodeInstrumentationPlan{
			Project:    proj.ScannedProject,
			Entrypoint: entrypoints[0],
			EnvVars:    envVars,
		}
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

func InstallOtelCollector(envURL, token, ingestToken, platformToken string, dryRun bool) error {
	cyan := color.New(color.FgMagenta)

	fmt.Println()
	cyan.Println("  Dynatrace OpenTelemetry Installation")
	fmt.Println()

	cp, err := prepareCollectorPlan(envURL, token, ingestToken)
	if err != nil {
		return err
	}

	if dryRun {
		cp.printDryRun(ingestToken)
		return nil
	}

	runtimes := detectAvailableRuntimes()
	projects := detectAllProjects(runtimes)

	var plan InstrumentationPlan
	if len(projects) > 0 {
		cyan.Println("  Detected projects:")
		fmt.Println("  " + strings.Repeat("─", 50))
		printProjectList(projects)

		if selected, ok := selectProject(projects); ok {
			plan = createRuntimePlan(selected, cp.apiURL, token, envURL, platformToken)
		}
	}
	fmt.Println()

	if plan != nil {
		cyan.Println("  This will install the OTel Collector and auto-instrument your application.")
	}
	fmt.Println()

	cyan.Println("  1) OTel Collector")
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
	cp.printConfigPreview(cyan, sep)

	if plan != nil {
		fmt.Println()
		cyan.Printf("  2) %s auto-instrumentation\n", plan.Runtime())
		plan.PrintPlanSteps()
	}

	fmt.Println()
	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return nil
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
