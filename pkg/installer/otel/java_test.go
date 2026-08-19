package otel

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func skipIfNoJava(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not installed on PATH")
	}
}

// redirectAgentDownloadURL points otelJavaAgentURL at srv for the duration of
// the test and sets HOME to a fresh temp dir so javaAgentPath() is isolated.
func redirectAgentDownloadURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := otelJavaAgentURL
	otelJavaAgentURL = srv.URL + "/opentelemetry-javaagent.jar"
	t.Cleanup(func() { otelJavaAgentURL = orig })
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
}

// makeMavenProjectWithFatJar creates a temp Maven project containing a single
// executable JAR in target/, sets the working directory, and feeds "1\n" to
// stdin for project selection. Single-entrypoint projects are auto-selected, so
// no additional stdin input is needed.
func makeMavenProjectWithFatJar(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	makeTestJar(t, targetDir, "app.jar", "Manifest-Version: 1.0\nMain-Class: com.example.App\n")
	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")
}

// isolatePathToJava restricts PATH to a temp dir containing only a java
// symlink. This ensures mvn/gradle are not found while java validation still
// passes, so tests that expect "no build tool" errors are not short-circuited
// by a system-installed Maven or Gradle.
func isolatePathToJava(t *testing.T) {
	t.Helper()
	javaPath, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not installed on PATH")
	}
	binDir := t.TempDir()
	if err := os.Symlink(javaPath, filepath.Join(binDir, "java")); err != nil {
		t.Fatalf("symlinking java: %v", err)
	}
	t.Setenv("PATH", binDir)
}

// ── detectJavaProjects tests ──────────────────────────────────────────────────

func TestDetectJavaProjects_Maven(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := detectJavaProjects(defaultScanRoots())
	if len(projects) == 0 {
		t.Fatal("expected at least one Java project, got none")
	}
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			if len(p.Markers) == 0 || p.Markers[0] != "pom.xml" {
				t.Errorf("expected marker pom.xml, got %v", p.Markers)
			}
		}
	}
	if !found {
		t.Errorf("dir %s not found in projects %v", dir, projects)
	}
}

func TestDetectJavaProjects_Gradle(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := detectJavaProjects(defaultScanRoots())
	if len(projects) == 0 {
		t.Fatal("expected at least one Java project, got none")
	}
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			hasGradle := false
			for _, m := range p.Markers {
				if m == "build.gradle" {
					hasGradle = true
				}
			}
			if !hasGradle {
				t.Errorf("expected marker build.gradle, got %v", p.Markers)
			}
		}
	}
	if !found {
		t.Errorf("dir %s not found in projects %v", dir, projects)
	}
}

func TestDetectJavaProjects_None(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	helpers.SetTestWorkingDir(t, dir)
	projects := detectJavaProjects(defaultScanRoots())
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			t.Errorf("unexpected project at %s with no Java markers", dir)
		}
	}
}

// ── DetectJavaPlan tests ──────────────────────────────────────────────────────

func TestDetectJavaPlan_NoJavaOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	plan := DetectJavaPlan("https://tenant.live.dynatrace.com", "token")
	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}
}

func TestDetectJavaPlan_FindsProject(t *testing.T) {
	skipIfNoJava(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	plan := DetectJavaPlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected Java plan")
	}
	if plan.Project.Path == "" {
		t.Fatal("expected selected project path to be set")
	}
	if plan.EnvURL == "" {
		t.Fatal("expected EnvURL to be set in plan")
	}
	if plan.Token == "" {
		t.Fatal("expected Token to be set in plan")
	}
}

func TestDetectJavaPlan_FindsProjectWithEntrypoint(t *testing.T) {
	skipIfNoJava(t)
	makeMavenProjectWithFatJar(t)

	plan := DetectJavaPlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected Java plan")
	}
	if plan.EntrypointCommand == "" {
		t.Fatal("expected EntrypointCommand to be set when fat JAR is present")
	}
	if !strings.Contains(plan.EntrypointCommand, "app.jar") {
		t.Fatalf("expected EntrypointCommand to reference the JAR, got %q", plan.EntrypointCommand)
	}
}

// ── JavaInstrumentationPlan tests ─────────────────────────────────────────────

func TestJavaInstrumentationPlan_Runtime(t *testing.T) {
	plan := &JavaInstrumentationPlan{}
	if got := plan.Runtime(); got != "Java" {
		t.Fatalf("Runtime() = %q, want %q", got, "Java")
	}
}

func TestJavaInstrumentationPlan_PrintPlanSteps(t *testing.T) {
	plan := &JavaInstrumentationPlan{Project: ScannedProject{Path: "/tmp/service"}}

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{"/tmp/service", otelJavaAgentURL, "(entrypoint will be detected at execution time)"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestJavaInstrumentationPlan_PrintPlanSteps_Updated(t *testing.T) {
	plan := &JavaInstrumentationPlan{
		Project:           ScannedProject{Path: "/tmp/service"},
		EntrypointCommand: "java -jar /tmp/app.jar",
		EnvVars: map[string]string{
			"OTEL_SERVICE_NAME": "my-svc",
		},
	}

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{"-javaagent:", otelJavaAgentURL, "OTEL_SERVICE_NAME"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

// TestJavaInstrumentationPlan_Execute verifies that Execute() on an empty plan does
// not panic even though the download will fail (no server reachable).
func TestJavaInstrumentationPlan_Execute(t *testing.T) {
	skipIfNoJava(t)

	origURL := otelJavaAgentURL
	otelJavaAgentURL = "http://127.0.0.1:0/no-such-agent.jar"
	t.Cleanup(func() { otelJavaAgentURL = origURL })

	plan := &JavaInstrumentationPlan{
		Project: ScannedProject{Path: t.TempDir()},
		EnvVars: map[string]string{"OTEL_SERVICE_NAME": "orders-api"},
	}

	helpers.CaptureStdout(t, func() {
		_ = plan.Execute()
	})
}

// TestJavaInstrumentationPlan_Execute_MultiModuleDispatch verifies that Execute()
// routes to executeMultiModule() when SubModules is non-empty. Before the fix,
// Execute() ignored SubModules and fell through to single-module entrypoint
// detection on the root, starting at most one process.
func TestJavaInstrumentationPlan_Execute_MultiModuleDispatch(t *testing.T) {
	skipIfNoJava(t)

	root := t.TempDir()
	subA := filepath.Join(root, "svc-a")
	subB := filepath.Join(root, "svc-b")
	if err := os.MkdirAll(subA, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subB, 0755); err != nil {
		t.Fatal(err)
	}

	agentDownloaded := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentDownloaded = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake-jar"))
	}))
	defer srv.Close()
	redirectAgentDownloadURL(t, srv)

	// Redirect color.Output so display.PrintStatusLine output is captured.
	var colorBuf bytes.Buffer
	origColorOutput := color.Output
	color.Output = &colorBuf
	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() {
		color.Output = origColorOutput
		color.NoColor = origNoColor
	})

	plan := &JavaInstrumentationPlan{
		Project: ScannedProject{Path: root},
		EnvURL:  "https://tenant.live.dynatrace.com",
		Token:   "token",
		SubModules: []SubModulePlan{
			{Name: "svc-a", Path: subA, EnvVars: map[string]string{"OTEL_SERVICE_NAME": "svc-a"}},
			{Name: "svc-b", Path: subB, EnvVars: map[string]string{"OTEL_SERVICE_NAME": "svc-b"}},
		},
	}

	helpers.CaptureStdout(t, func() {
		_ = plan.Execute()
	})
	output := colorBuf.String()

	if !agentDownloaded {
		t.Error("agent download not attempted — Execute() did not dispatch to executeMultiModule()")
	}
	if strings.Contains(output, "no runnable entrypoint detected") {
		t.Error("single-module error found in output — Execute() did not dispatch to executeMultiModule()")
	}
	if !strings.Contains(output, "svc-a") || !strings.Contains(output, "svc-b") {
		t.Errorf("expected both sub-module names in output, got:\n%s", output)
	}
}

// TestDetectJavaPlan_MultiModule_HasSubModules verifies that DetectJavaPlan
// returns a plan with SubModules populated for a Maven multi-module project.
func TestDetectJavaPlan_MultiModule_HasSubModules(t *testing.T) {
	skipIfNoJava(t)

	root := t.TempDir()
	rootPOM := `<?xml version="1.0"?><project xmlns="http://maven.apache.org/POM/4.0.0">` +
		`<modules><module>svc-a</module><module>svc-b</module></modules></project>`
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(rootPOM), 0644); err != nil {
		t.Fatal(err)
	}
	for _, mod := range []string{"svc-a", "svc-b"} {
		dir := filepath.Join(root, mod)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	helpers.SetTestWorkingDir(t, root)
	setTestStdin(t, "1\n")

	plan := DetectJavaPlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected plan for multi-module project")
	}
	if len(plan.SubModules) != 2 {
		t.Fatalf("expected 2 sub-modules, got %d", len(plan.SubModules))
	}
	if plan.SubModules[0].Name != "svc-a" || plan.SubModules[1].Name != "svc-b" {
		t.Errorf("unexpected sub-module names: %v", plan.SubModules)
	}
	if plan.EntrypointCommand != "" {
		t.Errorf("multi-module plan should not have EntrypointCommand set, got %q", plan.EntrypointCommand)
	}
}

// ── Download tests ────────────────────────────────────────────────────────────

func TestDownloadJavaAgent_CreatesDirectory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake-jar-content"))
	}))
	defer srv.Close()
	redirectAgentDownloadURL(t, srv)

	helpers.CaptureStdout(t, func() {
		path, err := downloadJavaAgent()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected agent file at %s: %v", path, statErr)
		}
		if _, statErr := os.Stat(filepath.Dir(path)); statErr != nil {
			t.Fatalf("expected directory to exist: %v", statErr)
		}
	})
}

func TestDownloadJavaAgent_ErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	redirectAgentDownloadURL(t, srv)

	_, err := downloadJavaAgent()
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), srv.URL) {
		t.Errorf("expected error to contain URL %q, got: %v", srv.URL, err)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to contain status code, got: %v", err)
	}
}

func TestDownloadJavaAgent_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()
	redirectAgentDownloadURL(t, srv)

	_, err := downloadJavaAgent()
	if err == nil {
		t.Fatal("expected error for connection closed by server")
	}
}

// ── displayInstrumentedCmd tests ──────────────────────────────────────────────

func TestDisplayInstrumentedCmd_JavaPrefix(t *testing.T) {
	ep := JavaEntrypoint{Command: "java -jar /tmp/app.jar"}
	got := displayInstrumentedCmd(ep, "/home/.opentelemetry/java/opentelemetry-javaagent.jar")
	want := "java -javaagent:/home/.opentelemetry/java/opentelemetry-javaagent.jar -jar /tmp/app.jar"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDisplayInstrumentedCmd_WrapperCmd(t *testing.T) {
	ep := JavaEntrypoint{Command: "./mvnw spring-boot:run"}
	got := displayInstrumentedCmd(ep, "/agent.jar")
	want := `JAVA_TOOL_OPTIONS="-javaagent:/agent.jar" ./mvnw spring-boot:run`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// ── buildInstrumentedCmd tests ────────────────────────────────────────────────

func TestBuildInstrumentedCmd_JavaPrefix(t *testing.T) {
	ep := JavaEntrypoint{Command: "java -jar /tmp/app.jar"}
	cmd := buildInstrumentedCmd(ep, "/agent.jar", "/project", nil)
	if cmd.Args[0] != "java" {
		t.Fatalf("expected binary java, got %q", cmd.Args[0])
	}
	if cmd.Args[1] != "-javaagent:/agent.jar" {
		t.Fatalf("expected -javaagent flag as first arg, got %q", cmd.Args[1])
	}
	if cmd.Dir != "/project" {
		t.Fatalf("expected Dir=/project, got %q", cmd.Dir)
	}
}

func TestBuildInstrumentedCmd_WrapperCmd(t *testing.T) {
	ep := JavaEntrypoint{Command: "./gradlew bootRun"}
	cmd := buildInstrumentedCmd(ep, "/agent.jar", "/project", map[string]string{"OTEL_SERVICE_NAME": "svc"})
	if cmd.Args[0] != "./gradlew" {
		t.Fatalf("expected binary ./gradlew, got %q", cmd.Args[0])
	}
	found := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "JAVA_TOOL_OPTIONS=") && strings.Contains(e, "/agent.jar") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected JAVA_TOOL_OPTIONS with agent path in env, got %v", cmd.Env)
	}
}

// ── InstallOtelJava tests ─────────────────────────────────────────────────────

func TestInstallOtelJava_JavaNotFound(t *testing.T) {
	t.Setenv("PATH", "")
	err := InstallOtelJava("https://tenant.live.dynatrace.com", "token", "svc", "", false)
	if err == nil {
		t.Fatal("expected error when java is not on PATH")
	}
}

func TestInstallOtelJava_DryRun(t *testing.T) {
	skipIfNoJava(t)
	makeMavenProjectWithFatJar(t)

	output := helpers.CaptureStdout(t, func() {
		err := InstallOtelJava("https://tenant.live.dynatrace.com", "tok", "test-svc", "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	checks := []string{
		"http://localhost:4318",
		"test-svc",
		otelJavaAgentURL,
		"OTEL_SERVICE_NAME",
		"-javaagent:",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected dry-run output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestInstallOtelJava_SkipReturnsInstallCancelled(t *testing.T) {
	skipIfNoJava(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "\n") // skip project selection

	helpers.CaptureStdout(t, func() {
		err := InstallOtelJava("https://tenant.live.dynatrace.com", "tok", "", "", false)
		if !errors.Is(err, installer.ErrInstallCancelled) {
			t.Errorf("expected installer.ErrInstallCancelled when skipping, got %v", err)
		}
	})
}

func TestInstallOtelJava_NoBuildArtifact_NoRunningProcess(t *testing.T) {
	skipIfNoJava(t)
	isolatePathToJava(t) // exclude system mvn/gradle so no fallback entrypoint is found

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	helpers.CaptureStdout(t, func() {
		err := InstallOtelJava("https://tenant.live.dynatrace.com", "tok", "", "", false)
		if err == nil {
			t.Fatal("expected error when no build tool is present, got nil")
		}
	})
}

func TestInstallOtelJava_AutoBuildFails(t *testing.T) {
	skipIfNoJava(t)
	// mvnw exists but lacks maven-wrapper.jar, and system mvn is excluded from
	// PATH, so resolveMavenCmd returns "" → detectJavaEntrypoints finds nothing
	// → attemptSingleModuleBuild reports no build tool available.
	isolatePathToJava(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mvnw"), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	helpers.CaptureStdout(t, func() {
		err := InstallOtelJava("https://tenant.live.dynatrace.com", "tok", "", "", false)
		if err == nil {
			t.Fatal("expected error when auto-build fails, got nil")
		}
	})
}

// ── javaAgentPath tests ───────────────────────────────────────────────────────

func TestJavaAgentPath_UsesHomeDirectory(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	path, err := javaAgentPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedSubpath := filepath.Join(".opentelemetry", "java", "opentelemetry-javaagent.jar")
	if !strings.HasSuffix(path, expectedSubpath) {
		t.Fatalf("expected path to end with %q, got %q", expectedSubpath, path)
	}
	if !strings.Contains(path, tmpHome) {
		t.Fatalf("expected path to contain home dir %q, got %q", tmpHome, path)
	}
}

// ── buildInstrumentedCmd tests ────────────────────────────────────────────────

func TestBuildInstrumentedCmd_WithEnvVars(t *testing.T) {
	ep := JavaEntrypoint{
		Command:     "java -jar app.jar",
		Description: "test app",
	}
	agentPath := "/path/to/agent.jar"
	projectDir := "/path/to/project"
	envVars := map[string]string{
		"CUSTOM_VAR":    "custom_value",
		"ANOTHER_VAR":   "another_value",
		"OTEL_EXPORTER": "should_be_overridden",
	}

	cmd := buildInstrumentedCmd(ep, agentPath, projectDir, envVars)
	if cmd == nil {
		t.Fatal("expected non-nil *exec.Cmd")
	}
	if !strings.Contains(cmd.String(), "agent.jar") {
		t.Fatalf("expected agent path in command, got: %s", cmd.String())
	}

	// Check that custom env vars are present
	found := false
	for _, env := range cmd.Env {
		if strings.Contains(env, "CUSTOM_VAR=custom_value") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected custom env var in command env, got: %v", cmd.Env)
	}
}

func TestBuildInstrumentedCmd_JavaPrefixCommand(t *testing.T) {
	ep := JavaEntrypoint{
		Command:     "java -Xmx512m -jar my-app.jar --arg1 value1",
		Description: "test",
	}
	agentPath := "/path/to/agent.jar"
	projectDir := "/tmp"

	cmd := buildInstrumentedCmd(ep, agentPath, projectDir, nil)
	if cmd == nil {
		t.Fatal("expected non-nil *exec.Cmd")
	}
	if base := filepath.Base(cmd.Path); base != "java" && base != "java.exe" {
		t.Fatalf("expected command to be java, got: %s", cmd.Path)
	}
}
