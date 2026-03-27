package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// InstrumentationPlan is the interface that all runtime instrumentation plans
// must implement. It allows the orchestrator to work with any runtime
// uniformly.
type InstrumentationPlan interface {
	Runtime() string
	PrintPlanSteps()
	Execute()
	SetTokens(envURL, platformToken string)
}

// runtimeInfo describes a supported runtime for the multi-runtime scanner.
type runtimeInfo struct {
	name       string // display name (e.g. "Python", "Java", "Node.js", "Go")
	binName    string // binary to check on PATH (e.g. "python3", "java", "node", "go")
	comingSoon bool   // if true, this runtime is hidden unless DTWIZ_ALL_RUNTIMES is set
}

// detectedProject is a project found during the multi-runtime scan.
// It embeds ScannedProject and adds the runtime name and optional extra
// metadata (e.g. Go module name).
type detectedProject struct {
	ScannedProject
	Runtime    string
	ModuleName string // only set for Go projects
}

// allRuntimesEnabled returns true when the DTWIZ_ALL_RUNTIMES feature flag
// is set to "true" or "1".
func allRuntimesEnabled() bool {
	v := os.Getenv("DTWIZ_ALL_RUNTIMES")
	return v == "true" || v == "1"
}

// detectAvailableRuntimes returns the list of supported runtimes with their
// coming-soon status. Only Python is GA by default; others require
// DTWIZ_ALL_RUNTIMES=true.
func detectAvailableRuntimes() []runtimeInfo {
	allEnabled := allRuntimesEnabled()
	return []runtimeInfo{
		{name: "Python", binName: "python3", comingSoon: false},
		{name: "Java", binName: "java", comingSoon: !allEnabled},
		{name: "Node.js", binName: "node", comingSoon: !allEnabled},
		{name: "Go", binName: "go", comingSoon: !allEnabled},
	}
}

// detectAllProjects scans for projects across all available runtimes,
// skipping coming-soon runtimes and runtimes whose binary is not on PATH.
func detectAllProjects(runtimes []runtimeInfo) []detectedProject {
	var all []detectedProject
	for _, rt := range runtimes {
		if rt.comingSoon {
			continue
		}
		if _, err := exec.LookPath(rt.binName); err != nil {
			continue
		}
		switch rt.name {
		case "Python":
			projects := detectPythonProjects()
			procs := detectPythonProcesses()
			matchProcessesToProjects(projects, procs)
			for _, p := range projects {
				all = append(all, detectedProject{ScannedProject: p, Runtime: "Python"})
			}
		case "Java":
			projects := detectJavaProjects()
			procs := detectProcesses("java", []string{"/bin/dtwiz"})
			for i := range projects {
				projects[i].RunningPIDs = processMatchPIDs(projects[i].Path, procs)
			}
			for _, p := range projects {
				all = append(all, detectedProject{ScannedProject: p, Runtime: "Java"})
			}
		case "Node.js":
			projects := detectNodeProjects()
			procs := detectNodeProcesses()
			matchProcessesToProjects(projects, procs)
			for _, p := range projects {
				all = append(all, detectedProject{ScannedProject: p, Runtime: "Node.js"})
			}
		case "Go":
			projects := detectGoProjects()
			for _, p := range projects {
				all = append(all, detectedProject{
					ScannedProject: p.ScannedProject,
					Runtime:        "Go",
					ModuleName:     p.ModuleName,
				})
			}
		}
	}
	return all
}

// printProjectList prints a numbered list of detected projects across all
// runtimes, with a trailing "Skip" option.
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

// selectProject prompts the user to select a project from the unified list.
// Returns the selected project and true, or zero-value and false if the user
// skips.
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

// createRuntimePlan builds the appropriate InstrumentationPlan for the
// selected project.
func createRuntimePlan(proj detectedProject, apiURL, token string) InstrumentationPlan {
	svcName := serviceNameFromPath(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	switch proj.Runtime {
	case "Python":
		entrypoints := detectPythonEntrypoints(proj.Path)
		needsVenv := detectProjectPip(proj.Path) == nil
		pyEnvVars := generateOtelPythonEnvVars(apiURL, token, svcName)
		return &PythonInstrumentationPlan{
			Project:     proj.ScannedProject,
			Entrypoints: entrypoints,
			NeedsVenv:   needsVenv,
			EnvVars:     pyEnvVars,
		}
	case "Java":
		return &JavaInstrumentationPlan{
			Project: proj.ScannedProject,
			EnvVars: envVars,
		}
	case "Node.js":
		entrypoints := detectNodeEntrypoints(proj.Path)
		var ep string
		if len(entrypoints) > 0 {
			ep = entrypoints[0]
		}
		return &NodeInstrumentationPlan{
			Project:    proj.ScannedProject,
			Entrypoint: ep,
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

// InstallOtelCollector installs the Dynatrace OTel Collector and offers
// runtime auto-instrumentation (Python, Java, …) in a single guided flow.
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

	// Detect all runtimes and let the user pick a project upfront.
	runtimes := detectAvailableRuntimes()
	projects := detectAllProjects(runtimes)

	var plan InstrumentationPlan
	if len(projects) > 0 {
		cyan.Println("  Detected projects:")
		fmt.Println("  " + strings.Repeat("─", 50))
		printProjectList(projects)

		if selected, ok := selectProject(projects); ok {
			plan = createRuntimePlan(selected, cp.apiURL, token)
		}
	}
	fmt.Println()

	// Show combined plan: collector + instrumentation.
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
		plan.SetTokens(envURL, platformToken)
		fmt.Printf("\n  ── %s auto-instrumentation ──\n\n", plan.Runtime())
		plan.Execute()
	}

	return nil
}
