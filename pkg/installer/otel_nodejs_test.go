package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectNodeProjects_Found(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	withCWD(t, dir)
	projects := detectNodeProjects()
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected project at %s, got %v", dir, projects)
	}
}

func TestDetectNodeProjects_ExcludesNodeModules(t *testing.T) {
	dir := t.TempDir()
	// Create node_modules subdirectory with a package.json inside.
	nmDir := filepath.Join(dir, "node_modules", "somelib")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Create the real project package.json.
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	withCWD(t, dir)
	projects := detectNodeProjects()
	for _, p := range projects {
		if filepath.Base(filepath.Dir(p.Path)) == "node_modules" {
			t.Errorf("node_modules project should be excluded, found: %s", p.Path)
		}
	}
}

func TestDetectNodeEntrypoints_Main(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "server.js" {
		t.Errorf("expected [server.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_ScriptsStart(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"start":"node app.js"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "app.js" {
		t.Errorf("expected [app.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_Fallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "index.js" {
		t.Errorf("expected [index.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_TypeScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Only a TypeScript variant exists.
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "app.ts" {
		t.Errorf("expected [app.ts], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_None(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 0 {
		t.Errorf("expected empty entrypoints, got %v", eps)
	}
}
