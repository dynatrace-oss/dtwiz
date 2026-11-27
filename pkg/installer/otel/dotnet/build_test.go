package dotnet_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

var (
	t0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Hour)
	t2 = t0.Add(2 * time.Hour)
)

// touch sets the modification time of dir/rel.
func touch(t *testing.T, dir, rel string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(dir, filepath.FromSlash(rel)), when, when); err != nil {
		t.Fatal(err)
	}
}

// builtApp writes a library and its runtime configuration for config/framework and dates the library.
func builtApp(t *testing.T, dir, config, framework, name string, when time.Time) string {
	t.Helper()
	rel := "bin/" + config + "/" + framework + "/"
	lib := writeFile(t, dir, rel+name+".dll", "dll")
	writeFile(t, dir, rel+name+".runtimeconfig.json", "{}")
	touch(t, dir, rel+name+".dll", when)
	return lib
}

func appService(dir string, frameworks ...string) dotnet.Service {
	return dotnet.Service{Name: "App", AssemblyName: "App", ProjectFile: filepath.Join(dir, "App.csproj"), Dir: dir, Frameworks: frameworks}
}

func TestResolveBuild_ExistingCurrentBuildIsReused(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	lib := builtApp(t, dir, "Debug", "net8.0", "App", t1)
	writeFile(t, dir, "Program.cs", "")
	touch(t, dir, "Program.cs", t0)
	newest, complete := dotnet.NewestSource(dir)

	// Act
	got := dotnet.ResolveBuild(appService(dir, "net8.0"), newest, complete)

	// Assert
	if !got.Existing || got.Stale || got.NeedsBuild() || got.Library != lib {
		t.Errorf("ResolveBuild() = %+v, want an existing build at %s", got, lib)
	}
}

func TestResolveBuild_NoBuildYetPredictsDefaultOutput(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "Program.cs", "")
	newest, complete := dotnet.NewestSource(dir)

	// Act
	got := dotnet.ResolveBuild(appService(dir, "net8.0"), newest, complete)

	// Assert
	want := filepath.Join(dir, "bin", "Debug", "net8.0", "App.dll")
	if !got.NeedsBuild() || got.Existing || got.Stale || got.Library != want {
		t.Errorf("ResolveBuild() = %+v, want a build producing %s", got, want)
	}
}

func TestResolveBuild_SourcesNewerThanLibraryNeedBuild(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	lib := builtApp(t, dir, "Debug", "net8.0", "App", t0)
	writeFile(t, dir, "Program.cs", "")
	touch(t, dir, "Program.cs", t1)
	newest, complete := dotnet.NewestSource(dir)

	// Act
	got := dotnet.ResolveBuild(appService(dir, "net8.0"), newest, complete)

	// Assert
	if !got.Stale || !got.NeedsBuild() || got.Library != lib {
		t.Errorf("ResolveBuild() = %+v, want a stale build at %s", got, lib)
	}
}

func TestResolveBuild_MostRecentConfigurationWins(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	builtApp(t, dir, "Debug", "net8.0", "App", t0)
	release := builtApp(t, dir, "Release", "net8.0", "App", t1)

	// Act
	got := dotnet.ResolveBuild(appService(dir, "net8.0"), time.Time{}, true)

	// Assert
	if got.Library != release || !got.Existing {
		t.Errorf("ResolveBuild() = %+v, want the Release library %s", got, release)
	}
}

func TestResolveBuild_LibraryWithoutRuntimeConfigIsNotBuilt(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "bin/Debug/net8.0/App.dll", "dll")

	// Act
	got := dotnet.ResolveBuild(appService(dir, "net8.0"), time.Time{}, true)

	// Assert
	if !got.NeedsBuild() || got.Stale {
		t.Errorf("ResolveBuild() = %+v, want a build without an existing library", got)
	}
}

func TestResolveBuild_MultiTargetUsesTheBuiltFramework(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	lib := builtApp(t, dir, "Debug", "net9.0", "App", t1)

	// Act
	got := dotnet.ResolveBuild(appService(dir, "net8.0", "net9.0"), time.Time{}, true)

	// Assert
	if got.Library != lib || !got.Existing {
		t.Errorf("ResolveBuild() = %+v, want the net9.0 library %s", got, lib)
	}
}

func TestResolveBuild_UnknownFramework(t *testing.T) {
	tests := []struct {
		name      string
		build     bool
		wantBuild bool
	}{
		{"library found in any framework directory", true, false},
		{"nothing built and no predictable path", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			var lib string
			if tt.build {
				lib = builtApp(t, dir, "Debug", "net11.0", "App", t1)
			}

			// Act
			got := dotnet.ResolveBuild(appService(dir), time.Time{}, true)

			// Assert
			if got.NeedsBuild() != tt.wantBuild || got.Library != lib {
				t.Errorf("ResolveBuild() = %+v, want NeedsBuild=%v Library=%q", got, tt.wantBuild, lib)
			}
		})
	}
}

func TestResolveBuild_UsesAssemblyNameWhenNameWasDisambiguated(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	lib := builtApp(t, dir, "Debug", "net8.0", "Api", t1)
	svc := dotnet.Service{Name: "src/a/Api", AssemblyName: "Api", Dir: dir, Frameworks: []string{"net8.0"}}

	// Act
	got := dotnet.ResolveBuild(svc, time.Time{}, true)

	// Assert
	if got.Library != lib {
		t.Errorf("Library = %q, want %q", got.Library, lib)
	}
}

func TestResolveBuild_IncompleteSourceCheckForcesBuild(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	builtApp(t, dir, "Debug", "net8.0", "App", t1)

	// Act
	got := dotnet.ResolveBuild(appService(dir, "net8.0"), time.Time{}, false)

	// Assert
	if !got.Stale || !got.NeedsBuild() {
		t.Errorf("ResolveBuild() = %+v, want a stale build when the source check is incomplete", got)
	}
}

func TestNewestSource_IgnoresBuildOutputAndOtherFiles(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "Program.cs", "")
	touch(t, dir, "Program.cs", t0)
	for _, ignored := range []string{"obj/Generated.cs", "bin/Debug/Old.cs", ".vs/cache.cs", "README.md"} {
		writeFile(t, dir, ignored, "")
		touch(t, dir, ignored, t2)
	}

	// Act
	newest, complete := dotnet.NewestSource(dir)

	// Assert
	if !complete || !newest.Equal(t0) {
		t.Errorf("NewestSource() = %v, %v; want %v, true", newest, complete, t0)
	}
}

func TestNewestSource_CountsCodeProjectAndAppsettings(t *testing.T) {
	for _, file := range []string{"Program.cs", "Pages/Index.razor", "App.csproj", "Directory.Build.props", "appsettings.Development.json"} {
		t.Run(file, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			writeFile(t, dir, file, "")
			touch(t, dir, file, t2)

			// Act
			newest, _ := dotnet.NewestSource(dir)

			// Assert
			if !newest.Equal(t2) {
				t.Errorf("NewestSource() = %v, want %v for %s", newest, t2, file)
			}
		})
	}
}

func TestNewestSourceWithin_StopsAtTheFileLimit(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	for i := 0; i < 6; i++ {
		writeFile(t, dir, fmt.Sprintf("files/%d.txt", i), "")
	}

	// Act
	_, complete := dotnet.NewestSourceWithin(dir, 5)

	// Assert
	if complete {
		t.Error("NewestSourceWithin() reported a complete walk over more files than its limit")
	}
}

func TestNewestSourceWithin_CompleteWithinTheLimit(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		writeFile(t, dir, fmt.Sprintf("files/%d.txt", i), "")
	}

	// Act
	_, complete := dotnet.NewestSourceWithin(dir, 5)

	// Assert
	if !complete {
		t.Error("NewestSourceWithin() reported an incomplete walk at exactly its limit")
	}
}
