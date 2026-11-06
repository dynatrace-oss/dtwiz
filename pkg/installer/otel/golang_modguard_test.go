package otel

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeModule(t *testing.T, dir, mod string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestModGuardRestore(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "module a\n") // go.sum absent
	g, err := newModGuard(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeModule(t, dir, "module a\nrequire x v1\n")
	_ = os.WriteFile(filepath.Join(dir, "go.sum"), []byte("x"), 0o644)

	if err := g.Restore(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "go.mod")); string(got) != "module a\n" {
		t.Errorf("go.mod not restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.sum")); !os.IsNotExist(err) {
		t.Errorf("go.sum created during build should have been removed, err=%v", err)
	}
	if _, err := os.Stat(modGuardDir(dir)); !os.IsNotExist(err) {
		t.Errorf("backup dir should be removed after a successful restore, err=%v", err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(filepath.Join(dir, "go.mod")); info.Mode().Perm() != 0o640 {
			t.Errorf("file mode not preserved: %v", info.Mode().Perm())
		}
	}
}

func TestModGuardBackupSurvivesOnDisk(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "module a\n")
	if _, err := newModGuard(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(modGuardDir(dir), "go.mod"))
	if err != nil || string(got) != "module a\n" {
		t.Errorf("on-disk backup missing or wrong: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(modGuardDir(dir), "go.sum"+absentSuffix)); err != nil {
		t.Errorf("absent go.sum should leave a marker: %v", err)
	}
}

func TestModGuardInterruptedRunBlocksNextRun(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "module a\n")
	if _, err := newModGuard(dir); err != nil {
		t.Fatal(err)
	}
	// Simulated kill during the build: otelc changed go.mod, Restore never ran.
	writeModule(t, dir, "module a\ngo 1.25\n")

	_, err := newModGuard(dir)
	if err == nil || !strings.Contains(err.Error(), "previous dtwiz build was interrupted") {
		t.Fatalf("expected interrupted-build error, got %v", err)
	}
	// The modified go.mod must not become the new baseline.
	got, _ := os.ReadFile(filepath.Join(modGuardDir(dir), "go.mod"))
	if string(got) != "module a\n" {
		t.Errorf("original backup was overwritten: %q", got)
	}
}

func TestModGuardStaleBackupIsDiscarded(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "module a\n")
	if _, err := newModGuard(dir); err != nil {
		t.Fatal(err)
	}
	// Files unchanged (previous run restored but could not delete the backup).
	if _, err := newModGuard(dir); err != nil {
		t.Fatalf("stale identical backup should not block: %v", err)
	}
}

func TestModGuardRestoreFailureIsReported(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "module a\n")
	g, err := newModGuard(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Make the target unwritable by replacing go.mod with a non-empty directory.
	_ = os.Remove(filepath.Join(dir, "go.mod"))
	if err := os.MkdirAll(filepath.Join(dir, "go.mod", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	err = g.Restore()
	if err == nil || !strings.Contains(err.Error(), modGuardDir(dir)) {
		t.Fatalf("expected error pointing at the backup dir, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(modGuardDir(dir), "go.mod")); statErr != nil {
		t.Errorf("backup must be kept after a failed restore: %v", statErr)
	}
}
