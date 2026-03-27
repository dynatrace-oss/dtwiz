package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGoProjects_Found(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	goMod := "module github.com/example/myapp\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	projects := detectGoProjects()
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			if p.ModuleName != "github.com/example/myapp" {
				t.Errorf("expected module name github.com/example/myapp, got %q", p.ModuleName)
			}
		}
	}
	if !found {
		t.Errorf("expected Go project at %s, got %v", dir, projects)
	}
}

func TestDetectGoProjects_None(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	projects := detectGoProjects()
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			t.Errorf("unexpected Go project at %s with no go.mod", dir)
		}
	}
}

func TestExtractGoModuleName(t *testing.T) {
	dir := t.TempDir()
	content := "module github.com/acme/service\n\ngo 1.22\n"
	goModPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := extractGoModuleName(goModPath)
	if got != "github.com/acme/service" {
		t.Errorf("expected github.com/acme/service, got %q", got)
	}
}
