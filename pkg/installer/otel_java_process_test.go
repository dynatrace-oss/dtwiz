package installer

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
	if err := os.WriteFile(filepath.Join(dir, "mvnw"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
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
	found := false
	for _, ep := range entrypoints {
		if ep.Command == "./mvnw spring-boot:run" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected './mvnw spring-boot:run' entrypoint, got %+v", entrypoints)
	}
}

func TestDetectJavaEntrypoints_MavenWrapperNonSpringBoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mvnw"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) != 0 {
		t.Fatalf("expected no entrypoints for Maven non-Spring Boot project, got %+v", entrypoints)
	}
}

func TestDetectJavaEntrypoints_GradleWrapperSpringBoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gradlew"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
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
	found := false
	for _, ep := range entrypoints {
		if ep.Command == "./gradlew bootRun" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected './gradlew bootRun' entrypoint, got %+v", entrypoints)
	}
}

func TestDetectJavaEntrypoints_GradleWrapperNoJar(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gradlew"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte("apply plugin: 'java'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entrypoints := detectJavaEntrypoints(dir)
	if len(entrypoints) == 0 {
		t.Fatal("expected 'gradlew run' entrypoint for non-Spring Boot Gradle project")
	}
	found := false
	for _, ep := range entrypoints {
		if ep.Command == "./gradlew run" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected './gradlew run' entrypoint, got %+v", entrypoints)
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
	captureStdout(t, func() {
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
	captureStdout(t, func() {
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
	captureStdout(t, func() {
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
