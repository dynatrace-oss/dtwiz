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

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/environment"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

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
	Services          []*GoServicePlan
	// ManualReason is set when the whole project is handled with manual steps.
	ManualReason string
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
		plan.runGoPreflight(context.Background())
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
	if tool, ok := goFindOtelcFn(); ok {
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
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	p.stopReplacedProcesses()
	p.buildServices(ctx)

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("interrupted: %w", err)
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
	fmt.Print("  Stopping running processes...\n")
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
	var tool *otelcTool
	needsTool := false
	for _, s := range p.Services {
		needsTool = needsTool || s.Method == goMethodOtelc
	}
	if needsTool {
		var err error
		tool, err = goEnsureOtelcFn()
		if err != nil {
			p.reportToolFailure(err)
			for _, s := range p.Services {
				if s.Method == goMethodOtelc {
					s.Method = goMethodClientOnly
				}
			}
		} else {
			logger.Debug("otelc ready", "path", tool.Path, "version", tool.Version)
			fmt.Printf("  Using otelc %s\n", strings.TrimSpace(tool.Version))
		}
	}

	type moduleFallback struct {
		fb  *goFallback
		err error
	}
	fallbacks := map[string]*moduleFallback{}

	for _, s := range p.Services {
		if ctx.Err() != nil {
			return
		}
		module := s.moduleDir(p.Project.Path)
		if s.Method == goMethodOtelc {
			fmt.Printf("  Building %s with compile-time instrumentation... ", s.Name)
			err := goBuildOtelcFn(ctx, tool, module, s.GoService, s.BinPath)
			if err == nil {
				fmt.Println("done.")
				continue
			}
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			if ctx.Err() != nil {
				return
			}
			fmt.Printf("    Falling back to client-only instrumentation for %s (outgoing HTTP calls only).\n", s.Name)
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
				fmt.Printf("  Building %s with client-only instrumentation... ", s.Name)
				err = mf.fb.buildClientOnly(ctx, s.GoService, s.BinPath)
			}
			if err == nil {
				fmt.Println("done.")
				continue
			}
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			s.Method = goMethodManual
			s.Err = err
		}
	}
	writeGoMarkers(attempted)
}

func (p *GoInstrumentationPlan) reportToolFailure(err error) {
	var tooOld *GoVersionTooOldError
	if errors.As(err, &tooOld) {
		fmt.Printf("  Compile-time instrumentation is not available: %v.\n", tooOld)
	} else {
		fmt.Printf("  Compile-time instrumentation is not available: %v\n", err)
	}
	fmt.Println("  Using client-only instrumentation instead: only outgoing HTTP calls will be traced.")
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
		}
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
		return fmt.Errorf("no services started — all processes failed to start")
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

// printManualSteps prints the packages, environment and walkthrough for
// services that are not instrumented automatically. The environment points at
// the local collector and carries no Dynatrace token.
func (p *GoInstrumentationPlan) printManualSteps() {
	fmt.Println()
	fmt.Printf("  cd %s\n", p.Project.Path)
	fmt.Println()
	fmt.Println("  Add the OTel Go SDK dependencies:")
	for _, pkg := range otelGoPackages {
		fmt.Printf("    go get %s\n", pkg)
	}
	fmt.Println()
	fmt.Printf("  Set the following environment variables (local collector at %s):\n\n", p.CollectorEndpoint)
	for _, line := range environment.FormatEnvExportLines(p.EnvVars) {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
	fmt.Println("  Initialize the OTel SDK in your main() function.")
	fmt.Printf("  See: %s\n", termLink("Instrument your Go application with OpenTelemetry", goWalkthrough))
}
