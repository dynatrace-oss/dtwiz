package installer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
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

	setTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil)
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

	setTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"package.json"}, nil)
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

	setTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"package.json"}, []string{"node_modules"})
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

	setTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"pom.xml", "build.gradle"}, nil)
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

	setTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil)
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

	setTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil)

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

	setTestWorkingDir(t, dir)
	projects := scanProjectDirs([]string{"go.mod"}, nil)
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

	setTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil)

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

	setTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil)

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

	setTestWorkingDir(t, root)
	projects := scanProjectDirs([]string{"go.mod"}, nil)

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

func TestScanProjectDirs_AncestorWalk(t *testing.T) {
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

	setTestWorkingDir(t, cwd)
	projects := scanProjectDirs([]string{"go.mod"}, nil)

	found := false
	for _, p := range projects {
		if strings.HasSuffix(p.Path, "sibling") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sibling project to be found via ancestor walk, got %v", projects)
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
