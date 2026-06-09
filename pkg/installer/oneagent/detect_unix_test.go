//go:build !windows

package oneagent

import (
	"os"
	"path/filepath"
	"testing"
)

// withInstallDir overrides oneAgentInstallDir for the duration of the test.
func withInstallDir(t *testing.T, dir string) {
	t.Helper()
	orig := oneAgentInstallDir
	oneAgentInstallDir = dir
	t.Cleanup(func() { oneAgentInstallDir = orig })
}

func TestOneAgentInstalled_DirExists(t *testing.T) {
	withInstallDir(t, t.TempDir())
	if !oneAgentInstalled() {
		t.Error("expected true when install dir exists")
	}
}

func TestOneAgentInstalled_DirNotExist_NoPATH(t *testing.T) {
	withInstallDir(t, filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("PATH", "")
	if oneAgentInstalled() {
		t.Error("expected false when install dir absent and oneagentctl not on PATH")
	}
}

func TestOneAgentInstalled_DirNotExist_FallbackPATH(t *testing.T) {
	// Create a fake oneagentctl executable in a temp dir and put it on PATH.
	bin := t.TempDir()
	stub := filepath.Join(bin, "oneagentctl")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withInstallDir(t, filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("PATH", bin)
	if !oneAgentInstalled() {
		t.Error("expected true when oneagentctl is on PATH")
	}
}
