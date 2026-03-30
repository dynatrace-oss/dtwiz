package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectJavaProjects_Maven(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}

	withCWD(t, dir)
	projects := detectJavaProjects()
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

	withCWD(t, dir)
	projects := detectJavaProjects()
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

	withCWD(t, dir)
	projects := detectJavaProjects()
	for _, p := range projects {
		// The temp dir itself should not appear (no markers).
		if p.Path == dir || p.Path == realDir {
			t.Errorf("unexpected project at %s with no Java markers", dir)
		}
	}
}
