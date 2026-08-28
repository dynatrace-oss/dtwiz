package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/config"
)

func TestLoad_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg == nil || len(cfg.Frontends) != 0 {
		t.Fatal("expected empty config")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "frontends:\n  https://abc.live.dynatrace.com:\n    id: FRONTEND-123\n    frontendName: dtwiz-foo-abc12345\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fe := cfg.Frontends["https://abc.live.dynatrace.com"]
	if fe.ID != "FRONTEND-123" {
		t.Errorf("unexpected ID: %s", fe.ID)
	}
	if fe.FrontendName != "dtwiz-foo-abc12345" {
		t.Errorf("unexpected frontendName: %s", fe.FrontendName)
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "frontends:\n\t bad\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestSave_CreatesDirectoryAndFile(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.ProjectConfig{
		Frontends: map[string]config.FrontendConfig{
			"https://abc.live.dynatrace.com": {ID: "FRONTEND-123", FrontendName: "dtwiz-foo-abc12345"},
		},
	}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load after save: %v", err)
	}
	fe := loaded.Frontends["https://abc.live.dynatrace.com"]
	if fe.ID != "FRONTEND-123" {
		t.Errorf("unexpected ID: %s", fe.ID)
	}
}

func TestSave_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	save(t, dir, map[string]config.FrontendConfig{
		"https://a.live.dynatrace.com": {ID: "OLD"},
	})
	save(t, dir, map[string]config.FrontendConfig{
		"https://a.live.dynatrace.com": {ID: "NEW"},
	})

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Frontends["https://a.live.dynatrace.com"].ID != "NEW" {
		t.Error("expected NEW id after overwrite")
	}
}

func TestLoad_TwoEnvironments(t *testing.T) {
	dir := t.TempDir()
	save(t, dir, map[string]config.FrontendConfig{
		"https://a.live.dynatrace.com": {ID: "A"},
		"https://b.live.dynatrace.com": {ID: "B"},
	})

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Frontends["https://a.live.dynatrace.com"].ID != "A" {
		t.Error("wrong ID for env A")
	}
	if loaded.Frontends["https://b.live.dynatrace.com"].ID != "B" {
		t.Error("wrong ID for env B")
	}
}

func TestLoad_UnknownEnvironmentKey(t *testing.T) {
	dir := t.TempDir()
	save(t, dir, map[string]config.FrontendConfig{
		"https://a.live.dynatrace.com": {ID: "A"},
	})

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	fe := loaded.Frontends["https://unknown.live.dynatrace.com"]
	if fe.ID != "" {
		t.Errorf("expected zero value for unknown env, got %q", fe.ID)
	}
}

func TestSave_UpdateOneEnvironmentPreservesOther(t *testing.T) {
	dir := t.TempDir()
	save(t, dir, map[string]config.FrontendConfig{
		"https://a.live.dynatrace.com": {ID: "A"},
		"https://b.live.dynatrace.com": {ID: "B"},
	})

	// Simulate the read-modify-write pattern callers must use.
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Frontends["https://a.live.dynatrace.com"] = config.FrontendConfig{ID: "A-updated"}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Frontends["https://a.live.dynatrace.com"].ID != "A-updated" {
		t.Error("expected A to be updated")
	}
	if loaded.Frontends["https://b.live.dynatrace.com"].ID != "B" {
		t.Error("expected B to be preserved")
	}
}

func TestLoad_UnknownKeysIgnored(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "futureField: x\nfrontends:\n  https://abc.live.dynatrace.com:\n    id: FRONTEND-123\n    frontendName: n\n    extra: y\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Frontends["https://abc.live.dynatrace.com"].ID; got != "FRONTEND-123" {
		t.Errorf("unexpected ID: %s", got)
	}
}

func TestSave_FileLayout(t *testing.T) {
	dir := t.TempDir()
	save(t, dir, map[string]config.FrontendConfig{
		"https://abc.live.dynatrace.com": {ID: "FRONTEND-123", FrontendName: "dtwiz-foo-abc12345"},
	})

	data, err := os.ReadFile(filepath.Join(dir, ".dtwiz", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := "frontends:\n    https://abc.live.dynatrace.com:\n        id: FRONTEND-123\n        frontendName: dtwiz-foo-abc12345\n"
	if string(data) != want {
		t.Errorf("unexpected file layout:\n%s", data)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions are not supported on Windows")
	}
	dir := t.TempDir()
	save(t, dir, map[string]config.FrontendConfig{"https://a.live.dynatrace.com": {ID: "A"}})

	info, err := os.Stat(filepath.Join(dir, ".dtwiz", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected mode 0600, got %o", perm)
	}
}

// helpers

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".dtwiz"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".dtwiz", "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func save(t *testing.T, dir string, frontends map[string]config.FrontendConfig) {
	t.Helper()
	if err := config.Save(dir, &config.ProjectConfig{Frontends: frontends}); err != nil {
		t.Fatal(err)
	}
}
