package otel

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

const testCollectorEndpoint = "http://127.0.0.1:4318"

const (
	dotNetWebNet8 = `<Project Sdk="Microsoft.NET.Sdk.Web"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`
	dotNetExe48   = `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><OutputType>Exe</OutputType><TargetFramework>net48</TargetFramework></PropertyGroup></Project>`
)

// fakeDotnetInstaller records calls and returns canned results.
type fakeDotnetInstaller struct {
	platformErr error
	installed   string
	result      dotnet.Result
	installErr  error
	installs    int
}

func (f *fakeDotnetInstaller) Archive() (dotnet.Archive, error) {
	if f.platformErr != nil {
		return dotnet.Archive{}, f.platformErr
	}
	return dotnet.SelectArchive("linux", "amd64", "glibc")
}

func (f *fakeDotnetInstaller) ActivationEnv() (map[string]string, error) {
	a, err := f.Archive()
	if err != nil {
		return nil, err
	}
	return dotnet.ActivationEnv("/home/u/.opentelemetry/dotnet", a), nil
}

func (f *fakeDotnetInstaller) Install(context.Context) (dotnet.Result, error) {
	f.installs++
	return f.result, f.installErr
}
func (f *fakeDotnetInstaller) Installed() (string, bool) { return f.installed, f.installed != "" }

// stubDotnetInstaller makes plan building use fake.
func stubDotnetInstaller(t *testing.T, fake *fakeDotnetInstaller) {
	t.Helper()
	old := dotnetNewInstallerFn
	t.Cleanup(func() { dotnetNewInstallerFn = old })
	dotnetNewInstallerFn = func() (dotnetInstaller, error) { return fake, nil }
}

func dotnetProject(dir string) detectedProject {
	return detectedProject{ScannedProject: ScannedProject{Path: dir}, Runtime: dotNetRuntimeName}
}

// withStdin makes reads from os.Stdin return input.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig; r.Close() })
}

// plainTerminal makes the terminal look like one without hyperlink support.
func plainTerminal(t *testing.T) {
	t.Helper()
	for _, v := range []string{"TERM_PROGRAM", "WT_SESSION", "VTE_VERSION"} {
		t.Setenv(v, "")
	}
}

const walkthroughBlock = "  Instrument your .NET app:\n    https://dt-url.net/otel-dotnet\n"

// captureAll returns what fn printed to stdout and through the colored status lines.
func captureAll(t *testing.T, fn func()) string {
	t.Helper()
	var colored string
	plain := helpers.CaptureStdout(t, func() { colored = helpers.CaptureOutput(t, fn) })
	return plain + "\n" + colored
}

func listTree(t *testing.T, dir string) string {
	t.Helper()
	var sb strings.Builder
	_ = filepath.Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err == nil {
			sb.WriteString(p + "|" + info.ModTime().String() + ";")
		}
		return nil
	})
	return sb.String()
}

func TestBuildDotnetInstrumentationPlan_MixedServices(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	writeDotNetFile(t, dir, "Legacy/Legacy.csproj", dotNetExe48)
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{})

	// Act
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)

	// Assert
	if plan.ManualReason != "" {
		t.Fatalf("ManualReason = %q, want empty", plan.ManualReason)
	}
	launch := "dotnet " + filepath.Join("Api", "bin", "Debug", "net8.0", "Api.dll") + "  (service: Api)"
	for _, want := range []string{
		"Project: " + dir,
		"Download opentelemetry-dotnet-instrumentation-linux-glibc-x64.zip to ",
		"Verify sha256 against checksums.txt",
		"CORECLR_ENABLE_PROFILING=1",
		"OTEL_EXPORTER_OTLP_ENDPOINT=" + testCollectorEndpoint,
		"Build (without instrumentation):",
		"dotnet build " + filepath.Join("Api", "Api.csproj"),
		launch,
		"Skipped: Legacy (.NET Framework, not supported yet)",
		"Stopping, building and launching are not performed yet",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preview missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Legacy.csproj") || strings.Contains(out, "Legacy.dll") {
		t.Errorf("an unsupported service must have no launch line:\n%s", out)
	}
	if strings.Contains(out, "No running") || strings.Contains(out, "Stop running processes") {
		t.Errorf("no stop line expected without running processes:\n%s", out)
	}
}

func TestBuildDotnetInstrumentationPlan_TwoServicesWithRunningProcesses(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Frontend/Frontend.csproj", dotNetWebNet8)
	writeDotNetFile(t, dir, "Order/Order.csproj", dotNetWebNet8)
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{})
	proj := dotnetProject(dir)
	proj.RunningProcessIDs = []int{4121, 4188}

	// Act
	plan := buildDotnetInstrumentationPlan(proj, testCollectorEndpoint)
	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)

	// Assert
	for _, want := range []string{
		"Stop running processes (PIDs: 4121, 4188)",
		"(service: Frontend)",
		"(service: Order)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preview missing %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "CORECLR_ENABLE_PROFILING"); n != 1 {
		t.Errorf("activation environment appears %d times, want once:\n%s", n, out)
	}
	if strings.Contains(out, "OTEL_SERVICE_NAME") {
		t.Errorf("the service name belongs on the launch lines, not in the shared environment:\n%s", out)
	}
	steps := []string{"Download", "Verify sha256", "Stop running processes", "Build (without instrumentation)", "Environment:", "Launch:"}
	last := -1
	for _, step := range steps {
		at := strings.Index(out, step)
		if at < 0 || at < last {
			t.Fatalf("step %q is missing or out of order (want %v):\n%s", step, steps, out)
		}
		last = at
	}
}

// markBuilt writes a built library and its runtime configuration for the Api service and dates them.
func markBuilt(t *testing.T, dir string, when time.Time) {
	t.Helper()
	writeDotNetFile(t, dir, "Api/bin/Debug/net8.0/Api.dll", "dll")
	writeDotNetFile(t, dir, "Api/bin/Debug/net8.0/Api.runtimeconfig.json", "{}")
	for _, f := range []string{"Api/bin/Debug/net8.0/Api.dll", "Api/bin/Debug/net8.0/Api.runtimeconfig.json"} {
		if err := os.Chtimes(filepath.Join(dir, filepath.FromSlash(f)), when, when); err != nil {
			t.Fatal(err)
		}
	}
}

func dateSources(t *testing.T, dir string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(dir, "Api", "Api.csproj"), when, when); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDotnetInstrumentationPlan_ExistingBuildNeedsNoBuildStep(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	old := time.Now().Add(-2 * time.Hour)
	dateSources(t, dir, old)
	markBuilt(t, dir, old.Add(time.Hour))
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{})

	// Act
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)

	// Assert
	if strings.Contains(out, "Build (without instrumentation)") || strings.Contains(out, "dotnet build") {
		t.Errorf("an existing current build needs no build step:\n%s", out)
	}
	if want := "dotnet " + filepath.Join("Api", "bin", "Debug", "net8.0", "Api.dll") + "  (service: Api, existing build)"; !strings.Contains(out, want) {
		t.Errorf("preview missing %q:\n%s", want, out)
	}
}

func TestBuildDotnetInstrumentationPlan_SourcesNewerThanBuildAreRebuilt(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	now := time.Now()
	markBuilt(t, dir, now.Add(-2*time.Hour))
	dateSources(t, dir, now.Add(-time.Hour))
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{})

	// Act
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)

	// Assert
	want := "dotnet build " + filepath.Join("Api", "Api.csproj") + "  (sources changed since the last build)"
	if !strings.Contains(out, want) {
		t.Errorf("preview missing %q:\n%s", want, out)
	}
	if strings.Contains(out, "existing build") {
		t.Errorf("a stale build must not be marked as an existing build:\n%s", out)
	}
}

func TestBuildDotnetInstrumentationPlan_OnlyServicesWithoutBuildAreBuilt(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	writeDotNetFile(t, dir, "Web/Web.csproj", dotNetWebNet8)
	old := time.Now().Add(-2 * time.Hour)
	for _, f := range []string{"Api/Api.csproj", "Web/Web.csproj"} {
		if err := os.Chtimes(filepath.Join(dir, filepath.FromSlash(f)), old, old); err != nil {
			t.Fatal(err)
		}
	}
	markBuilt(t, dir, old.Add(time.Hour))
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{})

	// Act
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)

	// Assert
	if !strings.Contains(out, "dotnet build "+filepath.Join("Web", "Web.csproj")) || strings.Contains(out, "dotnet build "+filepath.Join("Api", "Api.csproj")) {
		t.Errorf("only Web needs a build:\n%s", out)
	}
}

func TestBuildDotnetInstrumentationPlan_InstalledVersionShown(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{installed: "1.19.0"})

	// Act
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)

	// Assert
	if !strings.Contains(out, "Use installed OpenTelemetry .NET instrumentation 1.19.0 in") {
		t.Errorf("preview should show the installed version:\n%s", out)
	}
	for _, unwanted := range []string{"Download ", "Verify sha256", "newer release"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("an installed version needs no %q line:\n%s", unwanted, out)
		}
	}
}

func TestBuildDotnetInstrumentationPlan_ManualOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		dotnet     bool
		runtimes   string
		platform   error
		wantReason string
		wantInOut  []string
	}{
		{name: "dotnet missing",
			files:      map[string]string{"Api/Api.csproj": dotNetWebNet8},
			wantReason: ".NET is not installed",
			wantInOut:  []string{"Skipped: Api (.NET is not installed)", dotNetDownloadURL}},
		{name: "runtime missing for every service",
			files: map[string]string{"Api/Api.csproj": dotNetWebNet8}, dotnet: true,
			runtimes:   "Microsoft.NETCore.App 6.0.30 [/x]\n",
			wantReason: "none of the services can be instrumented automatically",
			wantInOut:  []string{"Skipped: Api (requires .NET 8 runtime, not installed)"}},
		{name: "all services unsupported",
			files: map[string]string{"Legacy/Legacy.csproj": dotNetExe48}, dotnet: true, runtimes: runtimes8,
			wantReason: "none of the services can be instrumented automatically",
			wantInOut:  []string{"Skipped: Legacy (.NET Framework, not supported yet)"}},
		{name: "library only",
			files: map[string]string{"Lib.csproj": dotNetLib}, dotnet: true, runtimes: runtimes8,
			wantReason: "no runnable .NET service was found in the project"},
		{name: "unsupported platform",
			files: map[string]string{"Api/Api.csproj": dotNetWebNet8}, dotnet: true, runtimes: runtimes8,
			platform:   &dotnet.UnsupportedPlatformError{Reason: "macOS on amd64 (Intel)"},
			wantReason: "macOS on amd64 (Intel) is not supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			plainTerminal(t)
			dir := t.TempDir()
			for rel, content := range tt.files {
				writeDotNetFile(t, dir, rel, content)
			}
			stubDotNet(t, tt.dotnet, tt.runtimes, nil)
			fake := &fakeDotnetInstaller{platformErr: tt.platform}
			stubDotnetInstaller(t, fake)

			// Act
			plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
			out := helpers.CaptureStdout(t, plan.PrintPlanSteps)
			execOut := helpers.CaptureStdout(t, func() {
				if err := plan.Execute(); err != nil {
					t.Errorf("Execute() error = %v, want nil for a manual outcome", err)
				}
			})

			// Assert
			if !strings.Contains(plan.ManualReason, tt.wantReason) {
				t.Errorf("ManualReason = %q, want it to contain %q", plan.ManualReason, tt.wantReason)
			}
			for _, want := range append(tt.wantInOut, "Method:  manual", "Not instrumented automatically") {
				if !strings.Contains(out+execOut, want) {
					t.Errorf("output missing %q:\n%s%s", want, out, execOut)
				}
			}
			if fake.installs != 0 {
				t.Errorf("Install called %d times, want 0 for a manual outcome", fake.installs)
			}
			if !strings.Contains(execOut, "Not instrumented automatically: "+plan.ManualReason+".\n"+walkthroughBlock) {
				t.Errorf("the walkthrough must follow the reason:\n%s", execOut)
			}
		})
	}
}

func TestDotnetPlanPrintPlanSteps_CreatesNothingAndShowsNoCredential(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{})
	t.Setenv("DT_PLATFORM_TOKEN", "dt0s16.SECRETVALUE")
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
	before := listTree(t, dir)

	// Act
	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)

	// Assert
	if after := listTree(t, dir); after != before {
		t.Errorf("project files changed:\nbefore %s\nafter  %s", before, after)
	}
	if strings.Contains(out, "SECRETVALUE") {
		t.Errorf("preview leaks a credential:\n%s", out)
	}
}

func TestDotnetPlanExecute_InstallsAndEndsInManualSteps(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	stubDotNet(t, true, runtimes8, nil)
	fake := &fakeDotnetInstaller{result: dotnet.Result{Dir: "/home/u/.opentelemetry/dotnet", Version: "1.20.0"}}
	stubDotnetInstaller(t, fake)
	plainTerminal(t)
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)
	before := listTree(t, dir)

	// Act
	var err error
	out := captureAll(t, func() { err = plan.Execute() })

	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if fake.installs != 1 {
		t.Errorf("Install called %d times, want 1", fake.installs)
	}
	for _, want := range []string{"1.20.0 installed", "/home/u/.opentelemetry/dotnet", "Automatic launch of .NET services is not available yet"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if after := listTree(t, dir); after != before {
		t.Errorf("project files changed")
	}
	statement := "Automatic launch of .NET services is not available yet; manual steps follow.\n"
	if !strings.Contains(out, statement+walkthroughBlock) {
		t.Errorf("the walkthrough must directly follow the statement:\n%s", out)
	}
	if n := strings.Count(out, "dt-url.net/otel-dotnet"); n != 1 {
		t.Errorf("walkthrough link shown %d times, want once:\n%s", n, out)
	}
}

func TestDotnetPlanExecute_WalkthroughIsAHyperlinkInCapableTerminals(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{result: dotnet.Result{Dir: "/d", Version: "1.20.0"}})
	t.Setenv("TERM_PROGRAM", "vscode")
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)

	// Act
	out := captureAll(t, func() { _ = plan.Execute() })

	// Assert
	if want := "\x1b]8;;https://dt-url.net/otel-dotnet\x1b\\Instrument your .NET app\x1b]8;;\x1b\\"; !strings.Contains(out, want) {
		t.Errorf("the text itself should be the OSC 8 hyperlink:\n%q", out)
	}
	if strings.Contains(out, "→") {
		t.Errorf("no separate address line is needed when the text is a link:\n%q", out)
	}
}

func TestDotnetPlanExecute_ReusedInstallation(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	stubDotNet(t, true, runtimes8, nil)
	stubDotnetInstaller(t, &fakeDotnetInstaller{result: dotnet.Result{Dir: "/d", Version: "1.20.0", Reused: true}})
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)

	// Act
	out := captureAll(t, func() { _ = plan.Execute() })

	// Assert
	if !strings.Contains(out, "1.20.0 already installed") {
		t.Errorf("output should say the installation is already installed:\n%s", out)
	}
}

func TestDotnetPlanExecute_FailureNamesTheStage(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetWebNet8)
	stubDotNet(t, true, runtimes8, nil)
	cause := &dotnet.InstallError{Stage: dotnet.StageVerification, Err: dotnet.ErrChecksumMismatch}
	stubDotnetInstaller(t, &fakeDotnetInstaller{installErr: cause})
	plan := buildDotnetInstrumentationPlan(dotnetProject(dir), testCollectorEndpoint)

	// Act
	var err error
	captureAll(t, func() { err = plan.Execute() })

	// Assert
	var ie *dotnet.InstallError
	if !errors.As(err, &ie) || ie.Stage != dotnet.StageVerification {
		t.Errorf("Execute() error = %v, want the verification-stage InstallError", err)
	}
}
