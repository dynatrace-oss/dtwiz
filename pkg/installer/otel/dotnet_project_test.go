package otel

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func writeDotNetFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scannedBaseNames(projects []ScannedProject) []string {
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, filepath.Base(p.Path))
	}
	sort.Strings(names)
	return names
}

func disableDotNet(t *testing.T) {
	t.Helper()
	featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, false)
}

const dotNetLib = `<Project Sdk="Microsoft.NET.Sdk"/>`

func TestDetectDotNetProjects_SolutionDirectoryIsOneProject(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "shop/Shop.sln", "Microsoft Visual Studio Solution File")
	writeDotNetFile(t, dir, "shop/src/Api/Api.csproj", dotNetLib)
	writeDotNetFile(t, dir, "shop/src/Worker/Worker.csproj", dotNetLib)
	helpers.SetTestWorkingDir(t, dir)

	// Act
	got := detectDotNetProjects(defaultScanRoots())

	// Assert
	if names := scannedBaseNames(got); len(names) != 1 || names[0] != "shop" {
		t.Errorf("projects = %v, want [shop]", names)
	}
}

func TestDetectDotNetProjects_SolutionInScanRootHidesMembers(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Shop.sln", "Microsoft Visual Studio Solution File")
	writeDotNetFile(t, dir, "Api/Api.csproj", dotNetLib)
	helpers.SetTestWorkingDir(t, dir)

	// Act
	got := detectDotNetProjects(defaultScanRoots())

	// Assert
	if len(got) != 1 || len(got[0].Markers) != 1 || got[0].Markers[0] != "Shop.sln" {
		t.Errorf("projects = %+v, want only the solution directory", got)
	}
}

func TestDetectDotNetProjects_StandaloneProjectsWithoutSolution(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "api/Api.csproj", dotNetLib)
	writeDotNetFile(t, dir, "tool/Tool.fsproj", dotNetLib)
	helpers.SetTestWorkingDir(t, dir)

	// Act
	got := detectDotNetProjects(defaultScanRoots())

	// Assert
	if names := scannedBaseNames(got); len(names) != 2 || names[0] != "api" || names[1] != "tool" {
		t.Errorf("projects = %v, want [api tool]", names)
	}
}

func TestDetectDotNetProjects_SolutionAndProjectInOneDirectoryListedOnce(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "app/App.sln", "Microsoft Visual Studio Solution File")
	writeDotNetFile(t, dir, "app/App.csproj", dotNetLib)
	helpers.SetTestWorkingDir(t, dir)

	// Act
	got := detectDotNetProjects(defaultScanRoots())

	// Assert
	if len(got) != 1 || len(got[0].Markers) != 2 {
		t.Errorf("projects = %+v, want one project with two markers", got)
	}
}

func TestDetectDotNetProjects_BuildOutputIgnored(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "bin/Copy/Copy.csproj", dotNetLib)
	writeDotNetFile(t, dir, "obj/Copy/Copy.csproj", dotNetLib)
	writeDotNetFile(t, dir, "packages/Pkg/Pkg.csproj", dotNetLib)
	writeDotNetFile(t, dir, "src/Api/Api.csproj", dotNetLib)
	helpers.SetTestWorkingDir(t, dir)

	// Act
	got := detectDotNetProjects(defaultScanRoots())

	// Assert
	if names := scannedBaseNames(got); len(names) != 1 || names[0] != "Api" {
		t.Errorf("projects = %v, want [Api]", names)
	}
}

func TestDetectDotNetProjects_SymlinkDuplicateListedOnce(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "real/Api.csproj", dotNetLib)
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	helpers.SetTestWorkingDir(t, dir)

	// Act
	got := detectDotNetProjects(defaultScanRoots())

	// Assert
	if len(got) != 1 {
		t.Errorf("projects = %+v, want one", got)
	}
}

func TestDetectDotNetProjects_NoDotnetToolchainNeeded(t *testing.T) {
	// Arrange
	t.Setenv("PATH", "")
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api.csproj", dotNetLib)
	helpers.SetTestWorkingDir(t, dir)

	// Act
	got := detectAllProjects([]runtimeInfo{{name: dotNetRuntimeName, enabled: true, detect: detectDotNetRuntimeProjects}}, defaultScanRoots())

	// Assert
	if len(got) != 1 || got[0].Runtime != dotNetRuntimeName {
		t.Errorf("projects = %+v, want one .NET project", got)
	}
}

func TestDetectAvailableRuntimes_DotNetGatedByFlag(t *testing.T) {
	tests := []struct {
		name string
		on   bool
	}{{"flag off", false}, {"flag on", true}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.on)

			// Act
			runtimes := detectAvailableRuntimes()

			// Assert
			for _, rt := range runtimes {
				if rt.name == dotNetRuntimeName {
					if rt.enabled != tt.on {
						t.Errorf(".NET enabled = %v, want %v", rt.enabled, tt.on)
					}
					return
				}
			}
			t.Error(".NET runtime not registered")
		})
	}
}

func TestDetectAllProjects_FlagOffRunsNoDotNetScan(t *testing.T) {
	// Arrange
	disableDotNet(t)
	scanned := false
	rt := detectAvailableRuntimes()
	for i := range rt {
		if rt[i].name == dotNetRuntimeName {
			rt[i].detect = func([]string) []detectedProject { scanned = true; return nil }
		}
	}

	// Act
	detectAllProjects(rt[len(rt)-1:], nil)

	// Assert
	if scanned {
		t.Error("the .NET scan ran with the flag off")
	}
}

func TestDetectDotNetRuntimeProjects_ListsProjectWithPIDs(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "api/Api.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web"/>`)
	realDir, _ := filepath.EvalSymlinks(dir)
	apiDir := filepath.Join(realDir, "api")
	helpers.SetTestWorkingDir(t, dir)
	original := listProcessesFn
	listProcessesFn = func() []DetectedProcess {
		return []DetectedProcess{
			{PID: 4242, Command: "dotnet run", WorkingDirectory: apiDir},
			{PID: 7, Command: "dotnet build-server", WorkingDirectory: apiDir},
			{PID: 8, Command: "dotnet run", WorkingDirectory: "/elsewhere"},
		}
	}
	t.Cleanup(func() { listProcessesFn = original })

	// Act
	got := detectDotNetRuntimeProjects(defaultScanRoots())

	// Assert
	if len(got) != 1 || len(got[0].RunningProcessIDs) != 1 || got[0].RunningProcessIDs[0] != 4242 {
		t.Errorf("projects = %+v, want one project with PID 4242", got)
	}
}

func TestDetectDotNetRuntimeProjects_ProcessScanFailureStillListsProjects(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api.csproj", dotNetLib)
	helpers.SetTestWorkingDir(t, dir)
	original := listProcessesFn
	listProcessesFn = func() []DetectedProcess { return nil }
	t.Cleanup(func() { listProcessesFn = original })

	// Act
	got := detectDotNetRuntimeProjects(defaultScanRoots())

	// Assert
	if len(got) != 1 || len(got[0].RunningProcessIDs) != 0 {
		t.Errorf("projects = %+v, want one project without PIDs", got)
	}
}

func TestInferRuntimeFromPath_DotNet(t *testing.T) {
	tests := []struct {
		name  string
		on    bool
		files []string
		want  string
	}{
		{"flag off", false, []string{"Api.csproj"}, ""},
		{"flag on", true, []string{"Api.csproj"}, dotNetRuntimeName},
		{"flag on, solution", true, []string{"App.sln"}, dotNetRuntimeName},
		{"package.json beats csproj", true, []string{"package.json", "Api.csproj"}, "Node.js"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.on)
			dir := t.TempDir()
			for _, f := range tt.files {
				writeDotNetFile(t, dir, f, "")
			}

			// Act
			got := inferRuntimeFromPath(dir)

			// Assert
			if got != tt.want {
				t.Errorf("inferRuntimeFromPath = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActiveManualLanguages_DotNetOfferedOnlyWhileFlagOff(t *testing.T) {
	tests := []struct {
		name string
		on   bool
		want bool
	}{{"flag off", false, true}, {"flag on", true, false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.on)

			// Act
			langs := activeManualLanguages()

			// Assert
			found := false
			for _, l := range langs {
				found = found || l.URLSlug == dotNetManualSlug
			}
			if found != tt.want {
				t.Errorf(".NET in manual languages = %v, want %v", found, tt.want)
			}
		})
	}
}
