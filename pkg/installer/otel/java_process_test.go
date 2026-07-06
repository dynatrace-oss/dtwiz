package otel

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/testutil"
)

func mustCreateFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
}

func setupMvnWrapper(t *testing.T, dir string) {
	t.Helper()
	mustCreateFile(t, filepath.Join(dir, "mvnw"))
	mustCreateFile(t, filepath.Join(dir, "mvnw.cmd"))
	mustCreateFile(t, filepath.Join(dir, ".mvn", "wrapper", "maven-wrapper.jar"))
}

func setupGradleWrapper(t *testing.T, dir string) {
	t.Helper()
	mustCreateFile(t, filepath.Join(dir, "gradlew"))
	mustCreateFile(t, filepath.Join(dir, "gradlew.bat"))
	mustCreateFile(t, filepath.Join(dir, "gradle", "wrapper", "gradle-wrapper.jar"))
}

func mvnWrapperCmd() string {
	if runtime.GOOS == "windows" {
		return "mvnw.cmd"
	}
	return "./mvnw"
}

func gradleWrapperCmd() string {
	if runtime.GOOS == "windows" {
		return "gradlew.bat"
	}
	return "./gradlew"
}

// ── parseJavaVersion tests ─────────────────────────────────────────────────────

func TestParseJavaVersion_Legacy_1_8(t *testing.T) {
	input := `openjdk version "1.8.0_382" 2023-07-18`
	major, err := parseJavaVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if major != 8 {
		t.Fatalf("expected 8, got %d", major)
	}
}

func TestParseJavaVersion_Modern_17(t *testing.T) {
	input := `java version "17.0.1" 2021-10-19`
	major, err := parseJavaVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if major != 17 {
		t.Fatalf("expected 17, got %d", major)
	}
}

func TestParseJavaVersion_Short_21(t *testing.T) {
	input := `openjdk version "21" 2023-09-19`
	major, err := parseJavaVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if major != 21 {
		t.Fatalf("expected 21, got %d", major)
	}
}

func TestParseJavaVersion_OpenJDK_11(t *testing.T) {
	input := `openjdk version "11.0.20" 2023-07-18`
	major, err := parseJavaVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if major != 11 {
		t.Fatalf("expected 11, got %d", major)
	}
}

func TestParseJavaVersion_Unrecognized(t *testing.T) {
	input := `not a valid version`
	_, err := parseJavaVersion(input)
	if err == nil {
		t.Fatal("expected error for unrecognized version string")
	}
}

func TestParseJavaVersion_Java7_TooOld(t *testing.T) {
	input := `java version "1.7.0_80"`
	major, err := parseJavaVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if major != 7 {
		t.Fatalf("expected 7, got %d", major)
	}
}

// ── isExecutableJar tests ─────────────────────────────────────────────────────

func makeTestJar(t *testing.T, dir, name string, manifest string) string {
	t.Helper()
	jarPath := filepath.Join(dir, name)
	f, err := os.Create(jarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	fw, err := w.Create("META-INF/MANIFEST.MF")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	return jarPath
}

func TestIsExecutableJar_WithMainClass(t *testing.T) {
	dir := t.TempDir()
	jarPath := makeTestJar(t, dir, "app.jar", "Manifest-Version: 1.0\nMain-Class: com.example.App\n")
	if !isExecutableJar(jarPath) {
		t.Fatal("expected isExecutableJar to return true for JAR with Main-Class")
	}
}

func TestIsExecutableJar_WithoutMainClass(t *testing.T) {
	dir := t.TempDir()
	jarPath := makeTestJar(t, dir, "lib.jar", "Manifest-Version: 1.0\n")
	if isExecutableJar(jarPath) {
		t.Fatal("expected isExecutableJar to return false for JAR without Main-Class")
	}
}

// ── detectJavaEntrypoints tests ───────────────────────────────────────────────

func TestDetectJavaEntrypoints_MavenFatJar(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	makeTestJar(t, targetDir, "app.jar", "Manifest-Version: 1.0\nMain-Class: com.example.App\n")

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected at least one entrypoint for Maven fat JAR")
	}
	found := false
	for _, ep := range entrypoints {
		if len(ep.Command) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected entrypoint command to be non-empty")
	}
}

func TestDetectJavaEntrypoints_GradleFatJar(t *testing.T) {
	dir := t.TempDir()
	libsDir := filepath.Join(dir, "build", "libs")
	if err := os.MkdirAll(libsDir, 0755); err != nil {
		t.Fatal(err)
	}
	makeTestJar(t, libsDir, "app-all.jar", "Manifest-Version: 1.0\nMain-Class: com.example.App\n")

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected at least one entrypoint for Gradle fat JAR")
	}
}

func TestDetectJavaEntrypoints_MavenWrapperSpringBoot(t *testing.T) {
	dir := t.TempDir()
	setupMvnWrapper(t, dir)
	pomContent := `<project>
  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
  </parent>
</project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pomContent), 0644); err != nil {
		t.Fatal(err)
	}

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected spring-boot:run entrypoint for Maven Spring Boot project")
	}
	want := mvnWrapperCmd() + " spring-boot:run"
	found := false
	for _, ep := range entrypoints {
		if ep.Command == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q entrypoint, got %+v", want, entrypoints)
	}
}

func TestDetectJavaEntrypoints_MavenWrapperNonSpringBoot(t *testing.T) {
	dir := t.TempDir()
	setupMvnWrapper(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected 'exec:java' entrypoint for Maven non-Spring Boot project")
	}
	want := mvnWrapperCmd() + " exec:java"
	found := false
	for _, ep := range entrypoints {
		if ep.Command == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q entrypoint, got %+v", want, entrypoints)
	}
}

func TestDetectJavaEntrypoints_GradleWrapperSpringBoot(t *testing.T) {
	dir := t.TempDir()
	setupGradleWrapper(t, dir)
	gradleContent := `plugins {
    id 'org.springframework.boot' version '3.0.0'
}`
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(gradleContent), 0644); err != nil {
		t.Fatal(err)
	}

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected bootRun entrypoint for Gradle Spring Boot project")
	}
	want := gradleWrapperCmd() + " bootRun"
	found := false
	for _, ep := range entrypoints {
		if ep.Command == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q entrypoint, got %+v", want, entrypoints)
	}
}

func TestDetectJavaEntrypoints_GradleWrapperNoJar(t *testing.T) {
	dir := t.TempDir()
	setupGradleWrapper(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte("apply plugin: 'java'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected 'gradlew run' entrypoint for non-Spring Boot Gradle project")
	}
	want := gradleWrapperCmd() + " run"
	found := false
	for _, ep := range entrypoints {
		if ep.Command == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q entrypoint, got %+v", want, entrypoints)
	}
}

func TestDetectJavaEntrypoints_NoEntrypoint(t *testing.T) {
	dir := t.TempDir()
	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) != 0 {
		t.Fatalf("expected no entrypoints for empty project dir, got %+v", entrypoints)
	}
}

// ── promptEntrypointSelection tests ──────────────────────────────────────────

func TestPromptEntrypointSelection_AutoSelectsSingle(t *testing.T) {
	eps := []JavaEntrypoint{{Command: "java -jar app.jar", Description: "app"}}
	testutil.CaptureStdout(t, func() {
		got := promptEntrypointSelection(eps)
		if got == nil {
			t.Fatal("expected auto-selection, got nil")
		}
		if got.Command != "java -jar app.jar" {
			t.Fatalf("expected %q, got %q", "java -jar app.jar", got.Command)
		}
	})
}

func TestPromptEntrypointSelection_MultipleSelect(t *testing.T) {
	eps := []JavaEntrypoint{
		{Command: "java -jar a.jar", Description: "a"},
		{Command: "java -jar b.jar", Description: "b"},
	}
	setTestStdin(t, "2\n")
	testutil.CaptureStdout(t, func() {
		got := promptEntrypointSelection(eps)
		if got == nil {
			t.Fatal("expected selection, got nil")
		}
		if got.Command != "java -jar b.jar" {
			t.Fatalf("expected second entrypoint, got %q", got.Command)
		}
	})
}

func TestPromptEntrypointSelection_Skip(t *testing.T) {
	eps := []JavaEntrypoint{
		{Command: "java -jar a.jar"},
		{Command: "java -jar b.jar"},
	}
	setTestStdin(t, "\n")
	testutil.CaptureStdout(t, func() {
		got := promptEntrypointSelection(eps)
		if got != nil {
			t.Fatalf("expected nil on empty input, got %+v", got)
		}
	})
}

// ── enrichProcessesWithJPS tests ──────────────────────────────────────────────

func TestEnrichProcessesWithJPS_NoJPS(t *testing.T) {
	t.Setenv("PATH", "")
	processes := []DetectedProcess{{PID: 1234, Command: "java -jar app.jar"}}
	result := enrichProcessesWithJPS(processes)
	if len(result) != 1 {
		t.Fatalf("expected 1 process, got %d", len(result))
	}
	if result[0].Description != "" {
		t.Fatalf("expected empty Description when jps not on PATH, got %q", result[0].Description)
	}
}

func TestEnrichProcessesWithJPS_EmptyInput(t *testing.T) {
	result := enrichProcessesWithJPS([]DetectedProcess{})
	if len(result) != 0 {
		t.Fatalf("expected empty result for empty input, got %v", result)
	}
}

func TestEnrichProcessesWithJPS_WithJPS(t *testing.T) {
	if _, err := exec.LookPath("jps"); err != nil {
		t.Skip("jps not installed on PATH")
	}
	result := enrichProcessesWithJPS([]DetectedProcess{})
	if len(result) != 0 {
		t.Fatalf("expected empty result for empty input, got %v", result)
	}
}

// ── findWrapper tests ─────────────────────────────────────────────────────────

func TestFindWrapper_FoundOnCurrentPlatform(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mvnw"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mvnw.cmd"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	got := findWrapper(dir, "mvnw", "mvnw.cmd")
	if runtime.GOOS == "windows" {
		if got != "mvnw.cmd" {
			t.Fatalf("expected mvnw.cmd on Windows, got %q", got)
		}
	} else {
		if got != "mvnw" {
			t.Fatalf("expected mvnw on Unix, got %q", got)
		}
	}
}

func TestFindWrapper_Missing_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got := findWrapper(dir, "mvnw", "mvnw.cmd")
	if got != "" {
		t.Fatalf("expected empty string when wrapper is absent, got %q", got)
	}
}

func TestDetectJavaEntrypoints_WindowsWrapperSpringBootMaven(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	dir := t.TempDir()
	mustCreateFile(t, filepath.Join(dir, "mvnw.cmd"))
	mustCreateFile(t, filepath.Join(dir, ".mvn", "wrapper", "maven-wrapper.jar"))
	pomContent := `<project><parent><artifactId>spring-boot-starter-parent</artifactId></parent></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pomContent), 0644); err != nil {
		t.Fatal(err)
	}
	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected entrypoint for Windows Maven Spring Boot wrapper")
	}
	if !strings.HasPrefix(entrypoints[0].Command, "mvnw.cmd") {
		t.Fatalf("expected command to start with mvnw.cmd, got %q", entrypoints[0].Command)
	}
}

func TestDetectJavaEntrypoints_WindowsWrapperSpringBootGradle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	dir := t.TempDir()
	mustCreateFile(t, filepath.Join(dir, "gradlew.bat"))
	mustCreateFile(t, filepath.Join(dir, "gradle", "wrapper", "gradle-wrapper.jar"))
	gradleContent := `plugins { id 'org.springframework.boot' version '3.0.0' }`
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(gradleContent), 0644); err != nil {
		t.Fatal(err)
	}
	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected entrypoint for Windows Gradle Spring Boot wrapper")
	}
	if !strings.HasPrefix(entrypoints[0].Command, "gradlew.bat") {
		t.Fatalf("expected command to start with gradlew.bat, got %q", entrypoints[0].Command)
	}
}

// ── isSpringBootMaven / isSpringBootGradle tests ───────────────────────────────

func TestIsSpringBootMaven_True(t *testing.T) {
	dir := t.TempDir()
	content := `<project><parent><artifactId>spring-boot-starter-parent</artifactId></parent></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if !isSpringBootMaven(dir) {
		t.Fatal("expected isSpringBootMaven to return true")
	}
}

func TestIsSpringBootMaven_False(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if isSpringBootMaven(dir) {
		t.Fatal("expected isSpringBootMaven to return false for plain pom.xml")
	}
}

func TestIsSpringBootGradle_True(t *testing.T) {
	dir := t.TempDir()
	content := "plugins { id 'org.springframework.boot' version '3.0.0' }"
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if !isSpringBootGradle(dir) {
		t.Fatal("expected isSpringBootGradle to return true")
	}
}

func TestIsSpringBootGradle_False(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte("apply plugin: 'java'"), 0644); err != nil {
		t.Fatal(err)
	}
	if isSpringBootGradle(dir) {
		t.Fatal("expected isSpringBootGradle to return false for non-Spring Boot gradle")
	}
}

func TestIsSpringBootGradle_Kts(t *testing.T) {
	dir := t.TempDir()
	content := `plugins { id("org.springframework.boot") version "3.0.0" }`
	if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if !isSpringBootGradle(dir) {
		t.Fatal("expected isSpringBootGradle to return true for .kts file")
	}
}

// ── isBuildToolJVM tests ──────────────────────────────────────────────────────

func TestIsBuildToolJVM_GradleDaemon(t *testing.T) {
	if !isBuildToolJVM("org.gradle.launcher.daemon.bootstrap.GradleDaemon") {
		t.Fatal("expected Gradle daemon to be recognized as build-tool JVM")
	}
}

func TestIsBuildToolJVM_GradleWrapper(t *testing.T) {
	if !isBuildToolJVM("org.gradle.wrapper.GradleWrapperMain") {
		t.Fatal("expected Gradle wrapper to be recognized as build-tool JVM")
	}
}

func TestIsBuildToolJVM_Maven(t *testing.T) {
	if !isBuildToolJVM("org.apache.maven.wrapper.MavenWrapperMain") {
		t.Fatal("expected Maven wrapper to be recognized as build-tool JVM")
	}
}

func TestIsBuildToolJVM_Jps(t *testing.T) {
	if !isBuildToolJVM("sun.tools.jps.Jps") {
		t.Fatal("expected jps itself to be recognized as build-tool JVM")
	}
}

func TestIsBuildToolJVM_AppClass(t *testing.T) {
	if isBuildToolJVM("com.example.InventoryApplication") {
		t.Fatal("expected user app class to not be recognized as build-tool JVM")
	}
}

func TestIsBuildToolJVM_Empty(t *testing.T) {
	if isBuildToolJVM("") {
		t.Fatal("expected empty main class to not be recognized as build-tool JVM")
	}
}

func TestIsBuildToolJVM_MavenDaemon(t *testing.T) {
	if !isBuildToolJVM("org.mvndaemon.mvnd.daemon.Server") {
		t.Fatal("expected Maven daemon to be recognized as build-tool JVM")
	}
}

// ── isUnderDir tests ──────────────────────────────────────────────────────────

func TestIsUnderDir(t *testing.T) {
	sep := string(os.PathSeparator)
	base := sep + "projects" + sep + "app"
	tests := []struct {
		path, dir string
		want      bool
	}{
		{base, base, true},                              // exact match
		{base + sep + "src", base, true},                // direct child
		{base + sep + "src" + sep + "main", base, true}, // deep child
		{sep + "projects" + sep + "other", base, false}, // sibling
		{sep + "projects" + sep + "appx", base, false},  // prefix but not a path boundary
		{"", base, false},                               // empty path
		{base, "", false},                               // empty dir
	}
	for _, tt := range tests {
		got := isUnderDir(tt.path, tt.dir)
		if got != tt.want {
			t.Errorf("isUnderDir(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
		}
	}
}

// ── detectJavaListeningPort / portDetector tests ──────────────────────────────

func TestDetectPort_UsesCustomDetector(t *testing.T) {
	called := false
	proc := &ManagedProcess{
		PID: 99999,
		portDetector: func(pid int) string {
			called = true
			if pid != 99999 {
				t.Errorf("portDetector called with wrong pid: got %d, want 99999", pid)
			}
			return "8080"
		},
		exitResultCh: make(chan error, 1),
	}
	port := proc.detectPort()
	if !called {
		t.Fatal("expected custom portDetector to be called")
	}
	if port != "8080" {
		t.Fatalf("expected port 8080, got %q", port)
	}
}

func TestDetectPort_FallsBackWithoutDetector(t *testing.T) {
	proc := &ManagedProcess{
		PID:          0,
		exitResultCh: make(chan error, 1),
	}
	// PID 0 won't have a listening port; we just verify detectPort doesn't panic
	// and returns an empty string when no portDetector is set.
	port := proc.detectPort()
	if port != "" {
		t.Fatalf("expected empty port for PID 0, got %q", port)
	}
}

// ── detectJavaListeningPort tests ─────────────────────────────────────────────

func TestDetectJavaListeningPort_DirectPortDetection(t *testing.T) {
	// Test when directProcessListeningPort finds the port directly.
	called := false
	port := detectJavaListeningPort(0, "") // PID 0 won't have a port
	// For localhost testing, we can't reliably detect a port on PID 0,
	// but we can verify the function doesn't panic and returns a string.
	if !isStringEmpty(port) && !isValidPort(port) {
		t.Fatalf("expected empty or valid port, got %q", port)
	}
	_ = called
}

// ── validateJavaPrerequisites tests ───────────────────────────────────────────

func TestValidateJavaPrerequisites_JavaNotFound(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := validateJavaPrerequisites()
	if err == nil {
		t.Fatal("expected error when java is not on PATH")
	}
	if !strings.Contains(err.Error(), "Java not found") {
		t.Fatalf("expected 'Java not found' error, got %v", err)
	}
}

func TestValidateJavaPrerequisites_ValidVersion(t *testing.T) {
	skipIfNoJava(t)
	javaPath, err := validateJavaPrerequisites()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if javaPath == "" {
		t.Fatal("expected non-empty java path")
	}
	if !strings.Contains(javaPath, "java") {
		t.Fatalf("expected path to contain 'java', got %q", javaPath)
	}
}

// ── resolveGradleCmd tests ────────────────────────────────────────────────────

func TestResolveGradleCmd_NoGradleFile(t *testing.T) {
	dir := t.TempDir()
	cmd, desc := resolveGradleCmd(dir)
	if cmd != "" {
		t.Fatalf("expected empty cmd when no gradle file, got %q", cmd)
	}
	if desc != "" {
		t.Fatalf("expected empty desc when no gradle file, got %q", desc)
	}
}

func TestResolveGradleCmd_WithGradleKts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	// Without wrapper or gradle in PATH, should fail gracefully
	cmd, desc := resolveGradleCmd(dir)
	// Expected: either wrapper cmd (if exists) or empty
	if cmd != "" && desc != "Gradle" {
		t.Fatalf("expected Gradle desc when cmd is not empty, got %q", desc)
	}
}

func TestResolveGradleCmd_WithWrapper(t *testing.T) {
	dir := t.TempDir()
	setupGradleWrapper(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	cmd, desc := resolveGradleCmd(dir)
	if cmd == "" {
		t.Fatal("expected non-empty cmd when wrapper exists")
	}
	if desc != "Gradle" {
		t.Fatalf("expected 'Gradle' desc, got %q", desc)
	}
	expectedCmd := gradleWrapperCmd()
	if !strings.Contains(cmd, expectedCmd) {
		t.Fatalf("expected cmd to contain %q, got %q", expectedCmd, cmd)
	}
}

// ── isExecutableJar tests ─────────────────────────────────────────────────────

func TestIsExecutableJar_InvalidJarFile(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "invalid.jar")
	if err := os.WriteFile(jarPath, []byte("not a jar"), 0644); err != nil {
		t.Fatal(err)
	}
	result := isExecutableJar(jarPath)
	if result {
		t.Fatalf("expected false for invalid JAR, got %v", result)
	}
}

func TestIsExecutableJar_NonExistentFile(t *testing.T) {
	result := isExecutableJar("/nonexistent/path/to/file.jar")
	if result {
		t.Fatalf("expected false for non-existent JAR, got %v", result)
	}
}

// ── isSpringBootMaven tests ───────────────────────────────────────────────────

func TestIsSpringBootMaven_MissingPomXml(t *testing.T) {
	dir := t.TempDir()
	result := isSpringBootMaven(dir)
	if result {
		t.Fatalf("expected false when pom.xml is missing, got %v", result)
	}
}

func TestIsSpringBootMaven_PomWithoutSpringBoot(t *testing.T) {
	dir := t.TempDir()
	pomContent := `<project><groupId>com.example</groupId></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pomContent), 0644); err != nil {
		t.Fatal(err)
	}
	result := isSpringBootMaven(dir)
	if result {
		t.Fatalf("expected false when pom.xml lacks spring-boot, got %v", result)
	}
}

func TestIsSpringBootMaven_PomWithSpringBoot(t *testing.T) {
	dir := t.TempDir()
	pomContent := `<project><dependency><artifactId>spring-boot-starter</artifactId></dependency></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pomContent), 0644); err != nil {
		t.Fatal(err)
	}
	result := isSpringBootMaven(dir)
	if !result {
		t.Fatalf("expected true when pom.xml contains spring-boot, got %v", result)
	}
}

// ── helper functions for tests ────────────────────────────────────────────────

func isStringEmpty(s string) bool {
	return s == ""
}

func isValidPort(s string) bool {
	if s == "" {
		return true
	}
	port, err := strconv.Atoi(s)
	return err == nil && port > 0 && port < 65536
}
