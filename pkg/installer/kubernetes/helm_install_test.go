package kubernetes

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHelmOperatorArgs_RollbackFlag(t *testing.T) {
	for _, tc := range []struct {
		helmMajor int
		wantFlag  string
	}{
		{3, "--atomic"},
		{4, "--rollback-on-failure"},
	} {
		args := helmOperatorArgs(tc.helmMajor, false)
		found := false
		for _, a := range args {
			if a == tc.wantFlag {
				found = true
			}
		}
		if !found {
			t.Errorf("helmMajor=%d: expected %q in install args", tc.helmMajor, tc.wantFlag)
		}

		args = helmOperatorUpgradeArgs(tc.helmMajor, false)
		found = false
		for _, a := range args {
			if a == tc.wantFlag {
				found = true
			}
		}
		if !found {
			t.Errorf("helmMajor=%d: expected %q in upgrade args", tc.helmMajor, tc.wantFlag)
		}
	}
}

func TestHelmOperatorArgs_DisableCSI(t *testing.T) {
	for _, disableCSI := range []bool{true, false} {
		for _, fn := range []struct {
			name string
			args []string
		}{
			{"install", helmOperatorArgs(3, disableCSI)},
			{"upgrade", helmOperatorUpgradeArgs(3, disableCSI)},
		} {
			hasCSIFlag := false
			for i, a := range fn.args {
				if a == "--set" && i+1 < len(fn.args) && fn.args[i+1] == "csidriver.enabled=false" {
					hasCSIFlag = true
				}
			}
			if disableCSI && !hasCSIFlag {
				t.Errorf("%s: disableCSI=true but --set csidriver.enabled=false not found", fn.name)
			}
			if !disableCSI && hasCSIFlag {
				t.Errorf("%s: disableCSI=false but --set csidriver.enabled=false present", fn.name)
			}
		}
	}
}

// createFakeWinget writes a fake winget executable to a temp directory and
// returns the directory path. exitCode controls what the fake binary exits with.
func createFakeWinget(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		script := filepath.Join(dir, "winget.bat")
		content := "@echo off\r\n"
		if exitCode != 0 {
			content += "exit /b 1\r\n"
		}
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("createFakeWinget: %v", err)
		}
	} else {
		script := filepath.Join(dir, "winget")
		content := "#!/bin/sh\n"
		if exitCode != 0 {
			content += "exit 1\n"
		}
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("createFakeWinget: %v", err)
		}
	}
	return dir
}

func TestInstallHelmWindows_WingetNotFound(t *testing.T) {
	// Use an empty temp dir so exec.LookPath("winget") fails.
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	err := installHelmWindows()
	if err == nil {
		t.Fatal("expected error when winget is not on PATH")
	}
	if !strings.Contains(err.Error(), "winget was not found") {
		t.Errorf("error should mention 'winget was not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "https://helm.sh/docs/intro/install/") {
		t.Errorf("error should contain install URL, got: %v", err)
	}
}

func TestInstallHelmWindows_WingetFails(t *testing.T) {
	dir := createFakeWinget(t, 1)
	t.Setenv("PATH", dir)

	err := installHelmWindows()
	if err == nil {
		t.Fatal("expected error when winget exits non-zero")
	}
	if !strings.Contains(err.Error(), "winget failed") {
		t.Errorf("error should mention 'winget failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "https://helm.sh/docs/intro/install/") {
		t.Errorf("error should contain install URL, got: %v", err)
	}
}

func TestInstallHelmWindows_WingetSucceeds(t *testing.T) {
	dir := createFakeWinget(t, 0)
	t.Setenv("PATH", dir)

	if err := installHelmWindows(); err != nil {
		t.Errorf("expected no error when winget succeeds, got: %v", err)
	}
}

// createFakeHelm writes a fake helm binary to a temp dir and returns that dir.
// "helm version" echoes versionOutput; "helm list" echoes listOutput.
func createFakeHelm(t *testing.T, versionOutput, listOutput string) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		script := filepath.Join(dir, "helm.bat")
		content := "@echo off\r\n" +
			"if \"%1\" == \"version\" echo " + versionOutput + "\r\n" +
			"if \"%1\" == \"list\" echo " + listOutput + "\r\n"
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("createFakeHelm: %v", err)
		}
	} else {
		script := filepath.Join(dir, "helm")
		content := "#!/bin/sh\n" +
			"case \"$1\" in\n" +
			"  version) echo '" + versionOutput + "' ;;\n" +
			"  list) echo '" + listOutput + "' ;;\n" +
			"esac\n"
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("createFakeHelm: %v", err)
		}
	}
	return dir
}

func TestIsHelmInstalled_Found(t *testing.T) {
	dir := createFakeHelm(t, "v3.14.0", "")
	t.Setenv("PATH", dir)
	if !isHelmInstalled() {
		t.Error("expected isHelmInstalled=true when helm is on PATH")
	}
}

func TestIsHelmInstalled_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if isHelmInstalled() {
		t.Error("expected isHelmInstalled=false when helm is not on PATH")
	}
}

func TestHelmMajorVersion_V3(t *testing.T) {
	dir := createFakeHelm(t, "v3.14.0+git", "")
	t.Setenv("PATH", dir)
	v, err := helmMajorVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 3 {
		t.Errorf("expected major version 3, got %d", v)
	}
}

func TestHelmMajorVersion_V4(t *testing.T) {
	dir := createFakeHelm(t, "v4.0.0+git", "")
	t.Setenv("PATH", dir)
	v, err := helmMajorVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 4 {
		t.Errorf("expected major version 4, got %d", v)
	}
}

func TestHelmMajorVersion_NotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := helmMajorVersion()
	if err == nil {
		t.Error("expected error when helm is not installed")
	}
}

func TestIsOperatorInstalled_Installed(t *testing.T) {
	dir := createFakeHelm(t, "v3.14.0", "dynatrace-operator")
	t.Setenv("PATH", dir)
	if !isOperatorInstalled() {
		t.Error("expected isOperatorInstalled=true when operator is listed")
	}
}

func TestIsOperatorInstalled_NotInstalled(t *testing.T) {
	dir := createFakeHelm(t, "v3.14.0", "")
	t.Setenv("PATH", dir)
	if isOperatorInstalled() {
		t.Error("expected isOperatorInstalled=false when operator is not listed")
	}
}
