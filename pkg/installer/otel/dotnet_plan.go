package otel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/environment"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// dotNetManualSlug is the manual-languages slug of .NET; the manual-languages
// list hides .NET while the flag is on, because it is offered as a project.
const dotNetManualSlug = "dotnet"

// dotNetWalkthrough is the Dynatrace walkthrough for instrumenting a .NET application.
const dotNetWalkthrough = "https://dt-url.net/otel-dotnet"

// dotnetInstaller installs the upstream instrumentation; *dotnet.Installer
// implements it and tests replace it.
type dotnetInstaller interface {
	Archive() (dotnet.Archive, error)
	ActivationEnv() (map[string]string, error)
	Install(ctx context.Context) (dotnet.Result, error)
	Installed() (version string, ok bool)
}

// Seams replaced in tests.
var dotnetNewInstallerFn = newDotnetInstaller

func newDotnetInstaller() (dotnetInstaller, error) {
	root, err := dotnet.DefaultRoot()
	if err != nil {
		return nil, err
	}
	return &dotnet.Installer{
		Root:   root,
		Source: dotnet.DefaultSource(),
		Client: httpClient,
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
		Libc:   func() string { return dotnet.DetectLibc(runCombinedOutput) },
		Attestation: dotnet.Attestation{
			LookPath: func(file string) (string, error) { return installer.ExecLookPath(file) },
			Run:      runCombinedOutput,
		},
	}, nil
}

// runCombinedOutput runs a command and returns stdout and stderr together.
func runCombinedOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

// DotnetServicePlan is one service of a .NET project and what the plan does with it.
type DotnetServicePlan struct {
	dotnet.Service
	// Instrumented is true when the service will be instrumented.
	Instrumented bool
	// Note says why the service is skipped, or qualifies an instrumented one.
	Note string
	// ServiceName is the service name the instrumented service reports.
	ServiceName string
	// Build says which library the service is launched from and whether it is
	// built first. Only set for instrumented services.
	Build dotnet.BuildInfo
	// LaunchCommand is the command that starts the service, run from the
	// project directory with the plan's environment.
	LaunchCommand string
}

// DotnetInstrumentationPlan prepares the machine for instrumenting a .NET
// project: it checks prerequisites and installs the OpenTelemetry .NET
// automatic instrumentation. Launching the services is a later step, so a
// successful execution still ends in the manual-steps outcome.
type DotnetInstrumentationPlan struct {
	Project  ScannedProject
	Services []DotnetServicePlan
	// ManualReason is set when nothing can be instrumented automatically.
	ManualReason string
	// Dir is the install directory of the instrumentation.
	Dir string
	// ArchiveName is the release archive for this machine.
	ArchiveName string
	// EnvVars is the environment the instrumented services are launched with,
	// without the per-service OTEL_SERVICE_NAME.
	EnvVars map[string]string
	// InstalledVersion is the version found in Dir when the plan was built, or "".
	InstalledVersion string

	installer dotnetInstaller
}

// Runtime returns the runtime name shown in the flow.
func (p *DotnetInstrumentationPlan) Runtime() string { return dotNetRuntimeName }

// buildDotnetInstrumentationPlan checks the prerequisites for the services of
// the project and builds the plan. It uses no network and changes nothing.
func buildDotnetInstrumentationPlan(proj detectedProject, collectorEndpoint string) *DotnetInstrumentationPlan {
	plan := &DotnetInstrumentationPlan{Project: proj.ScannedProject}

	services := dotNetServices(proj.ScannedProject)
	if len(services) == 0 {
		plan.ManualReason = "no runnable .NET service was found in the project"
		return plan
	}

	prereq := checkDotNetPrerequisites(services)
	for _, c := range prereq.Services {
		sp := DotnetServicePlan{Service: c.Service, Instrumented: c.Instrumented, Note: c.Note}
		if c.Instrumented {
			sp.ServiceName = environment.NormalizeServiceName(c.Service.Name)
		}
		plan.Services = append(plan.Services, sp)
	}
	if prereq.DotnetMissing {
		plan.ManualReason = ".NET is not installed (dotnet was not found on PATH); install it from " + dotNetDownloadURL
		return plan
	}

	inst, err := dotnetNewInstallerFn()
	if err != nil {
		plan.ManualReason = err.Error()
		return plan
	}
	archive, err := inst.Archive()
	if err != nil {
		plan.ManualReason = err.Error()
		return plan
	}
	if !plan.anyInstrumented() {
		plan.ManualReason = "none of the services can be instrumented automatically"
		return plan
	}

	plan.installer = inst
	plan.ArchiveName = archive.Name
	plan.resolveBuilds(proj.Path)
	if dir, err := dotnet.DefaultRoot(); err == nil {
		plan.Dir = dir
		plan.EnvVars = environment.GenerateBaseOtelEnvVars(collectorEndpoint, "")
		delete(plan.EnvVars, "OTEL_SERVICE_NAME")
		for k, v := range dotnet.ActivationEnv(dir, archive) {
			plan.EnvVars[k] = v
		}
	}
	if v, ok := inst.Installed(); ok {
		plan.InstalledVersion = v
	}
	logger.Debug("dotnet instrumentation plan built", "project", proj.Path, "services", len(plan.Services), "installed", plan.InstalledVersion)
	return plan
}

func (p *DotnetInstrumentationPlan) anyInstrumented() bool {
	for _, s := range p.Services {
		if s.Instrumented {
			return true
		}
	}
	return false
}

// resolveBuilds decides, for each instrumented service, whether it must be
// built and which library it is launched from.
func (p *DotnetInstrumentationPlan) resolveBuilds(projectDir string) {
	newest, complete := dotnet.NewestSource(projectDir)
	for i := range p.Services {
		s := &p.Services[i]
		if !s.Instrumented {
			continue
		}
		s.Build = dotnet.ResolveBuild(s.Service, newest, complete)
		s.LaunchCommand = dotnetLaunchCommand(projectDir, s.Build.Library, s.Service)
	}
}

// PrintPlanSteps lists what the plan will do, in order: download and verify,
// stop, build, and launch each supported service. It creates and changes
// nothing. Stopping, building and launching are not performed yet, which the
// last line says.
func (p *DotnetInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project: %s\n", p.Project.Path)
	if p.ManualReason != "" {
		p.printSkipped()
		fmt.Printf("     Method:  manual — %s\n", p.ManualReason)
		fmt.Println("     The application is not instrumented automatically; manual steps are printed after the collector is installed.")
		fmt.Println("     The OTel Collector is still installed.")
		return
	}

	if p.InstalledVersion != "" {
		fmt.Printf("     Use installed OpenTelemetry .NET instrumentation %s in %s\n", p.InstalledVersion, p.Dir)
	} else {
		fmt.Printf("     Download %s to %s\n", p.ArchiveName, p.Dir)
		fmt.Println("     Verify sha256 against checksums.txt, and the release attestation when the GitHub CLI is signed in")
	}
	if len(p.Project.RunningProcessIDs) > 0 {
		pidStrs := make([]string, len(p.Project.RunningProcessIDs))
		for i, pid := range p.Project.RunningProcessIDs {
			pidStrs[i] = strconv.Itoa(pid)
		}
		fmt.Printf("     Stop running processes (PIDs: %s)\n", strings.Join(pidStrs, ", "))
	}
	p.printBuilds()
	fmt.Println("     Environment:")
	for _, line := range environment.FormatPrintableEnvVars(p.EnvVars) {
		fmt.Printf("       %s\n", line)
	}
	fmt.Println("     Launch:")
	for _, s := range p.Services {
		if !s.Instrumented {
			continue
		}
		detail := "service: " + s.ServiceName
		if s.Build.Existing {
			detail += ", existing build"
		}
		fmt.Printf("       %s  (%s)\n", s.LaunchCommand, detail)
	}
	p.printSkipped()
	fmt.Println("     Stopping, building and launching are not performed yet; manual steps follow.")
}

// printBuilds lists the services that are built before they are launched. The
// build runs without the instrumentation environment.
func (p *DotnetInstrumentationPlan) printBuilds() {
	header := false
	for _, s := range p.Services {
		if !s.Instrumented || !s.Build.NeedsBuild() {
			continue
		}
		if !header {
			fmt.Println("     Build (without instrumentation):")
			header = true
		}
		line := "dotnet build " + relativeTo(p.Project.Path, s.ProjectFile)
		if s.Build.Stale {
			line += "  (sources changed since the last build)"
		}
		fmt.Printf("       %s\n", line)
	}
}

// printSkipped lists the services that are not instrumented, with the reason.
func (p *DotnetInstrumentationPlan) printSkipped() {
	for _, s := range p.Services {
		if !s.Instrumented {
			fmt.Printf("     Skipped: %s (%s)\n", s.Name, s.Note)
		}
	}
}

// relativeTo returns path relative to base, or path itself when that fails.
func relativeTo(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return path
}

// dotnetLaunchCommand is the command that starts a service from its built
// library; library is empty when the build output cannot be predicted.
func dotnetLaunchCommand(projectDir, library string, s dotnet.Service) string {
	if library == "" {
		return "dotnet <build output>/" + s.AssemblyName + ".dll"
	}
	return "dotnet " + relativeTo(projectDir, library)
}

// Execute installs (or reuses) the instrumentation and ends in the manual-steps
// outcome. A manual plan is not an error. It stops and starts no process.
func (p *DotnetInstrumentationPlan) Execute() error {
	if p.ManualReason != "" {
		fmt.Printf("  Not instrumented automatically: %s.\n", p.ManualReason)
		printDotNetWalkthrough()
		return nil
	}

	// Ctrl+C cancels the download; the installer removes its staging directory.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := p.installer.Install(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("interrupted: %w", ctx.Err())
		}
		display.PrintStatusLine("error", err.Error(), display.ColorError)
		return fmt.Errorf("installing OpenTelemetry .NET instrumentation: %w", err)
	}

	if res.Reused {
		display.PrintStatusLine("ok", fmt.Sprintf("OpenTelemetry .NET instrumentation %s already installed (%s)", res.Version, res.Dir), display.ColorOK)
	} else {
		display.PrintStatusLine("ok", fmt.Sprintf("OpenTelemetry .NET instrumentation %s installed (%s)", res.Version, res.Dir), display.ColorOK)
	}
	fmt.Println("  Automatic launch of .NET services is not available yet; manual steps follow.")
	printDotNetWalkthrough()
	return nil
}

// printDotNetWalkthrough shows "Instrument your .NET app" as a link to the
// walkthrough; terminals without link support get the address below the text.
func printDotNetWalkthrough() {
	fmt.Printf("  %s\n", termLink("Instrument your .NET app", dotNetWalkthrough))
}

// compile-time check that the plan satisfies the runtime plan interface.
var _ InstrumentationPlan = (*DotnetInstrumentationPlan)(nil)
