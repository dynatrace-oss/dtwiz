package otel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestIsIgnoredDir(t *testing.T) {
	ignored := []string{
		".git", ".venv", ".hidden",
		"node_modules", "__pycache__",
		"target", "vendor", "venv",
		"dist", "build", "out",
	}
	for _, name := range ignored {
		if !isIgnoredDir(name) {
			t.Errorf("isIgnoredDir(%q) = false, want true", name)
		}
	}
	notIgnored := []string{"src", "api", "mypackage", "services"}
	for _, name := range notIgnored {
		if isIgnoredDir(name) {
			t.Errorf("isIgnoredDir(%q) = true, want false", name)
		}
	}
}

func TestMatchingProcessIDs(t *testing.T) {
	procs := []DetectedProcess{
		{PID: 100, Command: "/usr/bin/python app.py", WorkingDirectory: "/home/user/projects/my-api"},
		{PID: 200, Command: "node /home/user/projects/my-api/server.js", WorkingDirectory: "/tmp"},
		{PID: 300, Command: "java -jar other.jar", WorkingDirectory: "/opt/other"},
	}

	pids := matchingProcessIDs("/home/user/projects/my-api", procs)
	sort.Ints(pids)
	if len(pids) != 2 || pids[0] != 100 || pids[1] != 200 {
		t.Errorf("matchingProcessIDs = %v, want [100, 200]", pids)
	}
}

func TestMatchingProcessIDs_CaseInsensitive(t *testing.T) {
	procs := []DetectedProcess{
		{PID: 42, Command: "python app.py", WorkingDirectory: "/Users/Bruno/Projects/MyApp"},
	}
	pids := matchingProcessIDs("/users/bruno/projects/myapp", procs)
	if len(pids) != 1 || pids[0] != 42 {
		t.Errorf("matchingProcessIDs (case-insensitive) = %v, want [42]", pids)
	}
}

func TestMatchingProcessIDs_NoMatch(t *testing.T) {
	procs := []DetectedProcess{
		{PID: 10, Command: "node index.js", WorkingDirectory: "/opt/other"},
	}
	pids := matchingProcessIDs("/home/user/myproject", procs)
	if len(pids) != 0 {
		t.Errorf("matchingProcessIDs = %v, want empty", pids)
	}
}

func TestMatchProcessesToProjects(t *testing.T) {
	projects := []ScannedProject{
		{Path: "/home/user/project-a"},
		{Path: "/home/user/project-b"},
	}
	procs := []DetectedProcess{
		{PID: 1, Command: "python app.py", WorkingDirectory: "/home/user/project-a"},
		{PID: 2, Command: "node server.js", WorkingDirectory: "/home/user/project-b"},
		{PID: 3, Command: "node /home/user/project-a/worker.js", WorkingDirectory: "/tmp"},
	}

	matchProcessesToProjects(projects, procs)

	sort.Ints(projects[0].RunningProcessIDs)
	if len(projects[0].RunningProcessIDs) != 2 || projects[0].RunningProcessIDs[0] != 1 || projects[0].RunningProcessIDs[1] != 3 {
		t.Errorf("project-a RunningProcessIDs = %v, want [1, 3]", projects[0].RunningProcessIDs)
	}
	if len(projects[1].RunningProcessIDs) != 1 || projects[1].RunningProcessIDs[0] != 2 {
		t.Errorf("project-b RunningProcessIDs = %v, want [2]", projects[1].RunningProcessIDs)
	}
}

func TestRunInParallel(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		left, right := runInParallel(
			func() string {
				started <- "left"
				<-release
				return "projects"
			},
			func() string {
				started <- "right"
				<-release
				return "processes"
			},
		)
		if left != "projects" || right != "processes" {
			t.Errorf("unexpected results: %q %q", left, right)
		}
	}()

	first := <-started
	if first == "" {
		t.Fatal("expected first task to start")
	}

	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected both tasks to start before either one finished")
	}

	close(release)
	<-done
}

func TestScanProjectDirs_CWD(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			if len(p.Markers) != 1 || p.Markers[0] != "go.mod" {
				t.Errorf("markers = %v, want [go.mod]", p.Markers)
			}
		}
	}
	if !found {
		t.Errorf("expected project at %s in results %v", dir, projects)
	}
}

func TestScanProjectDirs_SubDir(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	subDir := filepath.Join(dir, "myapp")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"package.json"}, nil, defaultScanRoots())
	realSubDir, _ := filepath.EvalSymlinks(subDir)
	found := false
	for _, p := range projects {
		if p.Path == subDir || p.Path == realSubDir || p.Path == filepath.Join(realDir, "myapp") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected project at %s in results %v", subDir, projects)
	}
}

func TestScanProjectDirs_ExcludeDirs(t *testing.T) {
	dir := t.TempDir()

	excludedDir := filepath.Join(dir, "node_modules")
	if err := os.Mkdir(excludedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(excludedDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"package.json"}, []string{"node_modules"}, defaultScanRoots())
	for _, p := range projects {
		if strings.Contains(p.Path, "node_modules") {
			t.Errorf("excluded dir appeared in results: %s", p.Path)
		}
	}
}

func TestScanProjectDirs_MultipleMarkers(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"pom.xml", "build.gradle"}, nil, defaultScanRoots())
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			if len(p.Markers) != 2 {
				t.Errorf("expected 2 markers, got %v", p.Markers)
			}
		}
	}
	if !found {
		t.Errorf("expected project at %s in results %v", dir, projects)
	}
}

func TestScanProjectDirs_NoMarkers(t *testing.T) {
	dir := t.TempDir()

	helpers.SetTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())
	realDir, _ := filepath.EvalSymlinks(dir)
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			t.Errorf("empty dir should not appear in results, got %v", projects)
		}
	}
}

func TestScanProjectDirs_NoiseDirs(t *testing.T) {
	dir := t.TempDir()

	noisy := filepath.Join(dir, "vendor")
	if err := os.Mkdir(noisy, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noisy, "go.mod"), []byte("module noise\n"), 0644); err != nil {
		t.Fatal(err)
	}

	legit := filepath.Join(dir, "myapp")
	if err := os.Mkdir(legit, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legit, "go.mod"), []byte("module myapp\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())

	for _, p := range projects {
		if strings.Contains(p.Path, "vendor") {
			t.Errorf("noise dir 'vendor' should be skipped, but found: %s", p.Path)
		}
	}
	found := false
	for _, p := range projects {
		if strings.HasSuffix(p.Path, "myapp") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected legitimate project myapp in results, got %v", projects)
	}
}

func TestScanProjectDirs_DotDirSkipped(t *testing.T) {
	dir := t.TempDir()

	hidden := filepath.Join(dir, ".hidden")
	if err := os.Mkdir(hidden, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "go.mod"), []byte("module hidden\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())
	for _, p := range projects {
		if strings.Contains(p.Path, ".hidden") {
			t.Errorf("dot-prefixed directory should be skipped, but found: %s", p.Path)
		}
	}
}

func TestScanProjectDirs_MonorepoGrouping(t *testing.T) {
	root := t.TempDir()

	group := filepath.Join(root, "group")
	if err := os.Mkdir(group, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"service-a", "service-b"} {
		sub := filepath.Join(group, name)
		if err := os.Mkdir(sub, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "go.mod"), []byte("module "+name+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	helpers.SetTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())

	paths := make(map[string]bool, len(projects))
	for _, p := range projects {
		paths[filepath.Base(p.Path)] = true
	}
	for _, want := range []string{"service-a", "service-b"} {
		if !paths[want] {
			t.Errorf("expected project %q to be found via monorepo grouping dir, got %v", want, projects)
		}
	}
}

func TestScanProjectDirs_DeepNesting(t *testing.T) {
	root := t.TempDir()

	// depth: root/a/b/c/d — four levels below cwd
	deep := filepath.Join(root, "a", "b", "c", "d")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "go.mod"), []byte("module deep\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())

	want := filepath.Join("a", "b", "c", "d")
	found := false
	for _, p := range projects {
		if strings.HasSuffix(filepath.ToSlash(p.Path), filepath.ToSlash(filepath.Join(root, want))) ||
			strings.HasSuffix(p.Path, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected project at depth 4 to be found, got %v", projects)
	}
}

func TestScanProjectDirs_SubtreePruning(t *testing.T) {
	root := t.TempDir()

	// parent project
	parent := filepath.Join(root, "myapp")
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte("module myapp\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// nested marker inside the same project — should not produce a second result
	nested := filepath.Join(parent, "internal", "sub")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module sub\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())

	count := 0
	for _, p := range projects {
		if strings.Contains(p.Path, "myapp") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 project under myapp (subtree pruned), got %d: %v", count, projects)
	}
}

// TestScanProjectDirs_RootMatchStillDescends guards against a regression where
// a marker file sitting directly in the scan root (e.g. a stray lockfile in
// the user's cwd) short-circuited the walk and hid every nested project.
func TestScanProjectDirs_RootMatchStillDescends(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module root\n"), 0644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(root, "nested-app")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module nested\n"), 0644); err != nil {
		t.Fatal(err)
	}

	realRoot, _ := filepath.EvalSymlinks(root)
	realNested, _ := filepath.EvalSymlinks(nested)

	helpers.SetTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())

	foundRoot, foundNested := false, false
	for _, p := range projects {
		if p.Path == realRoot {
			foundRoot = true
		}
		if p.Path == realNested {
			foundNested = true
		}
	}
	if !foundRoot {
		t.Errorf("expected root dir itself to be reported as a match, got %v", projects)
	}
	if !foundNested {
		t.Errorf("root match must not prevent descending into nested projects, got %v", projects)
	}
}

func TestScanProjectDirs_ParentNotScanned(t *testing.T) {
	grandparent := t.TempDir()

	sibling := filepath.Join(grandparent, "sibling")
	if err := os.Mkdir(sibling, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "go.mod"), []byte("module sibling\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cwd := filepath.Join(grandparent, "cwd")
	if err := os.Mkdir(cwd, 0755); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, cwd)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())

	if len(projects) != 0 {
		t.Errorf("expected no projects from cwd with no markers, got %v", projects)
	}
	for _, p := range projects {
		if strings.HasSuffix(p.Path, "sibling") {
			t.Errorf("parent directory must not be scanned; found sibling project outside cwd: %s", p.Path)
		}
	}
}

func TestIsIgnoredDir_WindowsSystemDirs(t *testing.T) {
	for _, name := range []string{"Windows", "System32", "SysWOW64", "WinSxS", "ProgramData"} {
		if !isIgnoredDir(name) {
			t.Errorf("isIgnoredDir(%q) = false, want true (Windows system dir)", name)
		}
	}
}

func TestIsIgnoredDir_CommonNamesNotIgnored(t *testing.T) {
	// dev, sys, proc are Linux virtual filesystem names at the root level, but
	// they are also common project subdirectory names. They must NOT be ignored.
	for _, name := range []string{"dev", "sys", "proc", "src", "lib", "cmd"} {
		if isIgnoredDir(name) {
			t.Errorf("isIgnoredDir(%q) = true, want false (valid project dir name)", name)
		}
	}
}

func TestScanProjectDirs_WindowsSystemDirSkipped(t *testing.T) {
	dir := t.TempDir()

	sysDir := filepath.Join(dir, "System32")
	if err := os.Mkdir(sysDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sysDir, "go.mod"), []byte("module sys\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	for _, p := range scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots()) {
		if strings.Contains(p.Path, "System32") {
			t.Errorf("Windows system dir 'System32' must be skipped, got %s", p.Path)
		}
	}
}

func TestScanProjectDirs_DevDirFound(t *testing.T) {
	dir := t.TempDir()

	devDir := filepath.Join(dir, "dev")
	if err := os.Mkdir(devDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "go.mod"), []byte("module dev\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	found := false
	for _, p := range scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots()) {
		if strings.HasSuffix(filepath.ToSlash(p.Path), "/dev") {
			found = true
		}
	}
	if !found {
		t.Errorf("project inside 'dev' directory must be found (not ignored)")
	}
}

func TestScanProjectDirs_WideParallelTree(t *testing.T) {
	root := t.TempDir()

	// Create 20 sibling project directories. The parallel scan must find all of
	// them (no goroutine races or lost updates).
	const siblings = 20
	names := make([]string, siblings)
	for i := range siblings {
		name := fmt.Sprintf("svc-%02d", i)
		names[i] = name
		sub := filepath.Join(root, name)
		if err := os.Mkdir(sub, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "go.mod"), []byte("module "+name+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	helpers.SetTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil, defaultScanRoots())

	found := make(map[string]bool, siblings)
	for _, p := range projects {
		found[filepath.Base(p.Path)] = true
	}
	for _, name := range names {
		if !found[name] {
			t.Errorf("parallel scan missed project %q; found: %v", name, projects)
		}
	}
}

// fakeHome redirects os.UserHomeDir() to a temp dir for the duration of the
// test by setting both $HOME (Unix) and $USERPROFILE (Windows).
func fakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

// TestScanProjectDirs_BundledExamples verifies that a Python project marker
// placed inside ~/.dtwiz/examples/schnitzel/ is found even when CWD is
// a completely different directory.
func TestScanProjectDirs_BundledExamples(t *testing.T) {
	home := fakeHome(t)
	bundled := filepath.Join(home, ".dtwiz", "examples", "schnitzel")
	if err := os.MkdirAll(bundled, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundled, "requirements.txt"), []byte("flask\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cwd := t.TempDir()
	helpers.SetTestWorkingDir(t, cwd)

	projects := scanProjectDirs([]string{"requirements.txt"}, nil, defaultScanRoots())

	found := false
	for _, p := range projects {
		if strings.HasSuffix(filepath.ToSlash(p.Path), "schnitzel") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected bundled schnitzel project to be found; got %v", projects)
	}
}

// TestScanProjectDirs_NoDuplicates ensures that a project appearing in both
// CWD and the bundled examples root is reported exactly once.
func TestScanProjectDirs_NoDuplicates(t *testing.T) {
	home := fakeHome(t)
	bundled := filepath.Join(home, ".dtwiz", "examples", "schnitzel")
	if err := os.MkdirAll(bundled, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundled, "requirements.txt"), []byte("flask\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// CWD == bundled path: the bundled scan is skipped, so the project is found exactly once via the CWD scan.
	helpers.SetTestWorkingDir(t, bundled)

	projects := scanProjectDirs([]string{"requirements.txt"}, nil, defaultScanRoots())

	count := 0
	for _, p := range projects {
		if strings.HasSuffix(filepath.ToSlash(p.Path), "schnitzel") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected schnitzel to appear exactly once; got %d occurrences in %v", count, projects)
	}
}

func TestScanProjectDirs_BundledExamplesFromHome(t *testing.T) {
	home := fakeHome(t)
	bundled := filepath.Join(home, ".dtwiz", "examples", "schnitzel")
	if err := os.MkdirAll(bundled, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundled, "requirements.txt"), []byte("flask\n"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, home)

	projects := scanProjectDirs([]string{"requirements.txt"}, nil, defaultScanRoots())

	count := 0
	for _, p := range projects {
		if strings.HasSuffix(filepath.ToSlash(p.Path), "schnitzel") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected schnitzel to appear exactly once when scanning from home; got %d occurrences in %v", count, projects)
	}
}

func TestParseWinProcessOutput_Empty(t *testing.T) {
	if got := parseWinProcessOutput(""); len(got) != 0 {
		t.Errorf("expected empty result for empty input, got %v", got)
	}
	if got := parseWinProcessOutput("   \r\n  \r\n"); len(got) != 0 {
		t.Errorf("expected empty result for whitespace-only input, got %v", got)
	}
}

func TestParseWinProcessOutput_StripsCRLF(t *testing.T) {
	// PowerShell on Windows uses \r\n line endings.
	raw := "1234|python.exe|C:\\Users\\user\r\n5678|flask|C:\\app\r\n"
	got := parseWinProcessOutput(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(got), got)
	}
	if got[0] != "1234|python.exe|C:\\Users\\user" {
		t.Errorf("line 0 = %q, want CR stripped", got[0])
	}
	if got[1] != "5678|flask|C:\\app" {
		t.Errorf("line 1 = %q, want CR stripped", got[1])
	}
}

func TestParseWinProcessOutput_SkipsBlankLines(t *testing.T) {
	raw := "line1\n\nline2\n\n\nline3\n"
	got := parseWinProcessOutput(raw)
	if len(got) != 3 {
		t.Fatalf("expected 3 non-blank lines, got %d: %v", len(got), got)
	}
}

func TestParseWinProcessOutput_SingleLine(t *testing.T) {
	raw := "42\r\n"
	got := parseWinProcessOutput(raw)
	if len(got) != 1 || got[0] != "42" {
		t.Errorf("got %v, want [\"42\"]", got)
	}
}

func TestPromptProjectSelection_SingleProjectRangeHint(t *testing.T) {
	projects := []ScannedProject{{Path: "/home/user/myapp", Markers: []string{"package.json"}}}
	setTestStdin(t, "\n") // skip selection
	output := helpers.CaptureStdout(t, func() {
		promptProjectSelection("Node.js", projects)
	})
	if !strings.Contains(output, "instrument [1] or press") {
		t.Errorf("expected range hint [1] in prompt text, got: %s", output)
	}
	if strings.Contains(output, "[1-1]") {
		t.Errorf("expected no [1-1] in prompt output for single project, got: %s", output)
	}
}

func TestParseWinProcessOutput_PipeDelimitedFields(t *testing.T) {
	// Verify pipe-delimited lines round-trip correctly through SplitN.
	raw := "100|C:\\Python312\\python.exe -m flask run|C:\\app\r\n"
	lines := parseWinProcessOutput(raw)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	parts := strings.SplitN(lines[0], "|", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 fields after SplitN, got %d: %v", len(parts), parts)
	}
	if parts[0] != "100" {
		t.Errorf("PID field = %q, want \"100\"", parts[0])
	}
	if parts[1] != "C:\\Python312\\python.exe -m flask run" {
		t.Errorf("CommandLine field = %q", parts[1])
	}
	if parts[2] != "C:\\app" {
		t.Errorf("WorkingDirectory field = %q", parts[2])
	}
}
