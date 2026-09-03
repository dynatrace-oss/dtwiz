package otel

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/environment"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// isElevatedFn reports whether the process has elevated privileges. Overridable in tests.
var isElevatedFn = installer.IsElevated

const otelHostMonitoringExtension = "com.dynatrace.extension.opentelemetry"

// activateHostMonitoringExtensionFn is overridable in tests.
var activateHostMonitoringExtensionFn = activateHostMonitoringExtension

// deactivateHostMonitoringExtensionFn is overridable in tests.
var deactivateHostMonitoringExtensionFn = deactivateHostMonitoringExtension

type extensionManager interface {
	EnsureInstalled(extensionName string) (bool, error)
	LatestExtensionVersion(extensionName string) (string, error)
	ActivateExtension(extensionName, version string) error
	DeactivateExtension(extensionName string) error
	DeleteExtensionVersion(extensionName, version string) error
	GetStatus(extensionName string) (installer.ExtensionStatus, error)
}

var newExtensionManagerFn = newExtensionManager

func newExtensionManager(envURL, platformToken string) (extensionManager, error) {
	return installer.NewExtensionClient(envURL, platformToken)
}

// removeHostMonitoringGrailRoutesFn is overridable in tests.
var removeHostMonitoringGrailRoutesFn = removeHostMonitoringGrailRoutes

// removeGrailRoutesFn is overridable in tests to inject fake results.
var removeGrailRoutesFn = removeGrailRoutes

// removeHostMonitoringGrailRoutes removes the OpenPipeline dynamic routing
// entries for metrics, logs, and spans added during install otel.
// All errors are advisory: a per-signal failure warns and does not abort deactivation.
func removeHostMonitoringGrailRoutes(envURL, platformToken string) {
	c, err := newSDKGrailClient(envURL, platformToken)
	if err != nil {
		logger.Debug("failed to create Grail client for route removal", "error", err)
		fmt.Println("  Warning: could not connect to Grail API; OpenPipeline routes were not removed.")
		return
	}
	removed, errs := removeGrailRoutesFn(context.Background(), c)
	for i, err := range errs {
		if err != nil {
			logger.Debug("failed to remove Grail route", "signal", grailSignals[i].name, "error", err)
			fmt.Printf("  Warning: could not remove OpenPipeline %s route; please remove it manually.\n", grailSignals[i].displayName)
		} else if removed[i] {
			display.ColorOK.Printf("  ✓ OpenPipeline %s route removed\n", grailSignals[i].displayName)
		}
	}
}

// activateHostMonitoringExtension ensures the OTel Host Monitoring extension is
// installed from the Dynatrace Hub and its environment configuration is activated.
// All errors are advisory: a failure logs a warning and returns without aborting the install.
func activateHostMonitoringExtension(envURL, platformToken string) {
	ec, err := newExtensionManagerFn(envURL, platformToken)
	if err != nil {
		logger.Debug("failed to create extension client for host monitoring activation", "error", err)
		fmt.Println("  Warning: could not connect to extensions API; host entity creation may not be available.")
		return
	}
	_, err = ec.EnsureInstalled(otelHostMonitoringExtension)
	if err != nil {
		logger.Debug("failed to ensure OTel host monitoring extension installed", "error", err)
		fmt.Println("  Warning: could not install OTel Host Monitoring extension; host entity creation may not be available.")
		return
	}
	version, err := ec.LatestExtensionVersion(otelHostMonitoringExtension)
	if err != nil {
		logger.Debug("failed to get OTel host monitoring extension version", "error", err)
		fmt.Println("  Warning: could not determine OTel Host Monitoring extension version; host entity creation may not be available.")
		return
	}
	if err := ec.ActivateExtension(otelHostMonitoringExtension, version); err != nil {
		logger.Debug("failed to activate OTel host monitoring extension", "error", err)
		fmt.Println("  Warning: could not activate OTel Host Monitoring extension; host entity creation may not be available.")
		return
	}
	display.ColorOK.Println("  ✓ OTel Host Monitoring extension active")
}

// buildExtensionActivationPreviewFn is overridable in tests.
var buildExtensionActivationPreviewFn = buildExtensionActivationPreview

// buildExtensionActivationPreview checks the current state of the OTel Host
// Monitoring extension without changing anything, so its status can be shown
// in the install preview alongside the OpenPipeline route plan.
func buildExtensionActivationPreview(envURL, platformToken string) (installer.ExtensionStatus, error) {
	ec, err := newExtensionManagerFn(envURL, platformToken)
	if err != nil {
		return 0, fmt.Errorf("create extension client: %w", err)
	}
	return ec.GetStatus(otelHostMonitoringExtension)
}

// printExtensionActivationPreview prints a one-line summary of what the
// extension activation step will do, as part of the install preview.
func printExtensionActivationPreview(status installer.ExtensionStatus) {
	display.ColorMessage.Println("  OpenTelemetry Host Monitoring extension")
	display.PrintSectionDivider()
	var msg string
	colorFn := display.ColorDefault
	switch status {
	case installer.ExtensionInstalledActive, installer.ExtensionInstalledInactive:
		// The Dynatrace API's per-version "active" flag isn't a reliable signal for this
		// extension: pipelines get provisioned on install, not on activation, so a tenant
		// can show "active": false while host monitoring already works end to end. Collapse
		// both installed states into one message rather than claim an activation state that
		// can't be confirmed.
		msg = "already installed"
		colorFn = display.ColorMuted
	case installer.ExtensionNotInstalled:
		msg = "will be installed and activated"
		colorFn = display.ColorOK
	}
	display.PrintStatusLine("Extension", msg, colorFn)
}

// deactivateHostMonitoringExtension removes the OTel Host Monitoring extension version from
// the tenant. All errors are advisory: a failure logs a warning and returns without aborting
// the uninstall.
func deactivateHostMonitoringExtension(envURL, platformToken string) {
	removeHostMonitoringGrailRoutesFn(envURL, platformToken)
	ec, err := newExtensionManagerFn(envURL, platformToken)
	if err != nil {
		logger.Debug("failed to create extension client for host monitoring deactivation", "error", err)
		fmt.Println("  Warning: could not connect to extensions API; OTel Host Monitoring extension was not removed.")
		return
	}
	if err := ec.DeactivateExtension(otelHostMonitoringExtension); err != nil {
		logger.Debug("failed to deactivate OTel host monitoring extension environment configuration", "error", err)
		fmt.Println("  Warning: could not deactivate OTel Host Monitoring extension; extension was not removed.")
		return
	}
	version, err := ec.LatestExtensionVersion(otelHostMonitoringExtension)
	if err != nil {
		if installer.IsExtensionNotFound(err, otelHostMonitoringExtension) {
			logger.Debug("OTel host monitoring extension not installed; nothing to remove")
			return
		}
		logger.Debug("failed to get OTel host monitoring extension version", "error", err)
		fmt.Println("  Warning: could not determine OTel Host Monitoring extension version; extension was not removed.")
		return
	}
	if err := ec.DeleteExtensionVersion(otelHostMonitoringExtension, version); err != nil {
		logger.Debug("failed to delete OTel host monitoring extension version", "error", err)
		fmt.Println("  Warning: could not remove OTel Host Monitoring extension; please remove it manually.")
		return
	}
	display.ColorOK.Println("  ✓ OTel Host Monitoring extension removed")
}

// buildTenantPrerequisitePreview shows extension and route previews.
// Returns nil values when no platform token is available or preview setup fails.
func buildTenantPrerequisitePreview(envURL, platformToken string) (grailRouteClient, []grailSignalPlan) {
	if platformToken == "" {
		logger.Debug("platform token not provided, skipping extension and route previews")
		return nil, nil
	}
	if status, err := buildExtensionActivationPreviewFn(envURL, platformToken); err != nil {
		fmt.Println()
		display.PrintWarning("OTel Host Monitoring extension", err)
	} else {
		fmt.Println()
		printExtensionActivationPreview(status)
	}
	c, plans, err := buildGrailRoutePlansFn(envURL, platformToken)
	if err != nil {
		fmt.Println()
		display.PrintWarning("OpenPipeline routes", err)
		return nil, nil
	}
	fmt.Println()
	printGrailPlan(plans)
	return c, plans
}

// reconcileGrailRoutes waits for host-monitoring pipelines, rebuilds the route
// plan, falls back to the preview snapshot if rebuilding fails, and returns the
// final plans with their per-signal apply errors.
func reconcileGrailRoutes(grailC grailRouteClient, grailPlans []grailSignalPlan) ([]grailSignalPlan, []error) {
	if grailC == nil {
		return nil, nil
	}
	if err := waitForGrailPipelinesFn(context.Background(), grailC, time.Sleep); err != nil {
		logger.Debug("OTel host-monitoring pipelines did not appear within the wait bound", "error", err)
	}
	if freshPlans, err := buildGrailPlans(context.Background(), grailC); err != nil {
		logger.Debug("failed to rebuild Grail route plans after extension activation, applying preview snapshot", "error", err)
	} else {
		grailPlans = freshPlans
	}
	logger.Debug("applying Grail route plans", "count", len(grailPlans))
	applyErrs := make([]error, len(grailPlans))
	for i, p := range grailPlans {
		applyErrs[i] = applyGrailPlan(context.Background(), grailC, p)
	}
	return grailPlans, applyErrs
}

func applyGrailRoutes(grailC grailRouteClient, grailPlans []grailSignalPlan) {
	plans, applyErrs := reconcileGrailRoutes(grailC, grailPlans)
	if plans == nil {
		return
	}
	printGrailApplyResults(plans, grailApplyErrsWithValidation(plans, applyErrs, nil))
}

// applyAndValidateGrailRoutes is the install-only path: after shared route
// reconciliation, it performs bounded readback validation before printing
// final route results.
func applyAndValidateGrailRoutes(grailC grailRouteClient, grailPlans []grailSignalPlan) {
	plans, applyErrs := reconcileGrailRoutes(grailC, grailPlans)
	if plans == nil {
		return
	}

	validations := grailRouteValidations(plans, applyErrs)
	validationErr := waitForGrailRoutesAppliedFn(context.Background(), grailC, validations, time.Sleep)
	if validationErr != nil {
		logger.Debug("OpenPipeline routes were not visible after apply", "error", validationErr)
	}
	printGrailApplyResults(plans, grailApplyErrsWithValidation(plans, applyErrs, validationErr))
}

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

// manualLanguageEntry describes a language that dtwiz does not auto-instrument.
// The user can select it to receive an OTel instrumentation guide after install.
type manualLanguageEntry struct {
	Key     string // single-character selection key
	Name    string // display name
	URLSlug string // slug for https://dt-url.net/otel-<slug>
}

var manualLanguages = []manualLanguageEntry{
	{Key: "p", Name: "PHP", URLSlug: "php"},
	{Key: "c", Name: "C++", URLSlug: "cpp"},
	{Key: "n", Name: ".NET", URLSlug: "dotnet"},
	{Key: "e", Name: "Elixir", URLSlug: "elixir"},
	{Key: "l", Name: "Erlang", URLSlug: "erlang"},
	{Key: "g", Name: "Go", URLSlug: "go"},
	{Key: "r", Name: "Ruby", URLSlug: "ruby"},
	{Key: "u", Name: "Rust", URLSlug: "rust"},
	{Key: "o", Name: "Other language", URLSlug: "other"},
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
	fmt.Println()
	display.ColorMuted.Println("  Following languages don't support automatic project detection yet:")
	for _, lang := range manualLanguages {
		fmt.Printf("  [%s]  %-14s — OTel instrumentation guide shown after install\n", lang.Key, lang.Name)
	}
	fmt.Println()
	fmt.Println("  [s]  Skip — I already have my application instrumented with OpenTelemetry or just want host monitoring")
}

// selectProject prompts the user to select a detected project, a manual language, or skip.
// Returns (project, languageSlug, ok):
//   - ok=false means skip / cancel
//   - languageSlug != "" means a manual language was selected (no project)
//   - otherwise a detected project was selected
func selectProject(projects []detectedProject) (detectedProject, string, bool) {
	fmt.Println()
	fmt.Print("  Your selection: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "" || answer == "s" {
		return detectedProject{}, "", false
	}
	for _, lang := range manualLanguages {
		if answer == lang.Key {
			return detectedProject{}, lang.URLSlug, true
		}
	}
	num, err := strconv.Atoi(answer)
	if err != nil || num < 1 || num > len(projects) {
		fmt.Println("  Invalid selection, skipping instrumentation.")
		return detectedProject{}, "", false
	}
	return projects[num-1], "", true
}

func createRuntimePlan(proj detectedProject, httpPort int, token, envURL, platformToken string) InstrumentationPlan {
	collectorEndpoint := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	svcName := environment.ProjectServiceName(proj.Path)

	switch proj.Runtime {
	case "Python":
		plan := buildPythonInstrumentationPlan(proj.ScannedProject, collectorEndpoint, envURL, platformToken)
		if plan == nil {
			return nil
		}
		return plan
	case "Java":
		return buildJavaInstrumentationPlan(proj, collectorEndpoint, token, envURL)
	case "Node.js":
		plan := buildNodeInstrumentationPlan(proj.ScannedProject, collectorEndpoint)
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
			EnvVars: environment.GenerateBaseOtelEnvVars(collectorEndpoint, svcName),
		}
	}
	return nil
}

var createRuntimePlanFn = createRuntimePlan

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

// InstallOtelCollector installs the OTel Collector with interactive project selection.
// Returns the URL slug of the manually-selected language (e.g. "go", "php") when
// the user chose a language that requires manual instrumentation; empty otherwise.
func InstallOtelCollector(envURL, token, platformToken string, dryRun bool) (string, error) {
	return InstallOtelCollectorWithProject(envURL, token, platformToken, "", dryRun)
}

// InstallOtelCollectorWithProject installs the OTel Collector, optionally with
// auto-instrumentation for a pre-selected project path.
// Returns the URL slug of the manually-selected language when the user chose one.
func InstallOtelCollectorWithProject(envURL, token, platformToken, projectPath string, dryRun bool) (string, error) {
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err != nil {
			return "", fmt.Errorf("project path not found: %s", projectPath)
		}
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace OpenTelemetry Installation")
	fmt.Println()

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
		logger.Debug("system.processes.created and process.disk.io are unavailable on macOS regardless of privilege level")
	}
	fmt.Println()

	runtimes := detectAvailableRuntimes()

	cp, err := prepareCollectorPlan(envURL, token)
	if err != nil {
		return "", err
	}

	var plan InstrumentationPlan
	var manualLang string
	if projectPath != "" {
		rt := inferRuntimeFromPath(projectPath)
		if rt == "" {
			return "", fmt.Errorf("could not detect runtime from project path: %s", projectPath)
		}
		projects := []ScannedProject{{Path: projectPath}}
		switch rt {
		case "Java":
			matchProcessesToProjects(projects, detectJavaProcesses())
		case "Node.js":
			matchProcessesToProjects(projects, detectNodeProcesses())
		default:
			matchProcessesToProjects(projects, detectPythonProcesses())
		}
		proj := detectedProject{ScannedProject: projects[0], Runtime: rt}
		plan = createRuntimePlanFn(proj, cp.httpPort, token, envURL, platformToken)
	} else {
		roots, err := selectScanRoots()
		if err != nil {
			return "", err
		}
		logger.Debug("scanning for projects", "roots", roots)
		projects := detectAllProjects(runtimes, roots)
		if len(projects) > 0 {
			display.ColorMessage.Println("  Detected projects:")
		} else {
			display.ColorMessage.Println("  Select how to instrument your application:")
		}
		display.PrintSectionDivider()
		printProjectList(projects)
		for {
			selected, lang, ok := selectProject(projects)
			if !ok {
				break
			}
			if lang != "" {
				manualLang = lang
				break
			}
			plan = createRuntimePlanFn(selected, cp.httpPort, token, envURL, platformToken)
			if plan != nil {
				break
			}
			// Project can't be auto-instrumented; ask if the user wants to try another.
			again, err := installer.ConfirmProceed("  Select another project?")
			if err != nil || !again {
				break
			}
			display.PrintSectionDivider()
			printProjectList(projects)
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

	grailC, grailPlans := buildTenantPrerequisitePreview(envURL, platformToken)

	fmt.Println()

	if dryRun {
		display.PrintStatusLine("dry-run", "no changes made", display.ColorMuted)
		return "", nil
	}

	ok, err := installer.ConfirmProceed("  Proceed with installation?")
	if err != nil {
		return "", fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return "", installer.ErrInstallCancelled
	}
	fmt.Println()

	if platformToken != "" {
		activateHostMonitoringExtensionFn(envURL, platformToken)
	}
	applyAndValidateGrailRoutes(grailC, grailPlans)

	if err := executeCollectorPlanFn(cp, envURL, platformToken, plan != nil); err != nil {
		return "", err
	}

	if plan != nil {
		fmt.Printf("\n  ── %s auto-instrumentation ──\n\n", plan.Runtime())
		if err := plan.Execute(); err != nil {
			return "", err
		}
	}

	return manualLang, nil
}
