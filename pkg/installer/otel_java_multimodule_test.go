package installer

import (
	"os"
	"path/filepath"
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

func TestNeedsBuild_TrueWhenJarsMissing(t *testing.T) {
	dir := t.TempDir()
	subs := []SubModule{{Name: "api", Path: dir}}
	if !needsBuild(subs) {
		t.Fatal("expected needsBuild to return true when no JAR is present")
	}
}

func TestNeedsBuild_FalseWhenJarsPresent(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	makeTestJar(t, targetDir, "app.jar", "Manifest-Version: 1.0\nMain-Class: com.example.App\n")
	subs := []SubModule{{Name: "api", Path: dir}}
	if needsBuild(subs) {
		t.Fatal("expected needsBuild to return false when executable JAR is present")
	}
}
