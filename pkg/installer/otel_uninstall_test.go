package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindNodeOtelDirs_Found(t *testing.T) {
	dir := t.TempDir()

	// Create a .otel/ directory with a package.json containing @opentelemetry deps.
	otelDir := filepath.Join(dir, ".otel")
	if err := os.MkdirAll(otelDir, 0755); err != nil {
		t.Fatal(err)
	}
	deps := map[string]interface{}{
		"name":    "otel-instrumentation",
		"private": true,
		"dependencies": map[string]string{
			"@opentelemetry/auto-instrumentations-node": "latest",
			"@opentelemetry/sdk-node":                   "latest",
		},
	}
	data, _ := json.MarshalIndent(deps, "", "  ")
	if err := os.WriteFile(filepath.Join(otelDir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	dirs := findNodeOtelDirs()

	// Resolve symlinks for comparison (macOS /tmp → /private/tmp).
	realOtelDir, _ := filepath.EvalSymlinks(otelDir)
	found := false
	for _, d := range dirs {
		realD, _ := filepath.EvalSymlinks(d)
		if realD == realOtelDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected .otel/ dir %s in results, got %v", otelDir, dirs)
	}
}

func TestFindNodeOtelDirs_ChildProjects(t *testing.T) {
	// The key scenario: CWD is a workspace root with Node.js projects in
	// subdirectories, each having their own .otel/ directory.
	dir := t.TempDir()

	var expectedDirs []string
	for _, project := range []string{"app-a", "app-b", "app-c"} {
		otelDir := filepath.Join(dir, project, ".otel")
		if err := os.MkdirAll(otelDir, 0755); err != nil {
			t.Fatal(err)
		}
		pkgJSON := `{"dependencies":{"@opentelemetry/auto-instrumentations-node":"latest"}}`
		if err := os.WriteFile(filepath.Join(otelDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
			t.Fatal(err)
		}
		realDir, _ := filepath.EvalSymlinks(otelDir)
		expectedDirs = append(expectedDirs, realDir)
	}

	setTestWorkingDir(t, dir)
	dirs := findNodeOtelDirs()

	// Resolve all found dirs for comparison.
	foundSet := make(map[string]bool)
	for _, d := range dirs {
		realD, _ := filepath.EvalSymlinks(d)
		foundSet[realD] = true
	}

	for _, want := range expectedDirs {
		if !foundSet[want] {
			t.Errorf("expected .otel/ dir %s in results, got %v", want, dirs)
		}
	}
}

func TestFindNodeOtelDirs_IgnoresNonOtelDirs(t *testing.T) {
	dir := t.TempDir()

	// Create a .otel/ directory with a package.json that does NOT contain @opentelemetry.
	otelDir := filepath.Join(dir, ".otel")
	if err := os.MkdirAll(otelDir, 0755); err != nil {
		t.Fatal(err)
	}
	pkgJSON := `{"name": "something-else", "dependencies": {"express": "4.0.0"}}`
	if err := os.WriteFile(filepath.Join(otelDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	dirs := findNodeOtelDirs()

	realOtelDir, _ := filepath.EvalSymlinks(otelDir)
	for _, d := range dirs {
		realD, _ := filepath.EvalSymlinks(d)
		if realD == realOtelDir {
			t.Errorf("expected .otel/ dir without @opentelemetry to be excluded, but it was included: %v", dirs)
		}
	}
}

func TestFindNodeOtelDirs_NoDirs(t *testing.T) {
	dir := t.TempDir()
	setTestWorkingDir(t, dir)

	dirs := findNodeOtelDirs()
	if len(dirs) != 0 {
		t.Errorf("expected empty result, got %v", dirs)
	}
}

func TestFindNodeOtelDirs_NoPackageJSON(t *testing.T) {
	dir := t.TempDir()

	// Create a .otel/ directory without package.json.
	otelDir := filepath.Join(dir, ".otel")
	if err := os.MkdirAll(otelDir, 0755); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	dirs := findNodeOtelDirs()

	realOtelDir, _ := filepath.EvalSymlinks(otelDir)
	for _, d := range dirs {
		realD, _ := filepath.EvalSymlinks(d)
		if realD == realOtelDir {
			t.Errorf("expected .otel/ dir without package.json to be excluded, got %v", dirs)
		}
	}
}

func TestIsNodeOtelDir_True(t *testing.T) {
	dir := t.TempDir()
	otelDir := filepath.Join(dir, ".otel")
	if err := os.MkdirAll(otelDir, 0755); err != nil {
		t.Fatal(err)
	}
	pkgJSON := `{"dependencies":{"@opentelemetry/auto-instrumentations-node":"latest"}}`
	if err := os.WriteFile(filepath.Join(otelDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	if !isNodeOtelDir(otelDir) {
		t.Error("expected isNodeOtelDir=true for dir with @opentelemetry dependency")
	}
}

func TestIsNodeOtelDir_False_NoOtelDeps(t *testing.T) {
	dir := t.TempDir()
	otelDir := filepath.Join(dir, ".otel")
	if err := os.MkdirAll(otelDir, 0755); err != nil {
		t.Fatal(err)
	}
	pkgJSON := `{"dependencies":{"express":"4.0.0"}}`
	if err := os.WriteFile(filepath.Join(otelDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	if isNodeOtelDir(otelDir) {
		t.Error("expected isNodeOtelDir=false for dir without @opentelemetry")
	}
}

func TestIsNodeOtelDir_False_NoPkgJSON(t *testing.T) {
	dir := t.TempDir()
	otelDir := filepath.Join(dir, ".otel")
	if err := os.MkdirAll(otelDir, 0755); err != nil {
		t.Fatal(err)
	}

	if isNodeOtelDir(otelDir) {
		t.Error("expected isNodeOtelDir=false for dir without package.json")
	}
}

func TestUninstallOtelCollector_IncludesNodeDirs(t *testing.T) {
	// This test verifies that the uninstall preview mentions Node.js
	// .otel/ directories when they exist. We use dry-run so nothing is deleted.
	// Note: colored output (header.Println, muted.Println, red.Printf) uses the
	// fatih/color package which writes to its own cached output, not the
	// pipe-swapped os.Stdout. We check for strings from plain fmt.Println calls.
	dir := t.TempDir()

	// Create a .otel/ directory with @opentelemetry deps.
	otelDir := filepath.Join(dir, ".otel")
	if err := os.MkdirAll(otelDir, 0755); err != nil {
		t.Fatal(err)
	}
	pkgJSON := `{"dependencies":{"@opentelemetry/auto-instrumentations-node":"latest"}}`
	if err := os.WriteFile(filepath.Join(otelDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)

	output := captureStdout(t, func() {
		_ = UninstallOtelCollector(true) // dry-run
	})

	// The ".otel/ directories that will be removed:" line is printed via fmt.Println
	// (not color), so it should appear in the captured output.
	if !strings.Contains(output, ".otel/ directories that will be removed") {
		t.Errorf("expected output to contain '.otel/ directories that will be removed', got:\n%s", output)
	}
	if !strings.Contains(output, ".otel") {
		t.Errorf("expected output to mention .otel/ directory, got:\n%s", output)
	}
}

// TestFindInstrumentedJavaProcesses_FiltersByAgentFlag verifies that only
// processes whose command contains the dtwiz agent JAR path are returned.
func TestFindInstrumentedJavaProcesses_FiltersByAgentFlag(t *testing.T) {
	agentPath, err := javaAgentPath()
	if err != nil {
		t.Skipf("cannot resolve java agent path: %v", err)
	}

	origDetect := detectJavaProcessesFunc
	origEnrich := enrichProcessesWithJPSFunc
	defer func() {
		detectJavaProcessesFunc = origDetect
		enrichProcessesWithJPSFunc = origEnrich
	}()

	detectJavaProcessesFunc = func() []DetectedProcess {
		return []DetectedProcess{
			{PID: 100, Command: "java -javaagent:" + agentPath + " -jar app.jar"},
			{PID: 200, Command: "java -jar other.jar"},
			{PID: 300, Command: "java -javaagent:/other/path/javaagent.jar -jar app.jar"},
		}
	}
	enrichProcessesWithJPSFunc = func(procs []DetectedProcess) []DetectedProcess { return procs }

	result := findInstrumentedJavaProcesses()

	if len(result) != 1 {
		t.Fatalf("expected 1 instrumented process, got %d", len(result))
	}
	if result[0].PID != 100 {
		t.Errorf("expected PID 100, got %d", result[0].PID)
	}
}

// TestFindInstrumentedJavaProcesses_NoneMatching verifies that an empty slice
// is returned when no processes contain the dtwiz agent flag.
func TestFindInstrumentedJavaProcesses_NoneMatching(t *testing.T) {
	origDetect := detectJavaProcessesFunc
	origEnrich := enrichProcessesWithJPSFunc
	defer func() {
		detectJavaProcessesFunc = origDetect
		enrichProcessesWithJPSFunc = origEnrich
	}()

	detectJavaProcessesFunc = func() []DetectedProcess {
		return []DetectedProcess{
			{PID: 200, Command: "java -jar other.jar"},
		}
	}
	enrichProcessesWithJPSFunc = func(procs []DetectedProcess) []DetectedProcess { return procs }

	result := findInstrumentedJavaProcesses()

	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

// TestJavaAgentDir_ReturnsParentOfJar verifies that javaAgentDir() returns
// the directory whose last path element is "java".
func TestJavaAgentDir_ReturnsParentOfJar(t *testing.T) {
	dir := javaAgentDir()
	if dir == "" {
		t.Fatal("javaAgentDir() returned empty string")
	}
	if filepath.Base(dir) != "java" {
		t.Errorf("expected last segment to be \"java\", got %q (full path: %s)", filepath.Base(dir), dir)
	}
}

// TestUninstallOtelCollector_JavaDryRun_NothingPresent verifies that when there
// are no Java processes and no agent dir, the Java section is absent from the
// dry-run preview.
func TestUninstallOtelCollector_JavaDryRun_NothingPresent(t *testing.T) {
	origDetect := detectJavaProcessesFunc
	origEnrich := enrichProcessesWithJPSFunc
	defer func() {
		detectJavaProcessesFunc = origDetect
		enrichProcessesWithJPSFunc = origEnrich
	}()

	detectJavaProcessesFunc = func() []DetectedProcess { return []DetectedProcess{} }
	enrichProcessesWithJPSFunc = func(procs []DetectedProcess) []DetectedProcess { return procs }

	// Point HOME to a temp dir that has no .opentelemetry/java directory.
	// Set both HOME and USERPROFILE to ensure os.UserHomeDir() uses the temp dir on all platforms.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	origCleaners := runtimeCleaners
	t.Cleanup(func() { runtimeCleaners = origCleaners })
	runtimeCleaners = []RuntimeCleaner{}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if strings.Contains(output, "Java") {
		t.Errorf("expected no Java section in output, but found one\nfull output:\n%s", output)
	}
}

// TestUninstallOtelCollector_JavaDryRun_AgentDirExists verifies that when the
// agent directory is present, the dry-run preview lists it but does not remove it.
func TestUninstallOtelCollector_JavaDryRun_AgentDirExists(t *testing.T) {
	origDetect := detectJavaProcessesFunc
	origEnrich := enrichProcessesWithJPSFunc
	defer func() {
		detectJavaProcessesFunc = origDetect
		enrichProcessesWithJPSFunc = origEnrich
	}()

	detectJavaProcessesFunc = func() []DetectedProcess { return []DetectedProcess{} }
	enrichProcessesWithJPSFunc = func(procs []DetectedProcess) []DetectedProcess { return procs }

	// Create the agent dir under a temp HOME.
	// Set both HOME and USERPROFILE to ensure os.UserHomeDir() uses the temp dir on all platforms.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	agentDir := filepath.Join(tmp, ".opentelemetry", "java")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	origCleaners := runtimeCleaners
	t.Cleanup(func() { runtimeCleaners = origCleaners })
	runtimeCleaners = []RuntimeCleaner{}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if !strings.Contains(output, "Java agent directory") {
		t.Errorf("expected agent dir section in output, but did not find it\nfull output:\n%s", output)
	}

	// The directory must NOT have been removed (dry-run).
	if _, err := os.Stat(agentDir); os.IsNotExist(err) {
		t.Error("agent dir was removed during dry-run — it should not have been")
	}
}
