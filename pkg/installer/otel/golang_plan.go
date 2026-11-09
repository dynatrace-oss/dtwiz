package otel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/environment"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// goFallbackNote is shown once when otelc cannot be used.
const goFallbackNote = "  Instrumentation with otelc is not possible, falling back to client-only instrumentation (outgoing HTTP calls only)."

// errOtelcUnusable marks an otelc binary that is not a working otelc (it has
// no module version).
var errOtelcUnusable = errors.New("otelc binary is not usable")

type goMethod string

const (
	goMethodOtelc      goMethod = "otelc"
	goMethodClientOnly goMethod = "client-only"
	goMethodManual     goMethod = "manual"
)

const (
	goMarkerFile   = "dtwiz-go-instrumentation.json"
	goWalkthrough  = "https://dt-url.net/otel-go"
	goReasonNoTool = "no Go toolchain was found on PATH"
)

// GoServicePlan is one service of a Go project with its chosen method.
type GoServicePlan struct {
	GoService
	Method  goMethod
	BinPath string
	Err     error // last build failure, set when the service ends up manual
	// Reason says why the service is manual when that was decided before any
	// build (for example a failed isolated pre-flight in a workspace).
	Reason string
}

// GoInstrumentationPlan instruments a Go project. Plans built by
// buildGoInstrumentationPlan (Automatic) build and launch each service; a zero
// value Automatic keeps the legacy manual-steps output used while the
// GoInstrumentation flag is off.
type GoInstrumentationPlan struct {
	Project GoProject
	EnvVars map[string]string

	Automatic         bool
	CollectorEndpoint string
	// Workspace is true when the project is part of a Go workspace (go.work).
	// Its services get an isolated pre-flight at the start of Execute.
	Workspace bool
	Services  []*GoServicePlan
	// ManualReason is set when the whole project is handled with manual steps.
	ManualReason string
	// firstTierErr is the first classified build failure, kept for telemetry.
	firstTierErr error
}

// Seams replaced in tests.
var (
	goLookPath          = func(file string) (string, error) { return installer.ExecLookPath(file) }
	goFindOtelcFn       = findOtelc
	goEnsureOtelcFn     = ensureOtelc
	goBuildOtelcFn      = buildWithOtelc
	goPrepareFallbackFn = prepareGoFallback
	goStopProcessesFn   = stopProcesses
	goListProcessesFn   = listProcesses
	goLaunchFn          = launchGoService
	goSummaryFn         = PrintProcessSummary
)

func goBinaryName(service string) string {
	if runtime.GOOS == "windows" {
		return service + ".exe"
	}
	return service
}

// buildGoInstrumentationPlan builds the plan for a selected Go project. It
// returns nil (after telling the user why) when there is nothing to instrument.
func buildGoInstrumentationPlan(proj detectedProject, collectorEndpoint string) *GoInstrumentationPlan {
	goProj := GoProject{ScannedProject: proj.ScannedProject, ModuleName: proj.ModuleName}
	if goProj.ModuleName == "" {
		goProj.ModuleName = extractGoModuleName(filepath.Join(proj.Path, "go.mod"))
	}

	services := discoverGoServices(proj.Path)
	if len(services) == 0 {
		fmt.Println()
		fmt.Printf("  No Go service (package main) found in %s — nothing to instrument.\n", proj.Path)
		return nil
	}

	var skipped []GoService
	kept := services[:0:0]
	for _, svc := range services {
		if moduleUsesOtelSDK(filepath.Join(svc.moduleDir(proj.Path), "go.mod")) {
			skipped = append(skipped, svc)
			continue
		}
		kept = append(kept, svc)
	}
	if len(skipped) > 0 {
		printGoSkippedServices(skipped, collectorEndpoint)
	}
	if len(kept) == 0 {
		return nil
	}

	plan := &GoInstrumentationPlan{
		Project:           goProj,
		Automatic:         true,
		CollectorEndpoint: collectorEndpoint,
		EnvVars:           environment.GenerateBaseOtelEnvVars(collectorEndpoint, environment.ProjectServiceName(proj.Path)),
	}
	for _, s := range kept {
		plan.Services = append(plan.Services, &GoServicePlan{
			GoService: s,
			Method:    goMethodOtelc,
			BinPath:   filepath.Join(goOtelDir(s.moduleDir(proj.Path)), "bin", goBinaryName(s.Name)),
		})
	}

	if _, err := goLookPath("go"); err != nil {
		plan.setManual(goReasonNoTool)
	} else if findGoWork(proj.Path) != "" {
		// Only recorded here: the pre-flight runs `go list -deps`, which can
		// download modules, so it waits until the user has confirmed (Execute).
		plan.Workspace = true
	}
	return plan
}

func (p *GoInstrumentationPlan) setManual(reason string) {
	p.ManualReason = reason
	for _, s := range p.Services {
		s.Method = goMethodManual
	}
}

func printGoSkippedServices(services []GoService, collectorEndpoint string) {
	names := make([]string, len(services))
	for i, s := range services {
		names[i] = s.Name
	}
	fmt.Println()
	fmt.Printf("  Skipping %s: the module already configures the OpenTelemetry SDK or an exporter.\n", strings.Join(names, ", "))
	fmt.Printf("  Point its OTLP exporter at the local collector instead: %s (OTLP/HTTP)\n", collectorEndpoint)
}

// PrintPlanSteps shows the project, the method per service and the tooling.
// It creates no files.
func (p *GoInstrumentationPlan) PrintPlanSteps() {
	if !p.Automatic {
		p.printLegacyPlanSteps()
		return
	}
	fmt.Printf("     Project:    %s\n", p.Project.Path)
	if p.Project.ModuleName != "" {
		fmt.Printf("     Module:     %s\n", p.Project.ModuleName)
	}
	if p.ManualReason != "" {
		fmt.Printf("     Method:     manual — %s\n", p.ManualReason)
		fmt.Println("     The application is not instrumented automatically; manual steps are printed after the collector is installed.")
		fmt.Println("     The OTel Collector is still installed.")
		return
	}

	if len(p.Project.RunningProcessIDs) > 0 {
		pids := make([]string, len(p.Project.RunningProcessIDs))
		for i, pid := range p.Project.RunningProcessIDs {
			pids[i] = fmt.Sprint(pid)
		}
		fmt.Printf("     Stop running processes (PIDs: %s)\n", strings.Join(pids, ", "))
	}
	fmt.Println("     Services:")
	usesOtelc, usesFallback, usesManual := false, false, false
	for _, s := range p.Services {
		fmt.Printf("       %-16s %-24s %s\n", s.Name, s.ProjectRel, goMethodLabel(s.Method))
		if s.Reason != "" {
			fmt.Printf("         %s\n", s.Reason)
		}
		usesOtelc = usesOtelc || s.Method == goMethodOtelc
		usesFallback = usesFallback || s.Method == goMethodClientOnly
		usesManual = usesManual || s.Method == goMethodManual
	}
	if usesManual {
		fmt.Println("     Manual-step services are not built; manual steps are printed after the run.")
	}
	if usesOtelc {
		fmt.Printf("     Tool:       %s\n", describeOtelcForPreview())
	}
	if usesFallback {
		fmt.Println("     Client-only services trace outgoing HTTP calls only; incoming requests are not traced.")
	}
	if p.Workspace {
		fmt.Println("     Workspace (go.work): each module is checked to build on its own; modules that cannot are left to manual steps.")
	}
	fmt.Println("     Project sources are not modified; go.mod and go.sum are restored after each build.")
	if dirs := p.moduleDirs(); len(dirs) == 1 {
		fmt.Printf("     Binaries and logs: %s\n", goOtelDir(dirs[0]))
	} else if len(dirs) > 1 {
		fmt.Printf("     Binaries and logs: %s in each of the %d modules\n", filepath.Join(".otel", goOtelDirName), len(dirs))
	}
	fmt.Println("     The first build downloads modules and needs a network connection.")
}

func goMethodLabel(m goMethod) string {
	switch m {
	case goMethodOtelc:
		return "compile-time instrumentation (otelc)"
	case goMethodClientOnly:
		return "client-only instrumentation (outgoing HTTP only)"
	default:
		return "manual steps"
	}
}

func describeOtelcForPreview() string {
	if tool, ok := goFindOtelcFn(context.Background()); ok {
		if tool.Version != "" {
			return fmt.Sprintf("otelc %s (already installed)", tool.Version)
		}
		return "otelc (already installed)"
	}
	if dir, err := isolatedOtelcDir(); err == nil {
		return fmt.Sprintf("otelc (latest release) will be installed to %s", dir)
	}
	return "otelc (latest release) will be installed"
}

// Execute builds, launches and reports the services. Manual outcomes are not
// errors; an error is returned only when no service could be started.
func (p *GoInstrumentationPlan) Execute() error {
	if !p.Automatic {
		return p.executeLegacy()
	}
	if p.ManualReason != "" {
		fmt.Printf("  Not instrumented automatically: %s.\n", p.ManualReason)
		p.printManualSteps()
		p.recordGoInstrumentation(0)
		return nil
	}

	// Ctrl+C cancels the pre-flight and the builds. The handler is released as
	// soon as they are done: launching does not look at ctx, so keeping it
	// installed would swallow Ctrl+C there.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if p.Workspace {
		p.runGoPreflight(ctx)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("interrupted: %w", err)
	}
	p.stopReplacedProcesses()
	p.buildServices(ctx)
	interrupted := ctx.Err() // read before stop(), which cancels ctx as well
	stop()

	if interrupted != nil {
		return fmt.Errorf("interrupted: %w", interrupted)
	}
	return p.launchServices()
}

// stopReplacedProcesses stops this project's running Go processes that the new
// binaries replace: `go run` and dtwiz-built binaries, or binaries named like a
// service. Other Go tools running in the directory are left alone.
func (p *GoInstrumentationPlan) stopReplacedProcesses() {
	if len(p.Project.RunningProcessIDs) == 0 {
		return
	}
	want := make(map[int]bool, len(p.Project.RunningProcessIDs))
	for _, pid := range p.Project.RunningProcessIDs {
		want[pid] = true
	}
	names := make(map[string]bool, len(p.Services))
	for _, s := range p.Services {
		names[strings.ToLower(goBinaryName(s.Name))] = true
	}
	var pids []int
	for _, proc := range goListProcessesFn() {
		if !want[proc.PID] {
			continue
		}
		fields := splitCommandLine(proc.Command)
		if len(fields) == 0 {
			continue
		}
		base := strings.ToLower(filepath.Base(fields[0]))
		isGoRun := (base == "go" || base == "go.exe") && len(fields) > 1 && fields[1] == "run"
		if isGoRun || names[base] || isDtwizGoBinary(proc.Command) {
			pids = append(pids, proc.PID)
		}
	}
	if len(pids) == 0 {
		return
	}
	fmt.Println("  Stopping running processes...")
	goStopProcessesFn(pids)
}

// dtwizGoBinDir is the path segment under which dtwiz places the services it launches.
const dtwizGoBinDir = "/.otel/go/bin/"

// isDtwizGoBinary reports whether a command line starts with an executable
// under a module's .otel/go/bin directory. Only absolute executable paths
// qualify (so `go run`, editors and shells that merely mention the path do
// not), and the check also holds for quoted and backslash Windows paths.
func isDtwizGoBinary(command string) bool {
	cmd := strings.TrimSpace(command)
	cmd = strings.TrimPrefix(cmd, `"`)
	absolute := strings.HasPrefix(cmd, "/") || (len(cmd) > 2 && cmd[1] == ':' && (cmd[2] == '\\' || cmd[2] == '/'))
	if !absolute {
		return false
	}
	norm := strings.ToLower(strings.ReplaceAll(cmd, `\`, "/"))
	return strings.Contains(norm, dtwizGoBinDir)
}

// buildServices builds every service, degrading per service: otelc, then the
// client-only build, then manual steps.
func (p *GoInstrumentationPlan) buildServices(ctx context.Context) {
	// Modules with at least one build attempt get files under .otel/go, even
	// when every attempt ends up manual.
	attempted := p.moduleDirs()
	// A marker from an earlier run means .otel/go is not ours to clean up now.
	preexisting := map[string]bool{}
	for _, m := range attempted {
		if _, err := os.Stat(filepath.Join(goOtelDir(m), goMarkerFile)); err == nil {
			preexisting[m] = true
		}
	}
	var tool *otelcTool
	otelcUsable := false
	needsTool := false
	for _, s := range p.Services {
		needsTool = needsTool || s.Method == goMethodOtelc
	}
	if needsTool {
		var err error
		tool, err = goEnsureOtelcFn(ctx)
		if err == nil && strings.TrimSpace(tool.Version) == "" {
			// A real otelc is a Go binary and always has a module version; a
			// binary without one cannot be used.
			err = errOtelcUnusable
		}
		if err != nil {
			p.reportToolFailure(err)
			for _, s := range p.Services {
				if s.Method == goMethodOtelc {
					s.Method = goMethodClientOnly
				}
			}
			p.noteTierError("otelc-install", err)
		} else {
			logger.Debug("otelc ready", "path", tool.Path, "version", tool.Version)
			otelcUsable = true
		}
	}
	if len(attempted) > 0 {
		if otelcUsable {
			fmt.Println("  The first build of each module can take up to a minute (downloading dependencies); later services in the same module are faster.")
		}
		fmt.Println()
	}

	type moduleFallback struct {
		fb  *goFallback
		err error
	}
	fallbacks := map[string]*moduleFallback{}
	// otelc's work directory is shared by the services of a module (later
	// builds are much faster) and removed when this run ends.
	var workDirs otelcWorkDirs
	defer workDirs.Close()
	fallbackNoted := false
	var clientBuildErrs []error

	for _, s := range p.Services {
		if ctx.Err() != nil {
			return
		}
		module := s.moduleDir(p.Project.Path)
		if s.Method == goMethodOtelc {
			fmt.Printf("  Building %s... ", s.Name)
			workDir, err := workDirs.For(module)
			if err == nil {
				err = goBuildOtelcFn(ctx, tool, module, workDir, s.GoService, s.BinPath)
			}
			if err == nil {
				fmt.Println("done.")
				continue
			}
			workDirs.Reset(module) // a failed build must not leave state for the next service
			if ctx.Err() != nil {
				fmt.Println("failed.")
				fmt.Printf("    %v\n", err)
				return
			}
			p.noteTierError("otelc-build", err)
			logger.Debug("otelc build failed, falling back to client-only instrumentation", "service", s.Name, "error", err)
			display.AbandonStdoutLine()
			if !fallbackNoted {
				fmt.Println(goFallbackNote)
				fallbackNoted = true
			}
			s.Method = goMethodClientOnly
		}
		if s.Method == goMethodClientOnly {
			mf := fallbacks[module]
			if mf == nil {
				mf = &moduleFallback{}
				mf.fb, mf.err = goPrepareFallbackFn(module, p.servicesOfModule(module))
				fallbacks[module] = mf
			}
			var err error
			if mf.err != nil {
				err = mf.err
			} else {
				fmt.Printf("  Building %s... ", s.Name)
				err = mf.fb.buildClientOnly(ctx, s.GoService, s.BinPath)
			}
			if err == nil {
				fmt.Println("done.")
				continue
			}
			fmt.Println("failed.")
			if ctx.Err() != nil {
				fmt.Printf("    %v\n", err)
			} else {
				logger.Debug("client-only build failed", "service", s.Name, "error", err)
			}
			p.noteTierError("go-client-build", err)
			clientBuildErrs = append(clientBuildErrs, err)
			s.Method = goMethodManual
			s.Err = err
		}
	}
	if ctx.Err() == nil && len(clientBuildErrs) > 0 {
		fmt.Println(goBuildFailureCause(clientBuildErrs))
	}
	p.finalizeGoModules(attempted, preexisting)
}

// goBuildFailureCause is the one-line explanation printed after the list of
// services whose client-only build failed. The full errors are in the debug log.
func goBuildFailureCause(errs []error) string {
	for _, err := range errs {
		if isGoNetworkError(err.Error()) {
			return "  Could not download the OpenTelemetry modules — check your network connection or GOPROXY (run with --debug for details)."
		}
	}
	return "  Could not build the instrumented services — run with --debug for details."
}

func (p *GoInstrumentationPlan) reportToolFailure(err error) {
	fmt.Println(goFallbackNote)
	var tooOld *GoVersionTooOldError
	switch {
	case errors.As(err, &tooOld):
		fmt.Printf("  %v.\n", tooOld)
	case errors.Is(err, errOtelcUnusable):
		logger.Debug("otelc is not usable", "error", err)
	default:
		fmt.Printf("  %v\n", err)
	}
}

// moduleDirs returns the distinct module directories of the services that are
// built (not manual), in service order.
func (p *GoInstrumentationPlan) moduleDirs() []string {
	var dirs []string
	seen := map[string]bool{}
	for _, s := range p.Services {
		if s.Method == goMethodManual {
			continue
		}
		dir := s.moduleDir(p.Project.Path)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// servicesOfModule returns the services that belong to one module.
func (p *GoInstrumentationPlan) servicesOfModule(module string) []GoService {
	var out []GoService
	for _, s := range p.Services {
		if s.moduleDir(p.Project.Path) == module {
			out = append(out, s.GoService)
		}
	}
	return out
}

// finalizeGoModules marks the modules that kept at least one built service and
// removes the files the failed build attempts left in modules where every
// service ended up manual. A module whose .otel/go existed before this run, or
// whose go.mod backup could not be restored, is left untouched.
func (p *GoInstrumentationPlan) finalizeGoModules(attempted []string, preexisting map[string]bool) {
	var built []string
	for _, module := range attempted {
		if p.moduleHasBuiltService(module) || preexisting[module] {
			built = append(built, module)
			continue
		}
		removeFailedGoArtifacts(module)
	}
	writeGoMarkers(built)
}

// moduleHasBuiltService reports whether any service of the module has a binary.
func (p *GoInstrumentationPlan) moduleHasBuiltService(module string) bool {
	for _, s := range p.Services {
		if s.moduleDir(p.Project.Path) == module && s.Method != goMethodManual {
			return true
		}
	}
	return false
}

// removeFailedGoArtifacts deletes <module>/.otel/go, and .otel when that leaves
// it empty. The modguard backup is the user's recovery path for go.mod/go.sum,
// so its presence keeps everything in place.
func removeFailedGoArtifacts(module string) {
	dir := goOtelDir(module)
	if _, err := os.Stat(filepath.Join(dir, goModguardDirName)); err == nil {
		logger.Warn("keeping Go instrumentation directory: go.mod backup present", "dir", dir)
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		logger.Warn("could not remove files of failed Go instrumentation", "dir", dir, "err", err)
		return
	}
	// os.Remove only succeeds on an empty directory, which is the intent.
	_ = os.Remove(filepath.Dir(dir))
}

// writeGoMarkers records, in every given module, that dtwiz created .otel/go,
// so uninstall can recognize it.
func writeGoMarkers(modules []string) {
	for _, module := range modules {
		dir := goOtelDir(module)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Warn("could not create Go instrumentation directory", "dir", dir, "err", err)
			continue
		}
		data, _ := json.Marshal(map[string]string{"created-by": "dtwiz", "runtime": "go"})
		if err := os.WriteFile(filepath.Join(dir, goMarkerFile), append(data, '\n'), 0644); err != nil {
			logger.Warn("could not write Go instrumentation marker", "dir", dir, "err", err)
			continue
		}
		registerOtelDir(dir)
	}
}

// launchServices starts every built service, prints the summary and reports
// manual and client-only services.
func (p *GoInstrumentationPlan) launchServices() error {
	var procs []*ManagedProcess
	var manual, clientOnly []*GoServicePlan
	fmt.Println()
	for _, s := range p.Services {
		switch s.Method {
		case goMethodManual:
			manual = append(manual, s)
			continue
		case goMethodClientOnly:
			clientOnly = append(clientOnly, s)
		}
		env := goServiceEnv(os.Environ(), p.EnvVars, s.Name)
		mp, err := goLaunchFn(s, s.moduleDir(p.Project.Path), env)
		if err != nil {
			fmt.Printf("  Failed to start %s: %v\n", s.Name, err)
			continue
		}
		procs = append(procs, mp)
	}

	if len(manual) > 0 {
		for _, s := range manual {
			if s.Reason != "" {
				fmt.Printf("  %s: not instrumented automatically: %s\n", s.Name, s.Reason)
			}
		}
		p.printManualSteps()
	}

	started, _ := goSummaryFn(procs, processSettleDelay)
	p.recordGoInstrumentation(len(started))

	if len(clientOnly) > 0 {
		names := make([]string, len(clientOnly))
		for i, s := range clientOnly {
			names[i] = s.Name
		}
		fmt.Printf("\n  Outgoing HTTP calls only (client-only instrumentation): %s\n", strings.Join(names, ", "))
	}

	if len(started) == 0 {
		if len(manual) == len(p.Services) {
			return nil // every service ended up with manual steps, which is not an error
		}
		fmt.Println()
		fmt.Println("  No services are running — check the logs above for errors.")
		return errors.New("no services started — all processes failed to start")
	}
	fmt.Println()
	fmt.Println("  Waiting for traffic — send requests to your services to generate traces and metrics.")
	return nil
}

// goSensitiveEnv lists environment variables never passed to launched services.
var goSensitiveEnv = map[string]bool{
	"DT_ACCESS_TOKEN": true, "DT_PLATFORM_TOKEN": true, "DT_API_TOKEN": true,
	"OTEL_EXPORTER_OTLP_HEADERS": true, "OTEL_EXPORTER_OTLP_TRACES_HEADERS": true,
	"OTEL_EXPORTER_OTLP_METRICS_HEADERS": true, "OTEL_EXPORTER_OTLP_LOGS_HEADERS": true,
}

// isDynatraceToken reports values shaped like Dynatrace API (dt0c01.) or
// platform (dt0s16.) tokens, whatever the variable is called.
func isDynatraceToken(value string) bool {
	return strings.HasPrefix(value, "dt0c01.") || strings.HasPrefix(value, "dt0s16.")
}

// goServiceEnv returns the environment of a launched service: the parent
// environment without Dynatrace credentials, plus the OTEL_* variables.
func goServiceEnv(parent []string, base map[string]string, serviceName string) []string {
	vars := maps.Clone(base)
	vars["OTEL_SERVICE_NAME"] = serviceName
	env := make([]string, 0, len(parent)+len(vars))
	for _, kv := range parent {
		key, value, _ := strings.Cut(kv, "=")
		if goSensitiveEnv[strings.ToUpper(key)] || isDynatraceToken(value) {
			continue
		}
		env = append(env, kv)
	}
	return append(env, environment.FormatEnvVars(vars)...)
}

// launchGoService starts one built service from its absolute path.
func launchGoService(svc *GoServicePlan, modulePath string, env []string) (*ManagedProcess, error) {
	logDir := filepath.Join(goOtelDir(modulePath), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create %s: %w", logDir, err)
	}
	logName := svc.Name + ".log"
	logFile, err := os.Create(filepath.Join(logDir, logName))
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	cmd := exec.Command(svc.BinPath)
	cmd.Dir = modulePath
	cmd.Env = env
	mp, err := StartManagedProcess(svc.Name, filepath.Join(".otel", "go", "logs", logName), "", cmd, logFile)
	if err != nil {
		return nil, err
	}
	return mp, nil
}

// printManualSteps prints the walkthrough link and environment for services
// that are not instrumented automatically. The environment points at the local
// collector and carries no Dynatrace token.
func (p *GoInstrumentationPlan) printManualSteps() {
	fmt.Println()
	fmt.Printf("  Project %s can't be autoinstrumented.\n", p.Project.Path)
	fmt.Printf("  For manual instrumentation follow: %s\n", termLink("Instrument your Go application with OpenTelemetry", goWalkthrough))
	fmt.Println()
	fmt.Printf("  Set the following environment variables (local collector at %s):\n\n", p.CollectorEndpoint)
	for _, line := range environment.FormatEnvExportLines(p.EnvVars) {
		fmt.Printf("    %s\n", line)
	}
}
