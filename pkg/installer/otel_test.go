package installer

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
)

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

func TestDetectAvailableRuntimes_DefaultEnabled(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.AllRuntimes, false)

	runtimes := detectAvailableRuntimes()

	for _, rt := range runtimes {
		switch rt.name {
		case "Python":
			if !rt.enabled {
				t.Errorf("Python should be enabled by default, got enabled=false")
			}
		case "Java", "Node.js", "Go":
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
	featureflags.SetCLIOverrideForTest(t, featureflags.AllRuntimes, true)

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

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printProjectList(projects)

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
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
		"Skip",
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
	projects := detectAllProjects(runtimes)
	if len(projects) != 0 {
		t.Errorf("expected 0 projects when all runtimes are disabled, got %d: %v", len(projects), projects)
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
	projects := detectAllProjects(runtimes)
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
	apiURL := "https://tenant.live.dynatrace.com"
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
		}, apiURL, token, envURL, platformToken)

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
		}, apiURL, token, envURL, platformToken)

		if plan != nil {
			t.Fatalf("expected nil plan, got %T", plan)
		}
	})

	t.Run("java returns plan", func(t *testing.T) {
		projectDir := t.TempDir()

		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: projectDir, Markers: []string{"pom.xml"}},
			Runtime:        "Java",
		}, apiURL, token, envURL, platformToken)

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
		}, apiURL, token, envURL, platformToken)

		nodePlan, ok := plan.(*NodeInstrumentationPlan)
		if !ok {
			t.Fatalf("expected NodeInstrumentationPlan, got %T", plan)
		}
		if nodePlan.Entrypoint != "server.js" {
			t.Fatalf("unexpected node entrypoint: %s", nodePlan.Entrypoint)
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
		}, apiURL, token, envURL, platformToken)

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
		}, apiURL, token, envURL, platformToken)

		goPlan, ok := plan.(*GoInstrumentationPlan)
		if !ok {
			t.Fatalf("expected GoInstrumentationPlan, got %T", plan)
		}
		if goPlan.Project.ModuleName != "github.com/example/svc" {
			t.Fatalf("unexpected module name: %s", goPlan.Project.ModuleName)
		}
	})

	t.Run("unknown runtime returns nil", func(t *testing.T) {
		plan := createRuntimePlan(detectedProject{
			ScannedProject: ScannedProject{Path: t.TempDir()},
			Runtime:        "RubyTubey",
		}, apiURL, token, envURL, platformToken)

		if plan != nil {
			t.Fatalf("expected nil plan, got %T", plan)
		}
	})
}
