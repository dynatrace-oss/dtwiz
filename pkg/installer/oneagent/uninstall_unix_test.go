//go:build !windows

package oneagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupInstallDir_PathAbsent(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "nonexistent")

	if err := cleanupInstallDir(absent, false); err != nil {
		t.Errorf("expected no error for absent path, got: %v", err)
	}
}

func TestCleanupInstallDir_PathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "somefile")
	if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := cleanupInstallDir(file, false); err != nil {
		t.Errorf("expected no error for file path, got: %v", err)
	}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Error("file was removed, but cleanupInstallDir should skip non-directories")
	}
}

func TestCleanupInstallDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "stub")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}

	if err := cleanupInstallDir(target, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestCleanupInstallDir_NonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "stub")
	if err := os.MkdirAll(filepath.Join(target, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "sub", "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := cleanupInstallDir(target, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("expected directory tree to be removed")
	}
}
