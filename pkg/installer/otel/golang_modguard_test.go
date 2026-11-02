package otel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupGuardModule(t *testing.T, goMod, goSum string) string {
	t.Helper()
	modguardRetryDelay = time.Millisecond
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	if goSum != "" {
		if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte(goSum), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mutateModule(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte("new sum\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestWithModGuard_RestoresAfterSuccess(t *testing.T) {
	dir := setupGuardModule(t, "module a\n", "sum a\n")
	if err := withModGuard(dir, func() error { mutateModule(t, dir); return nil }); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "go.mod")); got != "module a\n" {
		t.Fatalf("go.mod = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "go.sum")); got != "sum a\n" {
		t.Fatalf("go.sum = %q", got)
	}
	if _, err := os.Stat(filepath.Join(goOtelDir(dir), goModguardDirName)); !os.IsNotExist(err) {
		t.Fatal("backup should be removed after a clean restore")
	}
}

func TestWithModGuard_RestoresAfterFailureAndCancellation(t *testing.T) {
	for _, failure := range []error{errors.New("build failed"), errors.New("context canceled")} {
		dir := setupGuardModule(t, "module a\n", "sum a\n")
		err := withModGuard(dir, func() error { mutateModule(t, dir); return failure })
		if !errors.Is(err, failure) {
			t.Fatalf("expected build error to be returned, got %v", err)
		}
		if got := readFile(t, filepath.Join(dir, "go.mod")); got != "module a\n" {
			t.Fatalf("go.mod = %q after %v", got, failure)
		}
	}
}

func TestWithModGuard_UncommittedChangesRestoredByteIdentical(t *testing.T) {
	// A modified go.mod is not special: the backup is a copy of the file as it is.
	dir := setupGuardModule(t, "module a\n// local edit\n", "sum a\n// local edit\n")
	if err := withModGuard(dir, func() error { mutateModule(t, dir); return nil }); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "go.mod")); got != "module a\n// local edit\n" {
		t.Fatalf("go.mod = %q", got)
	}
}

func TestWithModGuard_AbsentGoSumRemoved(t *testing.T) {
	dir := setupGuardModule(t, "module a\n", "")
	if err := withModGuard(dir, func() error { mutateModule(t, dir); return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.sum")); !os.IsNotExist(err) {
		t.Fatal("go.sum created by the build must be removed")
	}
}

func TestNewModGuard_LeftoverDiffersRefuses(t *testing.T) {
	dir := setupGuardModule(t, "module a\n", "sum a\n")
	if _, err := newModGuard(dir); err != nil {
		t.Fatal(err)
	}
	// Simulate an interrupted run: the files were modified and the backup stayed.
	mutateModule(t, dir)

	_, err := newModGuard(dir)
	if err == nil || !strings.Contains(err.Error(), goModguardDirName) {
		t.Fatalf("expected refusal naming the backup path, got %v", err)
	}
	if got := readFile(t, filepath.Join(dir, "go.mod")); got != "module changed\n" {
		t.Fatal("refusal must not touch the module files")
	}
}

func TestNewModGuard_LeftoverIdenticalCleaned(t *testing.T) {
	dir := setupGuardModule(t, "module a\n", "sum a\n")
	if _, err := newModGuard(dir); err != nil {
		t.Fatal(err)
	}
	g, err := newModGuard(dir)
	if err != nil {
		t.Fatalf("identical leftover backup should be treated as stale: %v", err)
	}
	if err := g.Restore(); err != nil {
		t.Fatal(err)
	}
}

func TestNewModGuard_MissingGoMod(t *testing.T) {
	if _, err := newModGuard(t.TempDir()); err == nil {
		t.Fatal("expected an error for a module without go.mod")
	}
}
