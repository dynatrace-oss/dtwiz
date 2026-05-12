package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── multi-module detection tests ──────────────────────────────────────────────

func TestIsMavenMultiModule_MultiModule(t *testing.T) {
	dir := t.TempDir()
	pom := `<project>
  <modules>
    <module>api</module>
    <module>web</module>
  </modules>
</project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatal(err)
	}
	if !isMavenMultiModule(dir) {
		t.Fatal("expected isMavenMultiModule to return true for multi-module pom.xml")
	}
}

func TestIsMavenMultiModule_SingleModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if isMavenMultiModule(dir) {
		t.Fatal("expected isMavenMultiModule to return false for single-module pom.xml")
	}
}

func TestParseMavenModules_ReturnsModuleNames(t *testing.T) {
	dir := t.TempDir()
	pom := `<project>
  <modules>
    <module>api</module>
    <module>web</module>
    <module>core</module>
  </modules>
</project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatal(err)
	}
	modules, err := parseMavenModules(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(modules) != 3 {
		t.Fatalf("expected 3 modules, got %d: %v", len(modules), modules)
	}
	want := []string{"api", "web", "core"}
	for i, w := range want {
		if modules[i] != w {
			t.Fatalf("expected module[%d]=%q, got %q", i, w, modules[i])
		}
	}
}

func TestParseGradleSubprojects_ColonNotation(t *testing.T) {
	dir := t.TempDir()
	settings := "include ':api'\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}
	subs, err := parseGradleSubprojects(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 1 || subs[0] != "api" {
		t.Fatalf("expected [\"api\"], got %v", subs)
	}
}

func TestParseGradleSubprojects_NestedPath(t *testing.T) {
	dir := t.TempDir()
	settings := "include ':ui:web'\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}
	subs, err := parseGradleSubprojects(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 1 || subs[0] != "ui/web" {
		t.Fatalf("expected [\"ui/web\"], got %v", subs)
	}
}

func TestParseGradleSubprojects_MultiArgParens(t *testing.T) {
	dir := t.TempDir()
	settings := `include("api", "web")` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}
	subs, err := parseGradleSubprojects(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 || subs[0] != "api" || subs[1] != "web" {
		t.Fatalf("expected [api web], got %v", subs)
	}
}

func TestParseGradleSubprojects_MultiArgGroovy(t *testing.T) {
	dir := t.TempDir()
	settings := "include ':api', ':web'\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}
	subs, err := parseGradleSubprojects(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 || subs[0] != "api" || subs[1] != "web" {
		t.Fatalf("expected [api web], got %v", subs)
	}
}

func TestDetectMultiModule_Maven(t *testing.T) {
	dir := t.TempDir()
	pom := `<project>
  <modules>
    <module>api</module>
    <module>web</module>
  </modules>
</project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatal(err)
	}
	mm := detectMultiModule(dir)
	if mm == nil {
		t.Fatal("expected non-nil MultiModuleProject for Maven multi-module project")
	}
	if mm.BuildTool != "maven" {
		t.Fatalf("expected BuildTool=maven, got %q", mm.BuildTool)
	}
	if len(mm.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(mm.Modules))
	}
	names := []string{mm.Modules[0].Name, mm.Modules[1].Name}
	if names[0] != "api" || names[1] != "web" {
		t.Fatalf("expected modules [api web], got %v", names)
	}
}

func TestDetectMultiModule_Gradle(t *testing.T) {
	dir := t.TempDir()
	settings := "include ':api'\ninclude ':web'\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gradlew"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	mm := detectMultiModule(dir)
	if mm == nil {
		t.Fatal("expected non-nil MultiModuleProject for Gradle multi-project")
	}
	if mm.BuildTool != "gradle" {
		t.Fatalf("expected BuildTool=gradle, got %q", mm.BuildTool)
	}
	if len(mm.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(mm.Modules))
	}
}

func TestDetectMultiModule_NilForSingleModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if detectMultiModule(dir) != nil {
		t.Fatal("expected nil for single-module project")
	}
}

// ── parseMavenModules tests ───────────────────────────────────────────────────

func TestParseMavenModules_InvalidPath(t *testing.T) {
	result, err := parseMavenModules("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result on error, got %v", result)
	}
}

func TestParseMavenModules_EmptyModules(t *testing.T) {
	dir := t.TempDir()
	// Create pom.xml with no modules
	pomContent := `<?xml version="1.0"?><project><modelVersion>4.0.0</modelVersion></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pomContent), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := parseMavenModules(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result for pom with no modules, got %v", result)
	}
}

// ── mavenBuildCommand tests ───────────────────────────────────────────────────

func TestMavenBuildCommand_NoPomXml(t *testing.T) {
	dir := t.TempDir()
	cmd := mavenBuildCommand(dir)
	if cmd != "" {
		t.Fatalf("expected empty cmd when pom.xml missing, got %q", cmd)
	}
}

func TestMavenBuildCommand_WithMvnWrapper(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mvnw"), []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mvnw.cmd"), []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".mvn", "wrapper"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mvn", "wrapper", "maven-wrapper.jar"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := mavenBuildCommand(dir)
	if cmd == "" {
		t.Fatal("expected non-empty cmd when wrapper exists")
	}
	if !strings.Contains(cmd, "clean package") {
		t.Fatalf("expected cmd to contain 'clean package', got %q", cmd)
	}
}

// ── gradleBuildCommand tests ──────────────────────────────────────────────────

func TestGradleBuildCommand_NoBuildFile(t *testing.T) {
	dir := t.TempDir()
	cmd := gradleBuildCommand(dir)
	if cmd != "" {
		t.Fatalf("expected empty cmd when build file missing, got %q", cmd)
	}
}

func TestGradleBuildCommand_WithGradlew(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gradlew"), []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gradlew.bat"), []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "gradle", "wrapper"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gradle", "wrapper", "gradle-wrapper.jar"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := gradleBuildCommand(dir)
	if cmd == "" {
		t.Fatal("expected non-empty cmd when gradlew exists")
	}
	if !strings.Contains(cmd, "build") {
		t.Fatalf("expected cmd to contain 'build', got %q", cmd)
	}
}
