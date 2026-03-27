package installer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestServiceNameFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/projects/my-api", "my-api"},
		{"/opt/services/backend", "backend"},
		{"", "my-service"},
		{".", "my-service"},
		{"/", "my-service"},
		{"/single", "single"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := serviceNameFromPath(tt.path)
			if got != tt.want {
				t.Errorf("serviceNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGenerateBaseOtelEnvVars(t *testing.T) {
	envVars := generateBaseOtelEnvVars("https://abc123.live.dynatrace.com", "dt0c01.TOKEN", "my-svc")

	wantEndpoint := "https://abc123.live.dynatrace.com/api/v2/otlp"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != wantEndpoint {
		t.Errorf("ENDPOINT = %q, want %q", got, wantEndpoint)
	}

	wantHeaders := "Authorization=Api-Token%20dt0c01.TOKEN"
	if got := envVars["OTEL_EXPORTER_OTLP_HEADERS"]; got != wantHeaders {
		t.Errorf("HEADERS = %q, want %q", got, wantHeaders)
	}

	if got := envVars["OTEL_SERVICE_NAME"]; got != "my-svc" {
		t.Errorf("SERVICE_NAME = %q, want %q", got, "my-svc")
	}

	if got := envVars["OTEL_EXPORTER_OTLP_PROTOCOL"]; got != "http/protobuf" {
		t.Errorf("PROTOCOL = %q, want %q", got, "http/protobuf")
	}

	if got := envVars["OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE"]; got != "delta" {
		t.Errorf("TEMPORALITY = %q, want %q", got, "delta")
	}

	for _, key := range []string{"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER"} {
		if got := envVars[key]; got != "otlp" {
			t.Errorf("%s = %q, want %q", key, got, "otlp")
		}
	}
}

func TestGenerateBaseOtelEnvVars_TrailingSlash(t *testing.T) {
	envVars := generateBaseOtelEnvVars("https://abc123.live.dynatrace.com/", "tok", "svc")
	want := "https://abc123.live.dynatrace.com/api/v2/otlp"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != want {
		t.Errorf("ENDPOINT = %q, want %q (trailing slash should be stripped)", got, want)
	}
}

func TestProcessMatchPIDs(t *testing.T) {
	procs := []DetectedProcess{
		{PID: 100, Command: "/usr/bin/python app.py", CWD: "/home/user/projects/my-api"},
		{PID: 200, Command: "node /home/user/projects/my-api/server.js", CWD: "/tmp"},
		{PID: 300, Command: "java -jar other.jar", CWD: "/opt/other"},
	}

	pids := processMatchPIDs("/home/user/projects/my-api", procs)
	sort.Ints(pids)
	if len(pids) != 2 || pids[0] != 100 || pids[1] != 200 {
		t.Errorf("processMatchPIDs = %v, want [100, 200]", pids)
	}
}

func TestProcessMatchPIDs_CaseInsensitive(t *testing.T) {
	procs := []DetectedProcess{
		{PID: 42, Command: "python app.py", CWD: "/Users/Bruno/Projects/MyApp"},
	}
	pids := processMatchPIDs("/users/bruno/projects/myapp", procs)
	if len(pids) != 1 || pids[0] != 42 {
		t.Errorf("processMatchPIDs (case-insensitive) = %v, want [42]", pids)
	}
}

func TestProcessMatchPIDs_NoMatch(t *testing.T) {
	procs := []DetectedProcess{
		{PID: 10, Command: "node index.js", CWD: "/opt/other"},
	}
	pids := processMatchPIDs("/home/user/myproject", procs)
	if len(pids) != 0 {
		t.Errorf("processMatchPIDs = %v, want empty", pids)
	}
}

func TestMatchProcessesToProjects(t *testing.T) {
	projects := []ScannedProject{
		{Path: "/home/user/project-a"},
		{Path: "/home/user/project-b"},
	}
	procs := []DetectedProcess{
		{PID: 1, Command: "python app.py", CWD: "/home/user/project-a"},
		{PID: 2, Command: "node server.js", CWD: "/home/user/project-b"},
		{PID: 3, Command: "node /home/user/project-a/worker.js", CWD: "/tmp"},
	}

	matchProcessesToProjects(projects, procs)

	sort.Ints(projects[0].RunningPIDs)
	if len(projects[0].RunningPIDs) != 2 || projects[0].RunningPIDs[0] != 1 || projects[0].RunningPIDs[1] != 3 {
		t.Errorf("project-a RunningPIDs = %v, want [1, 3]", projects[0].RunningPIDs)
	}
	if len(projects[1].RunningPIDs) != 1 || projects[1].RunningPIDs[0] != 2 {
		t.Errorf("project-b RunningPIDs = %v, want [2]", projects[1].RunningPIDs)
	}
}

func TestEnvVarsToSlice(t *testing.T) {
	m := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	got := envVarsToSlice(m)
	sort.Strings(got)
	want := []string{"BAZ=qux", "FOO=bar"}
	if len(got) != len(want) {
		t.Fatalf("envVarsToSlice length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("envVarsToSlice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEnvVarsToSlice_Empty(t *testing.T) {
	got := envVarsToSlice(map[string]string{})
	if len(got) != 0 {
		t.Errorf("envVarsToSlice(empty) = %v, want empty", got)
	}
}

func TestGenerateEnvExportScript(t *testing.T) {
	envVars := map[string]string{
		"OTEL_SERVICE_NAME": "my-svc",
	}
	script := GenerateEnvExportScript(envVars)
	if !strings.Contains(script, "export OTEL_SERVICE_NAME=") {
		t.Errorf("script missing export line, got:\n%s", script)
	}
	if !strings.Contains(script, "my-svc") {
		t.Errorf("script missing service name, got:\n%s", script)
	}
	if !strings.HasPrefix(script, "# Dynatrace OpenTelemetry") {
		t.Errorf("script missing header comment, got:\n%s", script)
	}
}

func TestScanProjectDirs_CWD(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	// Create a marker file.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

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

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

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

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

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

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

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

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	projects := scanProjectDirs([]string{"go.mod"}, nil)
	realDir, _ := filepath.EvalSymlinks(dir)
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			t.Errorf("empty dir should not appear in results, got %v", projects)
		}
	}
}
