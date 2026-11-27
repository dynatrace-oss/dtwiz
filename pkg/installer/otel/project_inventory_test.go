package otel

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

// allToolchains makes every runtime binary available.
var allToolchains = map[string]string{"java": "/usr/bin/java", "node": "/usr/bin/node", "go": "/usr/bin/go"}

func realPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// projectKeys renders projects as "runtime path" for compact comparisons.
func projectKeys(inv ProjectInventory) []string {
	keys := make([]string, 0, len(inv.Projects))
	for _, p := range inv.Projects {
		keys = append(keys, string(p.Runtime)+" "+p.Path)
	}
	return keys
}

func runtimeIDs(inv ProjectInventory) []RuntimeID {
	ids := make([]RuntimeID, 0, len(inv.Runtimes))
	for _, r := range inv.Runtimes {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestDetectProjects_DefaultRootIsWorkingDirectory(t *testing.T) {
	// Arrange
	featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, false)
	stubRuntimeToolchain(t, allToolchains, true)
	cwd := t.TempDir()
	writeTree(t, cwd, map[string]string{"gateway/go.mod": "module example.com/gateway\n"})
	helpers.SetTestWorkingDir(t, cwd)

	// Act
	inv, err := DetectProjects(ProjectQuery{})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	root := realPath(t, cwd)
	if !reflect.DeepEqual(inv.Roots, []string{root}) {
		t.Errorf("Roots = %v, want [%s]", inv.Roots, root)
	}
	want := []string{"go " + filepath.Join(root, "gateway")}
	if !reflect.DeepEqual(projectKeys(inv), want) {
		t.Errorf("projects = %v, want %v", projectKeys(inv), want)
	}
	if inv.Projects[0].Module != "example.com/gateway" || inv.Projects[0].Status != Instrumentable {
		t.Errorf("unexpected project details: %+v", inv.Projects[0])
	}
}

func TestDetectProjects_ExplicitDirIsTheOnlyRoot(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, true)
	cwd := t.TempDir()
	writeTree(t, cwd, map[string]string{"cwdapp/go.mod": "module cwd\n"})
	helpers.SetTestWorkingDir(t, cwd)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"other/go.mod": "module other\n"})

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: dir})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"go " + filepath.Join(realPath(t, dir), "other")}
	if !reflect.DeepEqual(projectKeys(inv), want) {
		t.Errorf("projects = %v, want %v", projectKeys(inv), want)
	}
}

func TestDetectProjects_RelativeDirResolvesAgainstWorkingDirectory(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, true)
	cwd := t.TempDir()
	writeTree(t, cwd, map[string]string{"sub/app/go.mod": "module app\n"})
	helpers.SetTestWorkingDir(t, cwd)

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: "sub"})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{filepath.Join(realPath(t, cwd), "sub")}; !reflect.DeepEqual(inv.Roots, want) {
		t.Errorf("Roots = %v, want %v", inv.Roots, want)
	}
}

func TestDetectProjects_HomeAndBundledExamplesNotAdded(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, true)
	home := fakeHome(t)
	writeTree(t, home, map[string]string{
		"homeapp/go.mod": "module homeapp\n",
		".dtwiz/examples/schnitzel/requirements.txt": "flask\n",
	})
	cwd := t.TempDir() // disjoint from home
	if pathsInSameLineage(cwd, home) {
		t.Skipf("temp dirs unexpectedly share lineage (cwd=%q home=%q)", cwd, home)
	}
	helpers.SetTestWorkingDir(t, cwd)

	// Act
	inv, err := DetectProjects(ProjectQuery{})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inv.Projects) != 0 {
		t.Errorf("expected no projects from home or bundled examples, got %v", projectKeys(inv))
	}
}

func TestDetectProjects_BundledExamplesListedWhenTheyAreTheRoot(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, true)
	home := fakeHome(t)
	writeTree(t, home, map[string]string{".dtwiz/examples/schnitzel/requirements.txt": "flask\n"})

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: filepath.Join(home, ".dtwiz", "examples"), Runtimes: []RuntimeID{RuntimePython}})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inv.Projects) != 1 || !strings.HasSuffix(filepath.ToSlash(inv.Projects[0].Path), "schnitzel") {
		t.Errorf("expected the schnitzel project, got %v", projectKeys(inv))
	}
}

func TestDetectProjects_MissingDirectory(t *testing.T) {
	// Arrange
	missing := filepath.Join(t.TempDir(), "nope")

	// Act
	_, err := DetectProjects(ProjectQuery{Dir: missing})

	// Assert
	if err == nil || err.Error() != "directory not found: "+missing {
		t.Errorf("error = %v, want %q", err, "directory not found: "+missing)
	}
}

func TestDetectProjects_RuntimeFilter(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, true)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"billing/pom.xml":  "<project/>",
		"gateway/go.mod":   "module gateway\n",
		"web/package.json": "{}",
	})

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: dir, Runtimes: []RuntimeID{RuntimeJava, RuntimeGo}})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	root := realPath(t, dir)
	want := []string{"java " + filepath.Join(root, "billing"), "go " + filepath.Join(root, "gateway")}
	if !reflect.DeepEqual(projectKeys(inv), want) {
		t.Errorf("projects = %v, want %v", projectKeys(inv), want)
	}
	if got := runtimeIDs(inv); !reflect.DeepEqual(got, []RuntimeID{RuntimeJava, RuntimeGo}) {
		t.Errorf("runtimes = %v, want [java go]", got)
	}
}

func TestDetectProjects_UnavailableRuntimeIsListed(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, false)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"api/requirements.txt": "flask\n"})

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: dir, Runtimes: []RuntimeID{RuntimePython}})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inv.Projects) != 1 || inv.Projects[0].Status != RuntimeUnavailable {
		t.Fatalf("expected one unavailable Python project, got %+v", inv.Projects)
	}
	want := []RuntimeStatus{{ID: RuntimePython, Available: false, Reason: "python3 not found on PATH"}}
	if !reflect.DeepEqual(inv.Runtimes, want) {
		t.Errorf("runtimes = %+v, want %+v", inv.Runtimes, want)
	}
}

func TestDetectProjects_UnavailableRuntimeOutsideFilterIsAbsent(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, map[string]string{"java": "/usr/bin/java"}, true)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"web/package.json": "{}", "billing/pom.xml": "<project/>"})

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: dir, Runtimes: []RuntimeID{RuntimeJava}})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := runtimeIDs(inv); !reflect.DeepEqual(got, []RuntimeID{RuntimeJava}) {
		t.Errorf("runtimes = %v, want [java]", got)
	}
	for _, p := range inv.Projects {
		if p.Runtime != RuntimeJava {
			t.Errorf("unexpected project outside the filter: %+v", p)
		}
	}
}

func TestDetectProjects_DotNetFollowsFeatureFlag(t *testing.T) {
	tests := []struct {
		name         string
		dotNet       bool
		wantProjects int
	}{
		{"flag off hides dotnet", false, 0},
		{"flag on lists dotnet as manual steps", true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.dotNet)
			stubRuntimeToolchain(t, allToolchains, true)
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{"legacy/Legacy.csproj": "<Project Sdk=\"Microsoft.NET.Sdk\"/>"})

			// Act
			inv, err := DetectProjects(ProjectQuery{Dir: dir})

			// Assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(inv.Projects) != tt.wantProjects {
				t.Fatalf("projects = %v, want %d", projectKeys(inv), tt.wantProjects)
			}
			if reported := slicesContainsID(runtimeIDs(inv), RuntimeDotNet); reported != tt.dotNet {
				t.Errorf("runtimes = %v, dotnet reported = %v, want %v", runtimeIDs(inv), reported, tt.dotNet)
			}
			if tt.dotNet && inv.Projects[0].Status != ManualSteps {
				t.Errorf("status = %v, want ManualSteps", inv.Projects[0].Status)
			}
		})
	}
}

func slicesContainsID(ids []RuntimeID, id RuntimeID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestDetectProjects_SortedByPathThenRuntime(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, true)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"web/package.json":     "{}",
		"api/package.json":     "{}",
		"api/requirements.txt": "flask\n",
	})

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: dir, Runtimes: []RuntimeID{RuntimePython, RuntimeNodeJS}})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	root := realPath(t, dir)
	want := []string{
		"nodejs " + filepath.Join(root, "api"),
		"python " + filepath.Join(root, "api"),
		"nodejs " + filepath.Join(root, "web"),
	}
	if !reflect.DeepEqual(projectKeys(inv), want) {
		t.Errorf("projects = %v, want %v", projectKeys(inv), want)
	}
}

func TestDetectProjects_EmptyTree(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, allToolchains, true)
	dir := t.TempDir()

	// Act
	inv, err := DetectProjects(ProjectQuery{Dir: dir})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Projects == nil || len(inv.Projects) != 0 {
		t.Errorf("Projects = %#v, want empty non-nil slice", inv.Projects)
	}
}

func TestDetectProjects_WritesNothingToStdout(t *testing.T) {
	// Arrange
	stubRuntimeToolchain(t, nil, false) // every runtime unavailable
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"api/requirements.txt": "flask\n", "web/package.json": "{}"})

	// Act
	output := helpers.CaptureStdout(t, func() {
		if _, err := DetectProjects(ProjectQuery{Dir: dir}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Assert
	if output != "" {
		t.Errorf("expected no stdout output, got %q", output)
	}
}

func TestDetectProjects_InactiveRuntimeRejected(t *testing.T) {
	// Arrange
	featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, false)

	// Act
	_, err := DetectProjects(ProjectQuery{Dir: t.TempDir(), Runtimes: []RuntimeID{RuntimeDotNet}})

	// Assert
	if err == nil || !strings.Contains(err.Error(), `unknown runtime "dotnet"`) {
		t.Errorf("error = %v, want unknown runtime", err)
	}
}
