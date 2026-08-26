package otel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// captureInstallOutput runs InstallOtelCollector with dryRun=true and
// AutoConfirm=true (to bypass the "Continue?" prompt when no projects are
// found) and returns everything written to stdout+color.Output.
// isElevated controls the stub returned by isElevatedFn for the duration.
func captureInstallOutput(t *testing.T, isElevated bool) string {
	t.Helper()

	// Redirect stdout + color.Output so we capture both fmt.* and display.Color* output.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdout := os.Stdout
	oldColorOut := color.Output
	os.Stdout = w
	color.Output = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		color.Output = oldColorOut
	})

	// Override elevation check.
	origFn := isElevatedFn
	isElevatedFn = func() bool { return isElevated }
	t.Cleanup(func() { isElevatedFn = origFn })

	// AutoConfirm skips the "Continue installation?" prompt that appears when
	// no projects are found in the temp dir.
	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	// CWD/HOME → isolated temp dir so no projects are detected and non-interactive
	// scan-root selection does not walk the real home directory.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// dryRun=true prevents any download or process execution.
	_ = InstallOtelCollector("https://env.example.com", "mytoken", "", true)

	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func setTestStdin(t *testing.T, input string) {
	t.Helper()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}

	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write stdin input: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})
}

func TestInferRuntimeFromPath(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		dirs  []string
		want  string
	}{
		{name: "python requirements.txt", files: []string{"requirements.txt"}, want: "Python"},
		{name: "python pyproject.toml", files: []string{"pyproject.toml"}, want: "Python"},
		{name: "python manage.py", files: []string{"manage.py"}, want: "Python"},
		{name: "java pom.xml", files: []string{"pom.xml"}, want: "Java"},
		{name: "java build.gradle", files: []string{"build.gradle"}, want: "Java"},
		{name: "java build.gradle.kts", files: []string{"build.gradle.kts"}, want: "Java"},
		{name: "java .mvn directory", dirs: []string{".mvn"}, want: "Java"},
		{name: "nodejs package.json", files: []string{"package.json"}, want: "Node.js"},
		{name: "nodejs yarn.lock", files: []string{"yarn.lock"}, want: "Node.js"},
		{name: "nodejs pnpm-lock.yaml", files: []string{"pnpm-lock.yaml"}, want: "Node.js"},
		{name: "nodejs bun.lockb", files: []string{"bun.lockb"}, want: "Node.js"},
		{name: "nodejs .nvmrc", files: []string{".nvmrc"}, want: "Node.js"},
		{name: "nodejs .node-version", files: []string{".node-version"}, want: "Node.js"},
		{name: "java gradlew", files: []string{"gradlew"}, want: "Java"},
		{name: "java gradlew.bat", files: []string{"gradlew.bat"}, want: "Java"},
		{name: "java mvnw.cmd", files: []string{"mvnw.cmd"}, want: "Java"},
		{name: "no markers", files: []string{"README.md"}, want: ""},
		{name: "empty directory", files: []string{}, want: ""},
		// Java wins over Node.js when both present (e.g. Spring with frontend tooling).
		{name: "java beats nodejs", files: []string{"pom.xml", "package.json"}, want: "Java"},
		// Java wins over Python.
		{name: "java beats python", files: []string{"build.gradle", "requirements.txt"}, want: "Java"},
		// Node.js wins over Python.
		{name: "nodejs beats python", files: []string{"package.json", "requirements.txt"}, want: "Node.js"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte{}, 0644); err != nil {
					t.Fatal(err)
				}
			}
			for _, d := range tt.dirs {
				if err := os.Mkdir(filepath.Join(dir, d), 0755); err != nil {
					t.Fatal(err)
				}
			}
			got := inferRuntimeFromPath(dir)
			if got != tt.want {
				t.Errorf("inferRuntimeFromPath(%v, dirs=%v) = %q, want %q", tt.files, tt.dirs, got, tt.want)
			}
		})
	}
}

func TestDetectAvailableRuntimes_DefaultEnabled(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.AllRuntimes)
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")

	runtimes := detectAvailableRuntimes()

	for _, rt := range runtimes {
		switch rt.name {
		case "Python", "Node.js", "Java":
			if !rt.enabled {
				t.Errorf("%s should be enabled by default, got enabled=false", rt.name)
			}
		case "Go":
			if rt.enabled {
				t.Errorf("%s should be disabled by default, got enabled=true", rt.name)
			}
		}
	}
}

func TestDetectAvailableRuntimes_UnlockAll(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.AllRuntimes, true)

	runtimes := detectAvailableRuntimes()

	for _, rt := range runtimes {
		if !rt.enabled {
			t.Errorf("%s should be enabled when DTWIZ_ALL_RUNTIMES=true, got enabled=false", rt.name)
		}
	}
}

func TestDetectAvailableRuntimes_UnlockAll_1(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "1")

	if !featureflags.IsEnabled(featureflags.AllRuntimes) {
		t.Error("featureflags.IsEnabled(AllRuntimes) should return true when set via SetCLIOverrideForTest")
	}
}

func TestDetectedProjectsFromScan(t *testing.T) {
	projects := []ScannedProject{{Path: "/tmp/api", Markers: []string{"requirements.txt"}}, {Path: "/tmp/worker", Markers: []string{"pyproject.toml"}}}

	detected := detectedProjectsFromScan("Python", projects)

	if len(detected) != 2 {
		t.Fatalf("expected 2 detected projects, got %d", len(detected))
	}
	if detected[0].Runtime != "Python" || detected[0].Path != "/tmp/api" {
		t.Fatalf("unexpected first detected project: %+v", detected[0])
	}
	if detected[1].Runtime != "Python" || detected[1].Path != "/tmp/worker" {
		t.Fatalf("unexpected second detected project: %+v", detected[1])
	}
}

func TestDetectMatchedProjects_AttachesProcessMatches(t *testing.T) {
	projectFn := func() []ScannedProject {
		return []ScannedProject{{Path: "/tmp/api", Markers: []string{"requirements.txt"}}, {Path: "/tmp/worker", Markers: []string{"requirements.txt"}}}
	}
	processFn := func() []DetectedProcess {
		return []DetectedProcess{
			{PID: 101, WorkingDirectory: "/tmp/api"},
			{PID: 202, Command: "python /tmp/worker/main.py"},
		}
	}

	detected := detectMatchedProjects("Python", projectFn, processFn)

	if len(detected) != 2 {
		t.Fatalf("expected 2 detected projects, got %d", len(detected))
	}
	if got := detected[0].RunningProcessIDs; len(got) != 1 || got[0] != 101 {
		t.Fatalf("unexpected running PIDs for first project: %v", got)
	}
	if got := detected[1].RunningProcessIDs; len(got) != 1 || got[0] != 202 {
		t.Fatalf("unexpected running PIDs for second project: %v", got)
	}
}

// TestPrintProjectList_Formatting verifies the project list output format.
func TestPrintProjectList_Formatting(t *testing.T) {
	projects := []detectedProject{
		{ScannedProject: ScannedProject{Path: "/home/user/api", Markers: []string{"requirements.txt"}, RunningProcessIDs: []int{-1}}, Runtime: "Python"},
		{ScannedProject: ScannedProject{Path: "/home/user/svc", Markers: []string{"pom.xml"}}, Runtime: "Java"},
		{ScannedProject: ScannedProject{Path: "/home/user/go-svc", Markers: []string{"go.mod"}}, Runtime: "Go", ModuleName: "github.com/example/go-svc"},
	}

	// Capture stdout (including color output).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	oldColorOut := color.Output
	color.Output = w
	t.Cleanup(func() { color.Output = oldColorOut })

	oldNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = oldNoColor })

	printProjectList(projects)

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	output := string(out)

	checks := []string{
		"Python",
		"/home/user/api",
		"requirements.txt",
		"processes", // new: count label
		"PIDs: -1",  // PID fallback uses an invalid PID to keep output deterministic
		"Java",
		"/home/user/svc",
		"pom.xml",
		"github.com/example/go-svc",
		"Skip — If skipped",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("printProjectList output missing %q\nfull output:\n%s", c, output)
		}
	}
}

// TestPrintProjectList_ProcessCountFormat verifies that a project with running
// processes shows the "N processes (PIDs: ...)" annotation in the list output.
// Fixed high-numbered PIDs 99991 and 99992 are used because
// detectProcessListeningPort is unlikely to return a port for them in the test
// environment, giving us the PID-fallback path.
func TestPrintProjectList_ProcessCountFormat(t *testing.T) {
	projects := []detectedProject{
		{
			ScannedProject: ScannedProject{
				Path:              "/home/user/api",
				Markers:           []string{"requirements.txt"},
				RunningProcessIDs: []int{99991, 99992},
			},
			Runtime: "Python",
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printProjectList(projects)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	output := string(out)

	// Must contain the count label.
	if !strings.Contains(output, "2 processes") {
		t.Errorf("expected \"2 processes\" in output, got:\n%s", output)
	}
	// When no port is found, PIDs must appear as fallback.
	if !strings.Contains(output, "PIDs:") {
		t.Errorf("expected \"PIDs:\" fallback in output, got:\n%s", output)
	}
}

// TestPrintProjectList_NoAnnotationWhenNoProcesses verifies that projects with no
// running processes do not show any process annotation.
func TestPrintProjectList_NoAnnotationWhenNoProcesses(t *testing.T) {
	projects := []detectedProject{
		{
			ScannedProject: ScannedProject{Path: "/home/user/api", Markers: []string{"requirements.txt"}},
			Runtime:        "Python",
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printProjectList(projects)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	output := string(out)

	if strings.Contains(output, "processes") {
		t.Errorf("expected no process annotation for project with no running PIDs, got:\n%s", output)
	}
	if strings.Contains(output, "PIDs:") {
		t.Errorf("expected no PIDs annotation for project with no running PIDs, got:\n%s", output)
	}
}

func TestSelectProject(t *testing.T) {
	projects := []detectedProject{
		{ScannedProject: ScannedProject{Path: "/tmp/api"}, Runtime: "Python"},
		{ScannedProject: ScannedProject{Path: "/tmp/worker"}, Runtime: "Go"},
	}

	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantIdx int
	}{
		{name: "empty input skips", input: "\n", wantOK: false, wantIdx: -1},
		{name: "non numeric skips", input: "abc\n", wantOK: false, wantIdx: -1},
		{name: "out of range skips", input: "9\n", wantOK: false, wantIdx: -1},
		{name: "explicit skip option", input: "3\n", wantOK: false, wantIdx: -1},
		{name: "valid selection", input: "2\n", wantOK: true, wantIdx: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTestStdin(t, tt.input)

			project, ok := selectProject(projects)

			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if tt.wantIdx >= 0 && project.Path != projects[tt.wantIdx].Path {
				t.Fatalf("expected selected path %s, got %s", projects[tt.wantIdx].Path, project.Path)
			}
		})
	}
}

func TestDetectAllProjects_SkipsDisabled(t *testing.T) {
	runtimes := []runtimeInfo{
		{name: "Java", binName: "java", enabled: false, detect: detectJavaRuntimeProjects},
		{name: "Node.js", binName: "node", enabled: false, detect: detectNodeRuntimeProjects},
		{name: "Go", binName: "go", enabled: false, detect: detectGoRuntimeProjects},
	}
	projects := detectAllProjects(runtimes, defaultScanRoots())
	if len(projects) != 0 {
		t.Errorf("expected 0 projects when all runtimes are disabled, got %d: %v", len(projects), projects)
	}
}

// TestDetectAllProjects_SkipsPythonStub verifies that a python3 binary on PATH
// that does not output "Python 3.x" (e.g. a Windows Store stub that exits
// silently) is treated as unavailable and excluded from the active runtimes.
func TestDetectAllProjects_SkipsPythonStub(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based stubs only work on Unix")
	}
	dir := t.TempDir()
	// A stub that exists on PATH but produces no output — simulates the Windows
	// Store App Execution Alias behaviour in a non-interactive subprocess.
	createStubFile(t, filepath.Join(dir, "python3"), "#!/bin/sh\nexit 0\n", 0o755)
	t.Setenv("PATH", dir)

	runtimes := []runtimeInfo{
		{name: "Python", binName: "python3", enabled: true, detect: detectPythonRuntimeProjects},
	}
	projects := detectAllProjects(runtimes, defaultScanRoots())
	if len(projects) != 0 {
		t.Errorf("expected Python to be skipped when stub produces no version output, got %d project(s)", len(projects))
	}
}

// TestDetectAllProjects_IncludesWhenUnlocked verifies that setting up a temp
// Go project and unlocking all runtimes includes it in the scan.
func TestDetectAllProjects_IncludesWhenUnlocked(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	goMod := "module github.com/test/app\n\ngo 1.21\n"
	if err := os.WriteFile(dir+"/go.mod", []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	runtimes := []runtimeInfo{
		{name: "Go", binName: "go", enabled: true, detect: detectGoRuntimeProjects},
	}
	projects := detectAllProjects(runtimes, defaultScanRoots())
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Go project at %s in unified list, got %v", dir, projects)
	}
}

func TestCreateRuntimePlan(t *testing.T) {
	const httpPort = 4318
	token := "test-token"
	envURL := "https://tenant.apps.dynatrace.com"
	platformToken := "platform-token"

	t.Run("python returns plan when entrypoint exists", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(projectDir, "main.py"), []byte("print('ok')\n"), 0644); err != nil {
			t.Fatal(err)
		}

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"main.py"}},
			Runtime:        "Python",
		}, httpPort, token, envURL, platformToken)

		pythonPlan, ok := plan.(*PythonInstrumentationPlan)
		if !ok {
			t.Fatalf("expected PythonInstrumentationPlan, got %T", plan)
		}
		if len(pythonPlan.Entrypoints) != 1 || pythonPlan.Entrypoints[0] != "main.py" {
			t.Fatalf("unexpected python entrypoints: %v", pythonPlan.Entrypoints)
		}
		if !pythonPlan.NeedsVenv {
			t.Fatal("expected NeedsVenv=true when no project pip is present")
		}
		if pythonPlan.EnvURL != envURL || pythonPlan.PlatformToken != platformToken {
			t.Fatalf("python plan lost environment values: %+v", pythonPlan)
		}
	})

	t.Run("python returns nil when no entrypoint exists", func(t *testing.T) {
		projectDir := t.TempDir()

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"requirements.txt"}},
			Runtime:        "Python",
		}, httpPort, token, envURL, platformToken)

		if plan != nil {
			t.Fatalf("expected nil plan, got %T", plan)
		}
	})

	t.Run("java returns plan", func(t *testing.T) {
		projectDir := t.TempDir()

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"pom.xml"}},
			Runtime:        "Java",
		}, httpPort, token, envURL, platformToken)

		javaPlan, ok := plan.(*JavaInstrumentationPlan)
		if !ok {
			t.Fatalf("expected JavaInstrumentationPlan, got %T", plan)
		}
		if javaPlan.Project.Path != projectDir {
			t.Fatalf("unexpected Java project path: %s", javaPlan.Project.Path)
		}
	})

	t.Run("node returns plan when entrypoint exists", func(t *testing.T) {
		projectDir := t.TempDir()
		pkgJSON := `{"main":"server.js"}`
		if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "server.js"), []byte("console.log('ok')\n"), 0644); err != nil {
			t.Fatal(err)
		}

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"package.json"}},
			Runtime:        "Node.js",
		}, httpPort, token, envURL, platformToken)

		nodePlan, ok := plan.(*NodeInstrumentationPlan)
		if !ok {
			t.Fatalf("expected NodeInstrumentationPlan, got %T", plan)
		}
		if len(nodePlan.Entrypoints) == 0 || nodePlan.Entrypoints[0] != "server.js" {
			t.Fatalf("unexpected node entrypoints: %v", nodePlan.Entrypoints)
		}
	})

	t.Run("node returns nil when no entrypoint exists", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(`{"name":"svc"}`), 0644); err != nil {
			t.Fatal(err)
		}

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"package.json"}},
			Runtime:        "Node.js",
		}, httpPort, token, envURL, platformToken)

		if plan != nil {
			t.Fatalf("expected nil plan, got %T", plan)
		}
	})

	t.Run("go returns plan with module name", func(t *testing.T) {
		projectDir := t.TempDir()

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"go.mod"}},
			Runtime:        "Go",
			ModuleName:     "github.com/example/svc",
		}, httpPort, token, envURL, platformToken)

		goPlan, ok := plan.(*GoInstrumentationPlan)
		if !ok {
			t.Fatalf("expected GoInstrumentationPlan, got %T", plan)
		}
		if goPlan.Project.ModuleName != "github.com/example/svc" {
			t.Fatalf("unexpected module name: %s", goPlan.Project.ModuleName)
		}
	})

	t.Run("go plan uses collector endpoint from httpPort", func(t *testing.T) {
		projectDir := t.TempDir()

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"go.mod"}},
			Runtime:        "Go",
		}, 4320, token, envURL, platformToken)

		goPlan, ok := plan.(*GoInstrumentationPlan)
		if !ok {
			t.Fatalf("expected GoInstrumentationPlan, got %T", plan)
		}
		if got := goPlan.EnvVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != "http://127.0.0.1:4320" {
			t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want http://127.0.0.1:4320", got)
		}
	})

	t.Run("unknown runtime returns nil", func(t *testing.T) {
		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: t.TempDir()},
			Runtime:        "RubyTubey",
		}, httpPort, token, envURL, platformToken)

		if plan != nil {
			t.Fatalf("expected nil plan, got %T", plan)
		}
	})
}

// ── Host-monitoring install-flow messaging tests ──────────────────────────────

// TestInstallOtelCollector_Experimental_ShowsHostMonitoringHeader verifies that
// enabling the experimental flag causes the combined header ("service and host
// monitoring") to be printed, not the standard info box.
func TestInstallOtelCollector_Experimental_ShowsHostMonitoringHeader(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	output := captureInstallOutput(t, true /* elevated */)

	// The experimental path prints this exact sentence; the standard path never does.
	const experimentalHeader = "service and host monitoring"
	if !strings.Contains(output, experimentalHeader) {
		t.Errorf("expected %q in output when --experimental is set:\n%s", experimentalHeader, output)
	}
}

// TestInstallOtelCollector_Standard_ShowsInfoBox verifies that without the
// experimental flag the static info-box is shown and the "service and host
// monitoring" combined header (experimental-only) is absent.
func TestInstallOtelCollector_Standard_ShowsInfoBox(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	output := captureInstallOutput(t, true /* elevated */)

	// The experimental combined header must NOT appear.
	const experimentalHeader = "service and host monitoring"
	if strings.Contains(output, experimentalHeader) {
		t.Errorf("unexpected %q in output when --experimental is not set:\n%s", experimentalHeader, output)
	}
	// The info box always contains "service monitoring".
	if !strings.Contains(output, "service monitoring") {
		t.Errorf("expected 'service monitoring' info-box in output when --experimental is not set:\n%s", output)
	}
}

// TestInstallOtelCollector_Experimental_Linux_ElevationNotice_NotElevated verifies
// that on Linux, when the process is not elevated, an advisory about root /
// systemd-journal is printed.
func TestInstallOtelCollector_Experimental_Linux_ElevationNotice_NotElevated(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	output := captureInstallOutput(t, false /* not elevated */)

	if !strings.Contains(output, "systemd-journal") {
		t.Errorf("expected Linux elevation notice (systemd-journal) when not elevated:\n%s", output)
	}
}

// TestInstallOtelCollector_Experimental_Linux_NoNoticeWhenElevated verifies that
// on Linux, when the process is already elevated, no advisory is printed.
func TestInstallOtelCollector_Experimental_Linux_NoNoticeWhenElevated(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	output := captureInstallOutput(t, true /* elevated */)

	if strings.Contains(output, "systemd-journal") {
		t.Errorf("unexpected Linux elevation notice when process is already elevated:\n%s", output)
	}
}

// TestInstallOtelCollector_Experimental_Windows_ElevationNotice_NotElevated verifies
// that on Windows, when the process is not elevated, an advisory about Administrator
// privilege is printed.
func TestInstallOtelCollector_Experimental_Windows_ElevationNotice_NotElevated(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	output := captureInstallOutput(t, false /* not elevated */)

	if !strings.Contains(output, "Administrator") {
		t.Errorf("expected Windows elevation notice (Administrator) when not elevated:\n%s", output)
	}
}

// ── Extension activation integration tests ────────────────────────────────────

// stubActivation replaces activateHostMonitoringExtensionFn for the duration of
// the test and records whether it was called.
func stubActivation(t *testing.T) *bool {
	t.Helper()
	called := false
	orig := activateHostMonitoringExtensionFn
	activateHostMonitoringExtensionFn = func(_, _ string) { called = true }
	t.Cleanup(func() { activateHostMonitoringExtensionFn = orig })
	return &called
}

// stubExtensionPreview replaces buildExtensionActivationPreviewFn so install
// preview tests don't make a real network call against the test envURL.
func stubExtensionPreview(t *testing.T) {
	t.Helper()
	orig := buildExtensionActivationPreviewFn
	buildExtensionActivationPreviewFn = func(_, _ string) (installer.ExtensionStatus, error) {
		return installer.ExtensionInstalledActive, nil
	}
	t.Cleanup(func() { buildExtensionActivationPreviewFn = orig })
}

func TestPrintExtensionActivationPreview(t *testing.T) {
	tests := []struct {
		name   string
		status installer.ExtensionStatus
		want   string
	}{
		{name: "not installed", status: installer.ExtensionNotInstalled, want: "will be installed and activated"},
		{name: "installed inactive", status: installer.ExtensionInstalledInactive, want: "already installed"},
		{name: "installed active", status: installer.ExtensionInstalledActive, want: "already installed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureActivationOutput(t, func() { printExtensionActivationPreview(tt.status) })
			if !strings.Contains(out, "OpenTelemetry Host Monitoring extension") || !strings.Contains(out, tt.want) {
				t.Fatalf("printExtensionActivationPreview() output = %q, want %q", out, tt.want)
			}
		})
	}
}

// runInstallWithAutoConfirm calls InstallOtelCollectorWithProject with
// dryRun=false and AutoConfirm=true. DQL polling is stubbed out so the
// test completes immediately even when a collector binary is already installed.
func runInstallWithAutoConfirm(t *testing.T) error {
	t.Helper()
	stubExtensionPreview(t)

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("blocked test HTTP request to %s", req.URL.Host)
	})
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	// Stub out DQL polling to avoid a 2-minute wait when a collector binary
	// is already present on the machine.
	origWait := waitForLogInDynatraceFn
	waitForLogInDynatraceFn = func(_, _, _ string, _ time.Duration) error { return nil }
	t.Cleanup(func() { waitForLogInDynatraceFn = origWait })

	// Suppress output.
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}

	origStdout := os.Stdout
	origColorOut := color.Output
	os.Stdout = devNull
	color.Output = devNull

	t.Cleanup(func() {
		os.Stdout = origStdout
		color.Output = origColorOut
		_ = devNull.Close()
	})

	// CWD/HOME → isolated temp dir so no projects are detected and non-interactive
	// scan-root selection does not walk the real home directory.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	return InstallOtelCollectorWithProject("https://env.example.com", "tok", "", "", false)
}

// TestInstallOtelCollector_Experimental_CallsActivation verifies that enabling
// the experimental flag causes the extension activation helper to be invoked.
func TestInstallOtelCollector_Experimental_CallsActivation(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
	called := stubActivation(t)

	_ = runInstallWithAutoConfirm(t)

	if !*called {
		t.Error("expected activateHostMonitoringExtensionFn to be called when experimental is enabled")
	}
}

// TestInstallOtelCollector_NoExperimental_SkipsActivation verifies that without
// the experimental flag the extension activation helper is never invoked.
func TestInstallOtelCollector_NoExperimental_SkipsActivation(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")
	called := stubActivation(t)

	_ = runInstallWithAutoConfirm(t)

	if *called {
		t.Error("expected activateHostMonitoringExtensionFn NOT to be called when experimental is disabled")
	}
}

// TestInstallOtelCollector_DryRun_SkipsActivation verifies that --dry-run
// prevents the extension activation helper from being invoked.
func TestInstallOtelCollector_DryRun_SkipsActivation(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
	called := stubActivation(t)
	stubExtensionPreview(t)

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	_ = InstallOtelCollector("https://env.example.com", "tok", "", true /* dryRun */)

	if *called {
		t.Error("expected activateHostMonitoringExtensionFn NOT to be called on dry run")
	}
}

// ── activateHostMonitoringExtension unit tests ────────────────────────────────

type fakeExtensionManager struct {
	ensureFresh   bool
	ensureErr     error
	version       string
	versionErr    error
	activateErr   error
	deactivateErr error
	deleteErr     error
	status        installer.ExtensionStatus
	statusErr     error
	calls         []string
}

func (f *fakeExtensionManager) EnsureInstalled(extensionName string) (bool, error) {
	f.calls = append(f.calls, "EnsureInstalled:"+extensionName)
	return f.ensureFresh, f.ensureErr
}

func (f *fakeExtensionManager) LatestExtensionVersion(extensionName string) (string, error) {
	f.calls = append(f.calls, "LatestExtensionVersion:"+extensionName)
	return f.version, f.versionErr
}

func (f *fakeExtensionManager) ActivateExtension(extensionName, version string) error {
	f.calls = append(f.calls, "ActivateExtension:"+extensionName+":"+version)
	return f.activateErr
}

func (f *fakeExtensionManager) DeactivateExtension(extensionName string) error {
	f.calls = append(f.calls, "DeactivateExtension:"+extensionName)
	return f.deactivateErr
}

func (f *fakeExtensionManager) DeleteExtensionVersion(extensionName, version string) error {
	f.calls = append(f.calls, "DeleteExtensionVersion:"+extensionName+":"+version)
	return f.deleteErr
}

func (f *fakeExtensionManager) GetStatus(extensionName string) (installer.ExtensionStatus, error) {
	f.calls = append(f.calls, "GetStatus:"+extensionName)
	return f.status, f.statusErr
}

func stubExtensionManager(t *testing.T, manager extensionManager, err error) {
	t.Helper()
	orig := newExtensionManagerFn
	newExtensionManagerFn = func(_, _ string) (extensionManager, error) {
		return manager, err
	}
	t.Cleanup(func() { newExtensionManagerFn = orig })
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("calls = %v, want %v", got, want)
		}
	}
}

func captureActivationOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	oldStdout := os.Stdout
	oldColorOut := color.Output
	os.Stdout = w
	color.Output = w
	defer func() {
		os.Stdout = oldStdout
		color.Output = oldColorOut
		if w != nil {
			_ = w.Close()
		}
		if r != nil {
			_ = r.Close()
		}
	}()

	fn()

	_ = w.Close()
	w = nil
	out, _ := io.ReadAll(r)
	_ = r.Close()
	r = nil

	return string(out)
}

func TestActivateHostMonitoringExtension_AlreadyInstalled_Activates(t *testing.T) {
	fake := &fakeExtensionManager{version: "3.1.1"}
	stubExtensionManager(t, fake, nil)

	out := captureActivationOutput(t, func() {
		activateHostMonitoringExtension("https://env.example.com", "dt0s16.test")
	})

	if !strings.Contains(out, "✓ OTel Host Monitoring extension active") {
		t.Errorf("expected success message in output, got: %s", out)
	}
	if strings.Contains(out, "Warning:") {
		t.Errorf("unexpected warning in output: %s", out)
	}
	assertStringSlicesEqual(t, fake.calls, []string{
		"EnsureInstalled:" + otelHostMonitoringExtension,
		"LatestExtensionVersion:" + otelHostMonitoringExtension,
		"ActivateExtension:" + otelHostMonitoringExtension + ":3.1.1",
	})
}

func TestActivateHostMonitoringExtension_FreshInstall_InstallsThenActivates(t *testing.T) {
	fake := &fakeExtensionManager{ensureFresh: true, version: "3.1.1"}
	stubExtensionManager(t, fake, nil)

	out := captureActivationOutput(t, func() {
		activateHostMonitoringExtension("https://env.example.com", "dt0s16.test")
	})

	if !strings.Contains(out, "✓ OTel Host Monitoring extension active") {
		t.Errorf("expected success message in output, got: %s", out)
	}
	assertStringSlicesEqual(t, fake.calls, []string{
		"EnsureInstalled:" + otelHostMonitoringExtension,
		"LatestExtensionVersion:" + otelHostMonitoringExtension,
		"ActivateExtension:" + otelHostMonitoringExtension + ":3.1.1",
	})
}

func TestActivateHostMonitoringExtension_ActivationFails_WarnsAndContinues(t *testing.T) {
	fake := &fakeExtensionManager{version: "3.1.1", activateErr: errors.New("activate failed")}
	stubExtensionManager(t, fake, nil)

	out := captureActivationOutput(t, func() {
		activateHostMonitoringExtension("https://env.example.com", "dt0s16.test")
	})

	if !strings.Contains(out, "Warning: could not activate OTel Host Monitoring extension; host entity creation may not be available.") {
		t.Errorf("expected activation failure warning in output, got: %s", out)
	}
}

// ── deactivateHostMonitoringExtension unit tests ─────────────────────────────

// stubGrailRouteRemoval replaces removeHostMonitoringGrailRoutesFn for the
// duration of the test with a no-op so deactivation tests don't hit the
// Grail API.
func stubGrailRouteRemoval(t *testing.T) {
	t.Helper()
	orig := removeHostMonitoringGrailRoutesFn
	removeHostMonitoringGrailRoutesFn = func(_, _ string) {}
	t.Cleanup(func() { removeHostMonitoringGrailRoutesFn = orig })
}

// stubDeactivation replaces deactivateHostMonitoringExtensionFn for the duration
// of the test and records whether it was called.
func stubDeactivation(t *testing.T) *bool {
	t.Helper()
	called := false
	orig := deactivateHostMonitoringExtensionFn
	deactivateHostMonitoringExtensionFn = func(_, _ string) { called = true }
	t.Cleanup(func() { deactivateHostMonitoringExtensionFn = orig })
	return &called
}

func TestDeactivateHostMonitoringExtension_HappyPath(t *testing.T) {
	stubGrailRouteRemoval(t)
	fake := &fakeExtensionManager{version: "3.1.1"}
	stubExtensionManager(t, fake, nil)

	out := captureActivationOutput(t, func() {
		deactivateHostMonitoringExtension("https://env.example.com", "dt0s16.test")
	})

	if !strings.Contains(out, "✓ OTel Host Monitoring extension removed") {
		t.Errorf("expected success message in output, got: %s", out)
	}
	if strings.Contains(out, "Warning:") {
		t.Errorf("unexpected warning in output: %s", out)
	}
	assertStringSlicesEqual(t, fake.calls, []string{
		"DeactivateExtension:" + otelHostMonitoringExtension,
		"LatestExtensionVersion:" + otelHostMonitoringExtension,
		"DeleteExtensionVersion:" + otelHostMonitoringExtension + ":3.1.1",
	})
}

func TestDeactivateHostMonitoringExtension_VersionNotFound_Warns(t *testing.T) {
	stubGrailRouteRemoval(t)
	fake := &fakeExtensionManager{deactivateErr: errors.New("deactivate failed")}
	stubExtensionManager(t, fake, nil)

	out := captureActivationOutput(t, func() {
		deactivateHostMonitoringExtension("https://env.example.com", "dt0s16.test")
	})

	if !strings.Contains(out, "Warning: could not deactivate OTel Host Monitoring extension; extension was not removed.") {
		t.Errorf("expected deactivation failure warning in output, got: %s", out)
	}
}

func TestDeactivateHostMonitoringExtension_DeleteFails_Warns(t *testing.T) {
	stubGrailRouteRemoval(t)
	fake := &fakeExtensionManager{version: "3.1.1", deleteErr: errors.New("delete failed")}
	stubExtensionManager(t, fake, nil)

	out := captureActivationOutput(t, func() {
		deactivateHostMonitoringExtension("https://env.example.com", "dt0s16.test")
	})

	if !strings.Contains(out, "Warning: could not remove OTel Host Monitoring extension; please remove it manually.") {
		t.Errorf("expected delete failure warning in output, got: %s", out)
	}
}

func TestDeactivateHostMonitoringExtension_CallsGrailRouteRemoval(t *testing.T) {
	grailCalled := false
	orig := removeHostMonitoringGrailRoutesFn
	removeHostMonitoringGrailRoutesFn = func(_, _ string) { grailCalled = true }
	t.Cleanup(func() { removeHostMonitoringGrailRoutesFn = orig })

	fake := &fakeExtensionManager{version: "3.1.1"}
	stubExtensionManager(t, fake, nil)

	captureActivationOutput(t, func() {
		deactivateHostMonitoringExtension("https://env.example.com", "dt0s16.test")
	})

	if !grailCalled {
		t.Error("expected removeHostMonitoringGrailRoutesFn to be called from deactivateHostMonitoringExtension")
	}
}

func TestRemoveHostMonitoringGrailRoutes_PrintsSuccessPerSignal(t *testing.T) {
	allRemoved := make([]bool, len(grailSignals))
	for i := range allRemoved {
		allRemoved[i] = true
	}
	orig := removeGrailRoutesFn
	removeGrailRoutesFn = func(_ context.Context, _ grailRouteClient) ([]bool, []error) {
		return allRemoved, make([]error, len(grailSignals))
	}
	t.Cleanup(func() { removeGrailRoutesFn = orig })

	out := captureActivationOutput(t, func() {
		removeHostMonitoringGrailRoutes("http://fake", "dt0s16.test")
	})

	for _, sig := range grailSignals {
		want := "✓ OpenPipeline " + sig.displayName + " route removed"
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}

func TestRemoveHostMonitoringGrailRoutes_NoOutputWhenSkipped(t *testing.T) {
	orig := removeGrailRoutesFn
	removeGrailRoutesFn = func(_ context.Context, _ grailRouteClient) ([]bool, []error) {
		// All skipped: removed=false, err=nil.
		return make([]bool, len(grailSignals)), make([]error, len(grailSignals))
	}
	t.Cleanup(func() { removeGrailRoutesFn = orig })

	out := captureActivationOutput(t, func() {
		removeHostMonitoringGrailRoutes("http://fake", "dt0s16.test")
	})

	if strings.Contains(out, "route removed") {
		t.Errorf("expected no route removed output when skipped, got: %s", out)
	}
	if strings.Contains(out, "Warning") {
		t.Errorf("expected no warning when skipped, got: %s", out)
	}
}

// stubPromptDecision replaces promptUninstallDecisionFn for the duration of the
// test with a function that returns the given decision without reading stdin.
func stubPromptDecision(t *testing.T, decision uninstallDecision) {
	t.Helper()
	orig := promptUninstallDecisionFn
	promptUninstallDecisionFn = func() (uninstallDecision, error) { return decision, nil }
	t.Cleanup(func() { promptUninstallDecisionFn = orig })
}

// runUninstallDryRun calls UninstallOtelCollector in dry-run mode with AutoConfirm.
func runUninstallDryRun(t *testing.T) {
	t.Helper()
	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	origStdout := os.Stdout
	origColorOut := color.Output
	os.Stdout = devNull
	color.Output = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		color.Output = origColorOut
		_ = devNull.Close()
	})

	_ = UninstallOtelCollector("https://env.example.com", "dt0s16.test", true)
}

// runUninstallWithConfirm calls UninstallOtelCollector with dryRun=false and AutoConfirm=true.
func runUninstallWithConfirm(t *testing.T) {
	t.Helper()
	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	origStdout := os.Stdout
	origColorOut := color.Output
	os.Stdout = devNull
	color.Output = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		color.Output = origColorOut
		_ = devNull.Close()
	})

	_ = UninstallOtelCollector("https://env.example.com", "dt0s16.test", false)
}

func TestUninstallOtelCollector_Experimental_DeleteAll_CallsDeactivation(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
	stubPromptDecision(t, uninstallAll)
	called := stubDeactivation(t)

	runUninstallWithConfirm(t)

	if !*called {
		t.Error("expected deactivateHostMonitoringExtensionFn to be called when user selects Delete all")
	}
}

func TestUninstallOtelCollector_Experimental_CollectorOnly_SkipsDeactivation(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
	stubPromptDecision(t, uninstallCollectorOnly)
	called := stubDeactivation(t)

	runUninstallWithConfirm(t)

	if *called {
		t.Error("expected deactivateHostMonitoringExtensionFn NOT to be called when user selects Only collector")
	}
}

func TestUninstallOtelCollector_NoExperimental_SkipsDeactivation(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")
	called := stubDeactivation(t)

	runUninstallWithConfirm(t)

	if *called {
		t.Error("expected deactivateHostMonitoringExtensionFn NOT to be called when experimental is disabled")
	}
}

func TestUninstallOtelCollector_DryRun_SkipsDeactivation(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
	called := stubDeactivation(t)

	runUninstallDryRun(t)

	if *called {
		t.Error("expected deactivateHostMonitoringExtensionFn NOT to be called on dry run")
	}
}

// ── buildExtensionActivationPreview unit tests ────────────────────────────────

func TestBuildExtensionActivationPreview_NotInstalled(t *testing.T) {
	fake := &fakeExtensionManager{status: installer.ExtensionNotInstalled}
	stubExtensionManager(t, fake, nil)

	status, err := buildExtensionActivationPreview("https://env.example.com", "dt0s16.test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != installer.ExtensionNotInstalled {
		t.Errorf("expected ExtensionNotInstalled, got %v", status)
	}
}

func TestBuildExtensionActivationPreview_InstalledNotActive(t *testing.T) {
	fake := &fakeExtensionManager{status: installer.ExtensionInstalledInactive}
	stubExtensionManager(t, fake, nil)

	status, err := buildExtensionActivationPreview("https://env.example.com", "dt0s16.test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != installer.ExtensionInstalledInactive {
		t.Errorf("expected ExtensionInstalledInactive, got %v", status)
	}
}

func TestBuildExtensionActivationPreview_InstalledAndActive(t *testing.T) {
	fake := &fakeExtensionManager{status: installer.ExtensionInstalledActive}
	stubExtensionManager(t, fake, nil)

	status, err := buildExtensionActivationPreview("https://env.example.com", "dt0s16.test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != installer.ExtensionInstalledActive {
		t.Errorf("expected ExtensionInstalledActive, got %v", status)
	}
}

func TestBuildExtensionActivationPreview_APIError(t *testing.T) {
	fake := &fakeExtensionManager{statusErr: errors.New("status failed")}
	stubExtensionManager(t, fake, nil)

	if _, err := buildExtensionActivationPreview("https://env.example.com", "dt0s16.test"); err == nil {
		t.Error("expected error from a failing extensions API")
	}
}

func TestGrailPreviewAndApplyMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action      grailAction
		wantPreview string
		wantApply   string
	}{
		{action: grailActionCreate, wantPreview: "create route", wantApply: "route created"},
		{action: grailActionReEnable, wantPreview: "re-enable route", wantApply: "route re-enabled"},
		{action: grailActionNoop, wantPreview: "already configured", wantApply: "already configured"},
		{action: grailActionSkip, wantPreview: "pending", wantApply: "skip"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.wantPreview, func(t *testing.T) {
			t.Parallel()

			preview, _ := grailPreviewMessage(tt.action)
			if !strings.Contains(preview, tt.wantPreview) {
				t.Fatalf("grailPreviewMessage() = %q, want %q", preview, tt.wantPreview)
			}
			apply, _ := grailApplyMessage(tt.action)
			if !strings.Contains(apply, tt.wantApply) {
				t.Fatalf("grailApplyMessage() = %q, want %q", apply, tt.wantApply)
			}
		})
	}
}

func TestPrintGrailPlanAndApplyResults(t *testing.T) {
	plans := []grailSignalPlan{
		{signal: grailSignals[0], action: grailActionCreate},
		{signal: grailSignals[1], action: grailActionNoop},
		{signal: grailSignals[2], action: grailActionSkip},
	}

	previewOut := captureActivationOutput(t, func() { printGrailPlan(plans) })
	for _, want := range []string{"OpenPipeline dynamic routes", "Metrics", "create route", "Logs", "already configured", "Spans", "pending"} {
		if !strings.Contains(previewOut, want) {
			t.Fatalf("printGrailPlan() output missing %q:\n%s", want, previewOut)
		}
	}

	applyOut := captureActivationOutput(t, func() { printGrailApplyResults(plans, []error{nil, fmt.Errorf("boom"), nil}) })
	for _, want := range []string{"OpenPipeline dynamic routes", "Metrics", "route created", "Logs", "warning", "boom", "Spans", "skip"} {
		if !strings.Contains(applyOut, want) {
			t.Fatalf("printGrailApplyResults() output missing %q:\n%s", want, applyOut)
		}
	}
}

// TestInstallOtelCollector_Experimental_Darwin_AlwaysShowsUnavailableNotice verifies
// that on macOS the advisory about system.processes.created / process.disk.io being
// unavailable is printed regardless of privilege level.
func TestInstallOtelCollector_Experimental_Darwin_AlwaysShowsUnavailableNotice(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin-only test")
	}
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	for _, tt := range []struct {
		name       string
		isElevated bool
	}{
		{name: "elevated", isElevated: true},
		{name: "not elevated", isElevated: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output := captureInstallOutput(t, tt.isElevated)
			if !strings.Contains(output, "macOS") {
				t.Errorf("expected macOS unavailability notice in output:\n%s", output)
			}
		})
	}
}
