package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectAvailableRuntimes_DefaultEnabled(t *testing.T) {
	os.Unsetenv("DTWIZ_ALL_RUNTIMES")

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
	os.Setenv("DTWIZ_ALL_RUNTIMES", "true")
	defer os.Unsetenv("DTWIZ_ALL_RUNTIMES")

	runtimes := detectAvailableRuntimes()

	for _, rt := range runtimes {
		if !rt.enabled {
			t.Errorf("%s should be enabled when DTWIZ_ALL_RUNTIMES=true, got enabled=false", rt.name)
		}
	}
}

// TestDetectAvailableRuntimes_UnlockAll_1 verifies that DTWIZ_ALL_RUNTIMES=1 also works.
func TestDetectAvailableRuntimes_UnlockAll_1(t *testing.T) {
	os.Setenv("DTWIZ_ALL_RUNTIMES", "1")
	defer os.Unsetenv("DTWIZ_ALL_RUNTIMES")

	if !allRuntimesEnabled() {
		t.Error("allRuntimesEnabled() should return true when DTWIZ_ALL_RUNTIMES=1")
	}
}

// TestPrintProjectList_Formatting verifies the project list output format.
func TestPrintProjectList_Formatting(t *testing.T) {
	projects := []detectedProject{
		{ScannedProject: ScannedProject{Path: "/home/user/api", Markers: []string{"requirements.txt"}, RunningPIDs: []int{1234}}, Runtime: "Python"},
		{ScannedProject: ScannedProject{Path: "/home/user/svc", Markers: []string{"pom.xml"}}, Runtime: "Java"},
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
		"PIDs: 1234",
		"Java",
		"/home/user/svc",
		"pom.xml",
		"Skip",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("printProjectList output missing %q\nfull output:\n%s", c, output)
		}
	}
}

func TestDetectAllProjects_SkipsDisabled(t *testing.T) {
	runtimes := []runtimeInfo{
		{name: "Java", binName: "java", enabled: false},
		{name: "Node.js", binName: "node", enabled: false},
		{name: "Go", binName: "go", enabled: false},
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
	goMod := fmt.Sprintf("module github.com/test/app\n\ngo 1.21\n")
	if err := os.WriteFile(dir+"/go.mod", []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	runtimes := []runtimeInfo{
		{name: "Go", binName: "go", enabled: true},
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
