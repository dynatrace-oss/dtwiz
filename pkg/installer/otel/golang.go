package otel

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/environment"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// otelGoPackages are fetched into the isolated .otel/go.mod (never the project's own go.mod).
var otelGoPackages = []string{
	"github.com/Dynatrace/OneAgent-SDK-for-Go",
	"go.opentelemetry.io/otel",
	"go.opentelemetry.io/otel/sdk",
	"go.opentelemetry.io/otel/sdk/metric",
	"go.opentelemetry.io/otel/sdk/log",
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp",
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp",
	"go.opentelemetry.io/contrib/bridges/otelslog",
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
	"go.opentelemetry.io/contrib/instrumentation/runtime",
}

const goOtelDirName = ".otel"

type GoProject struct {
	ScannedProject
	ModuleName string
}

func extractGoModuleName(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		logger.Warn("failed to read go.mod", "path", goModPath, "err", err)
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			logger.Debug("go module name extracted", "path", goModPath, "module", mod)
			return mod
		}
	}
	return ""
}

func detectGoProjects(roots []string) []GoProject {
	scanned := scanProjectDirs([]string{"go.mod"}, nil, roots)
	projects := make([]GoProject, 0, len(scanned))
	for _, s := range scanned {
		moduleName := extractGoModuleName(filepath.Join(s.Path, "go.mod"))
		projects = append(projects, GoProject{
			ScannedProject: s,
			ModuleName:     moduleName,
		})
	}
	logger.Debug("Go projects detected", "count", len(projects))
	return projects
}

func detectGoProcesses() []DetectedProcess {
	return detectProcesses("go run", nil)
}

type GoInstrumentationPlan struct {
	Project  GoProject
	EnvVars  map[string]string
	Packages []goMainPackage
	// Otelc selects compile-time instrumentation (otelc go build): server spans, connected
	// traces and library coverage. When false (Go < 1.25 or otelc unavailable) the build
	// falls back to an injected init() that only traces outgoing HTTP calls.
	Otelc bool
}

func (p *GoInstrumentationPlan) Runtime() string { return "Go" }

// buildGoInstrumentationPlan returns nil when the project has no `package main` to build.
func buildGoInstrumentationPlan(proj GoProject, collectorEndpoint string) *GoInstrumentationPlan {
	var pkgs []goMainPackage
	skipped := 0
	for _, mp := range detectGoMainPackages(proj.Path, environment.ProjectServiceName(proj.Path)) {
		if goModuleUsesOtelSDK(mp.ModuleDir) {
			fmt.Printf("  Skipping %s — it already sets up the OpenTelemetry SDK itself.\n", mp.ServiceName)
			skipped++
			continue
		}
		pkgs = append(pkgs, mp)
	}
	if len(pkgs) == 0 && skipped > 0 {
		fmt.Println("  Nothing to instrument: the services are already instrumented. Point their OTLP exporter at the")
		fmt.Println("  collector (gRPC 127.0.0.1:4317, HTTP http://127.0.0.1:4318) to send data to Dynatrace.")
		return nil
	}
	if len(pkgs) == 0 {
		fmt.Printf("  No main package found in %s (searched the module and any go.work modules) — nothing to instrument.\n", proj.Path)
		return nil
	}
	return &GoInstrumentationPlan{
		Project:  proj,
		EnvVars:  environment.GenerateBaseOtelEnvVars(collectorEndpoint, pkgs[0].ServiceName),
		Packages: pkgs,
		Otelc:    goToolchainSupportsOtelc(),
	}
}

func DetectGoPlan(collectorEndpoint string) *GoInstrumentationPlan {
	if _, err := exec.LookPath("go"); err != nil {
		logger.Debug("go not found on PATH", "skipping Go instrumentation")
		return nil
	}

	projects, processes := runInParallel(func() []GoProject {
		return detectGoProjects(defaultScanRoots())
	}, detectGoProcesses)
	if len(projects) == 0 {
		logger.Debug("no Go projects detected", "skipping Go instrumentation")
		return nil
	}

	scanned := make([]ScannedProject, len(projects))
	for i := range projects {
		scanned[i] = projects[i].ScannedProject
	}

	matchProcessesToProjects(scanned, processes)

	sel := promptProjectSelection("Go", scanned)
	if sel == nil {
		return nil
	}

	for _, p := range projects {
		if p.Path == sel.Path {
			p.ScannedProject = *sel
			return buildGoInstrumentationPlan(p, collectorEndpoint)
		}
	}
	return nil
}

// goModuleUsesOtelSDK reports whether the module directly requires the OpenTelemetry SDK or an
// OTLP exporter, i.e. the application already sets up its own OTel pipeline. Injecting a second
// init would fight with it, and upgrading its pinned OTel versions can break its own code.
func goModuleUsesOtelSDK(moduleDir string) bool {
	data, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "require "))
		if strings.Contains(line, "// indirect") {
			continue
		}
		if strings.HasPrefix(line, "go.opentelemetry.io/otel/sdk") || strings.HasPrefix(line, "go.opentelemetry.io/otel/exporters/") {
			return true
		}
	}
	return false
}

// goModuleLabel is the module path relative to the project, as a "dir/" prefix; empty for the project root.
func goModuleLabel(projectPath, moduleDir string) string {
	rel, err := filepath.Rel(projectPath, moduleDir)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel) + "/"
}

func goBinaryName(svcName string) string {
	if runtime.GOOS == "windows" {
		return svcName + ".exe"
	}
	return svcName
}

func (p *GoInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project:    %s\n", p.Project.Path)
	if p.Project.ModuleName != "" {
		fmt.Printf("     Module:     %s\n", p.Project.ModuleName)
	}
	if p.Otelc {
		fmt.Println("     Method:     compile-time instrumentation (otelc go build) — server and client spans, project sources are not modified")
		fmt.Printf("     Tooling:    otelc (%s), installed to ~/.opentelemetry/go/bin if not on PATH\n", otelcInstallPath)
		fmt.Println("     Entrypoints (built with otelc and launched with OTEL_* settings):")
		for _, mp := range p.Packages {
			fmt.Printf("       ./%s  →  service %s\n", mp.RelDir, mp.ServiceName)
		}
		return
	}
	fmt.Printf("     Isolation:  %s/ (go.mod copy + generated init; project sources are not modified)\n", goOtelDirName)
	fmt.Println("     Dependencies (go get into .otel/go.mod):")
	for _, pkg := range otelGoPackages {
		fmt.Printf("       %s\n", pkg)
	}
	fmt.Println("     Entrypoints (built and launched with the OTel SDK initialized in init()):")
	for _, mp := range p.Packages {
		fmt.Printf("       ./%s  →  service %s\n", mp.RelDir, mp.ServiceName)
	}
}

// goCmd builds a `go` command running in the project directory against the isolated
// .otel/go.mod. GOWORK=off because -modfile cannot be combined with workspace mode.
func goCmd(moduleDir string, args ...string) *exec.Cmd {
	otelDir := filepath.Join(moduleDir, goOtelDirName)
	full := append([]string{args[0], "-modfile=" + filepath.Join(otelDir, "go.mod")}, args[1:]...)
	cmd := exec.Command("go", full...)
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	return cmd
}

func runGo(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", strings.Join(cmd.Args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// prepareGoOtelDir creates the module's .otel/ with a copy of its go.mod/go.sum. Each module needs
// its own copy: they have different module paths and pinned dependencies.
func prepareGoOtelDir(moduleDir string) error {
	otelDir := filepath.Join(moduleDir, goOtelDirName)
	if err := os.MkdirAll(otelDir, 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(otelDir, "go.mod"), data, 0o644); err != nil {
		return err
	}
	if sum, err := os.ReadFile(filepath.Join(moduleDir, "go.sum")); err == nil {
		if err := os.WriteFile(filepath.Join(otelDir, "go.sum"), sum, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writeGoSharedFiles writes, once per project in <project>/.otel/, the init file shared by every
// service and a single build overlay mapping it into each main package directory. It returns the
// overlay path. Entries for other services are harmless when building one package.
func writeGoSharedFiles(projectPath string, pkgs []goMainPackage) (string, error) {
	otelDir := filepath.Join(projectPath, goOtelDirName)
	if err := os.MkdirAll(otelDir, 0o755); err != nil {
		return "", err
	}
	initPath := filepath.Join(otelDir, goInitFileName)
	if err := os.WriteFile(initPath, []byte(goInitSource), 0o644); err != nil {
		return "", err
	}
	replace := make(map[string]string, len(pkgs))
	for _, mp := range pkgs {
		replace[filepath.Join(mp.ModuleDir, filepath.FromSlash(mp.PkgDir), goInitFileName)] = initPath
	}
	data, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		return "", err
	}
	overlayPath := filepath.Join(otelDir, "overlay.json")
	return overlayPath, os.WriteFile(overlayPath, data, 0o644)
}

func (p *GoInstrumentationPlan) prepareOverlayBuild() (string, error) {
	proj := p.Project
	overlayPath, err := writeGoSharedFiles(proj.Path, p.Packages)
	if err != nil {
		return "", fmt.Errorf("writing shared build files: %w", err)
	}

	modules := map[string]bool{}
	for _, mp := range p.Packages {
		if modules[mp.ModuleDir] {
			continue
		}
		modules[mp.ModuleDir] = true
		label := goModuleLabel(proj.Path, mp.ModuleDir)

		fmt.Printf("  Preparing %s%s/ ... ", label, goOtelDirName)
		if err := prepareGoOtelDir(mp.ModuleDir); err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			return "", fmt.Errorf("creating %s/ directory: %w", goOtelDirName, err)
		}
		fmt.Println("done.")

		fmt.Printf("  Installing OTel packages for %s(go get)... ", label)
		if err := runGo(goCmd(mp.ModuleDir, append([]string{"get"}, otelGoPackages...)...)); err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			return "", fmt.Errorf("installing OTel packages (%s/%s): %w", runtime.GOOS, runtime.GOARCH, err)
		}
		fmt.Println("done.")
	}

	return overlayPath, nil
}

func (p *GoInstrumentationPlan) Execute() error {
	proj := p.Project

	if len(proj.RunningProcessIDs) > 0 {
		fmt.Print("  Stopping running processes... ")
		stopProcesses(proj.RunningProcessIDs)
		fmt.Println("done.")
	}

	var otelcBin, overlayPath string
	if p.Otelc {
		fmt.Print("  Preparing otelc... ")
		bin, err := ensureOtelc()
		if err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			fmt.Println("  Falling back to SDK injection (outgoing HTTP calls only).")
			p.Otelc = false
		} else {
			otelcBin = bin
			fmt.Println("done.")
		}
	}

	if !p.Otelc {
		var err error
		overlayPath, err = p.prepareOverlayBuild()
		if err != nil {
			return err
		}
	}

	var procs []*ManagedProcess
	for _, mp := range p.Packages {
		binDir := filepath.Join(mp.ModuleDir, goOtelDirName, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", binDir, err)
		}
		binPath := filepath.Join(binDir, goBinaryName(mp.ServiceName))
		fmt.Printf("  Building %s (./%s)... ", mp.ServiceName, mp.RelDir)
		var buildErr error
		if p.Otelc {
			buildErr = buildWithOtelc(otelcBin, mp, binPath)
		} else {
			buildErr = runGo(goCmd(mp.ModuleDir, "build", "-overlay="+overlayPath, "-o", binPath, "./"+mp.PkgDir))
		}
		if err := buildErr; err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			return fmt.Errorf("building %s: %w", mp.ServiceName, err)
		}
		fmt.Println("done.")

		epEnvVars := maps.Clone(p.EnvVars)
		epEnvVars["OTEL_SERVICE_NAME"] = mp.ServiceName
		cmd := exec.Command(binPath)
		cmd.Dir = mp.ModuleDir
		cmd.Env = append(os.Environ(), environment.FormatEnvVars(epEnvVars)...)
		if started := launchEntrypoint(mp.ServiceName, mp.ModuleDir, mp.RelDir, cmd); started != nil {
			procs = append(procs, started)
		}
	}

	fmt.Println()
	startedServices, _ := PrintProcessSummary(procs, processSettleDelay)
	if len(startedServices) == 0 {
		fmt.Println()
		fmt.Println("  No services are running — check the logs above for errors.")
		return fmt.Errorf("no services started — all processes failed to start")
	}
	if len(startedServices) < len(procs) {
		return fmt.Errorf("%d of %d service(s) failed to start — check the logs above for errors", len(procs)-len(startedServices), len(procs))
	}

	fmt.Println()
	fmt.Println("  Waiting for traffic — send requests to your services to generate traces and metrics.")
	if p.Otelc {
		fmt.Println("  Incoming and outgoing HTTP, gRPC and database calls are traced at compile time (otelc);")
		fmt.Println("  see https://opentelemetry.io/docs/zero-code/go/compile-time/")
		return nil
	}
	fmt.Println("  Incoming HTTP requests are traced once handlers are wrapped with otelhttp.NewHandler;")
	fmt.Println("  see https://docs.dynatrace.com/docs/ingest-from/opentelemetry/walkthroughs/go")
	return nil
}
