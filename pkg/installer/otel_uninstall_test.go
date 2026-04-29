package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCandidateOtelDirs_IncludesExistingInstallDir(t *testing.T) {
	dir := t.TempDir()
	infos := []otelProcessInfo{
		{pid: 1, binaryPath: filepath.Join(dir, "otelcol"), installDir: dir},
	}
	dirs := candidateOtelDirs(infos)
	if len(dirs) == 0 {
		t.Fatal("expected at least one candidate dir")
	}
	found := false
	for _, d := range dirs {
		if d == dir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s in candidate dirs, got %v", dir, dirs)
	}
}

func TestCandidateOtelDirs_ExcludesNonExistentInstallDir(t *testing.T) {
	infos := []otelProcessInfo{
		{pid: 1, binaryPath: "/nonexistent/otelcol", installDir: "/nonexistent"},
	}
	dirs := candidateOtelDirs(infos)
	for _, d := range dirs {
		if d == "/nonexistent" {
			t.Errorf("non-existent dir /nonexistent should not appear in candidates")
		}
	}
}

func TestCandidateOtelDirs_SkipsEmptyInstallDir(t *testing.T) {
	infos := []otelProcessInfo{
		{pid: 1, binaryPath: "", installDir: ""},
	}
	dirs := candidateOtelDirs(infos)
	for _, d := range dirs {
		if d == "" {
			t.Error("empty string should not appear in candidate dirs")
		}
	}
}

func TestCandidateOtelDirs_DeduplicatesSameDir(t *testing.T) {
	dir := t.TempDir()
	infos := []otelProcessInfo{
		{pid: 1, installDir: dir},
		{pid: 2, installDir: dir},
	}
	dirs := candidateOtelDirs(infos)
	count := 0
	for _, d := range dirs {
		if d == dir {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected dir to appear exactly once, got %d times", count)
	}
}

func TestCandidateOtelDirs_CwdFallback(t *testing.T) {
	base := t.TempDir()
	otelDir := filepath.Join(base, "opentelemetry")
	if err := os.Mkdir(otelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Resolve symlinks so the comparison matches os.Getwd() output on macOS.
	realOtelDir, _ := filepath.EvalSymlinks(otelDir)
	setTestWorkingDir(t, base)

	dirs := candidateOtelDirs(nil)
	found := false
	for _, d := range dirs {
		if d == realOtelDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cwd fallback %s in candidate dirs, got %v", realOtelDir, dirs)
	}
}

func TestRemoveWithRetry_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := removeWithRetry(f); err != nil {
		t.Fatalf("removeWithRetry error: %v", err)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestRemoveWithRetry_RemovesDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(filepath.Join(target, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "nested", "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := removeWithRetry(target); err != nil {
		t.Fatalf("removeWithRetry error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("directory should have been removed")
	}
}

func TestRemoveWithRetry_NonExistentPath(t *testing.T) {
	// os.RemoveAll returns nil for non-existent paths, so this should succeed.
	if err := removeWithRetry("/nonexistent/path/that/does/not/exist"); err != nil {
		t.Errorf("expected nil for non-existent path, got: %v", err)
	}
}
