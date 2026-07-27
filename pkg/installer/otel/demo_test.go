package otel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/version"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestConfirmProceedAutoConfirm(t *testing.T) {
	orig := installer.AutoConfirm
	defer func() { installer.AutoConfirm = orig }()

	installer.AutoConfirm = true
	ok, err := installer.ConfirmProceed("Test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ConfirmProceed to return true when AutoConfirm=true")
	}
}

// TestInstallOtelCollectorWithProject_DryRun verifies that dry-run goes through
// the full interactive flow (project detection) but makes no changes on disk.
func TestInstallOtelCollectorWithProject_DryRun(t *testing.T) {
	dir := t.TempDir()
	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "y\n")

	output := helpers.CaptureStdout(t, func() {
		err := InstallOtelCollectorWithProject(
			"https://fake.live.dynatrace.com", "tok", "tok", "", true,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// The collector config preview (printed via fmt.Printf) must be present.
	if !strings.Contains(output, "fake.live.dynatrace.com") {
		t.Fatalf("expected collector config in dry-run output, got:\n%s", output)
	}
	// No files must be written.
	if _, err := os.Stat(filepath.Join(dir, "opentelemetry")); !os.IsNotExist(err) {
		t.Fatal("dry-run must not create the opentelemetry directory")
	}
}

func TestInstallOtelCollectorWithProjectPathNotFound(t *testing.T) {
	err := InstallOtelCollectorWithProject("https://fake.live.dynatrace.com", "tok", "tok", "/nonexistent/path", false)
	if err == nil {
		t.Fatal("expected error for non-existent project path")
	}
	if err.Error() != "project path not found: /nonexistent/path" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestInstallOtelPythonProjectPathNotFound(t *testing.T) {
	err := InstallOtelPython("https://fake.live.dynatrace.com", "tok", "plat", "", "/nonexistent/path", false)
	if err == nil {
		t.Fatal("expected error for non-existent project path")
	}
	if err.Error() != "project path not found: /nonexistent/path" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestDemoInstallCmdCurrentOS verifies that pythonInstallPlan returns a non-nil
// command (or no error) on the current OS when any Python prerequisite is missing.
func TestDemoInstallCmdCurrentOS(t *testing.T) {
	cmd, err := pythonInstallPlan()
	if err != nil && cmd != nil {
		t.Fatalf("should not return both a command and an error: cmd=%v err=%v", cmd, err)
	}
	// On unsupported OS pythonInstallPlan returns (nil, nil) — acceptable.
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		// On these OSes we expect either nil (prerequisites already satisfied) or a valid install command slice
		if err != nil {
			// Acceptable only on macOS without brew
			t.Logf("pythonInstallPlan returned error (expected on macOS without brew): %v", err)
		}
	}
}

func TestInstallPythonWindows_WingetNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := installPythonWindows()
	if err == nil {
		t.Fatal("expected error when winget is not on PATH")
	}
	if !strings.Contains(err.Error(), "winget was not found") {
		t.Fatalf("error should mention missing winget, got: %v", err)
	}
	if !strings.Contains(err.Error(), "install winget") {
		t.Fatalf("error should mention installing winget, got: %v", err)
	}
	if !strings.Contains(err.Error(), "python.org/downloads") {
		t.Fatalf("error should include manual Python install URL, got: %v", err)
	}
}

func TestInstallPythonWindows_WingetFailureIncludesRootCause(t *testing.T) {
	dir := t.TempDir()
	createPythonDemoTestCommand(t, dir, "winget", "winget boom", 1)
	t.Setenv("PATH", dir)

	err := installPythonWindows()
	if err == nil {
		t.Fatal("expected error when winget fails and Python is still unavailable")
	}
	if !strings.Contains(err.Error(), "could not install Python 3 via winget") {
		t.Fatalf("error should mention winget install failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "winget boom") {
		t.Fatalf("error should include winget root cause, got: %v", err)
	}
}

func createPythonDemoTestCommand(t *testing.T, dir, name, output string, exitCode int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".bat")
		content := "@echo off\r\necho " + output + "\r\n"
		if exitCode != 0 {
			content += "exit /b 1\r\n"
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatalf("create test command: %v", err)
		}
		return
	}

	path := filepath.Join(dir, name)
	content := "#!/bin/sh\necho " + output + "\n"
	if exitCode != 0 {
		content += "exit 1\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("create test command: %v", err)
	}
}

func TestDemoExamplesURL(t *testing.T) {
	origBase := releaseBaseURL
	releaseBaseURL = "https://example.com/releases"
	defer func() { releaseBaseURL = origBase }()

	origVer := version.Version
	defer func() { version.Version = origVer }()

	origTag := version.SnapshotTag
	defer func() { version.SnapshotTag = origTag }()

	tests := []struct {
		ver         string
		snapshotTag string
		want        string
	}{
		{"1.2.3", "", "https://example.com/releases/download/v1.2.3/dtwiz-examples.tar.gz"},
		{"dev", "", "https://example.com/releases/latest/download/dtwiz-examples.tar.gz"},
		{"1.2.4-next", "", "https://example.com/releases/latest/download/dtwiz-examples.tar.gz"},
		{"1.2.4-next", "snapshot-feat-foo", "https://example.com/releases/download/snapshot-feat-foo/dtwiz-examples.tar.gz"},
	}
	for _, tc := range tests {
		version.Version = tc.ver
		version.SnapshotTag = tc.snapshotTag
		got := demoExamplesURL()
		if got != tc.want {
			t.Errorf("version %q snapshotTag %q: got %q, want %q", tc.ver, tc.snapshotTag, got, tc.want)
		}
	}
}

func TestBundledDemoPath(t *testing.T) {
	path, err := BundledDemoPath()
	if err != nil {
		t.Fatalf("BundledDemoPath returned error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got: %s", path)
	}
	if filepath.Base(path) != demoDirName {
		t.Fatalf("expected path to end with %q, got: %s", demoDirName, path)
	}
}

func TestDownloadDemoExamples(t *testing.T) {
	// Build a fixture tar.gz containing examples/schnitzel/README.md.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("readme")
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "examples/schnitzel/README.md",
		Mode:     0644,
		Size:     int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(buf.Bytes()) //nolint:errcheck
	}))
	defer srv.Close()

	orig := releaseBaseURL
	releaseBaseURL = srv.URL
	defer func() { releaseBaseURL = orig }()

	// dst mimics BundledDemoPath() but inside a temp dir.
	base := t.TempDir()
	dst := filepath.Join(base, "examples", "schnitzel")

	if err := downloadDemoExamples(dst); err != nil {
		t.Fatalf("downloadDemoExamples: %v", err)
	}

	target := filepath.Join(dst, "README.md")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected %s to exist after extraction: %v", target, err)
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("evil")
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "../../evil.txt",
		Mode:     0644,
		Size:     int64(len(content)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(content)
	tw.Close()
	gw.Close()

	dest := t.TempDir()
	err := extractTarGz(&buf, dest)
	if err == nil {
		t.Fatal("expected error for path traversal entry")
	}
	if !strings.Contains(err.Error(), "illegal path") {
		t.Fatalf("error should mention illegal path, got: %v", err)
	}
}

func TestDownloadDemoExamples_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := releaseBaseURL
	releaseBaseURL = srv.URL
	defer func() { releaseBaseURL = orig }()

	dst := filepath.Join(t.TempDir(), "examples", "schnitzel")
	err := downloadDemoExamples(dst)
	if err == nil {
		t.Fatal("expected error for non-200 HTTP response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error should include status code, got: %v", err)
	}
}

func TestInstallPythonWindows_WingetSucceedsButPythonUnavailable(t *testing.T) {
	dir := t.TempDir()
	createPythonDemoTestCommand(t, dir, "winget", "winget install ok", 0)
	t.Setenv("PATH", dir)

	err := installPythonWindows()
	if err == nil {
		t.Fatal("expected error when winget succeeds but Python is still unavailable")
	}
	if !strings.Contains(err.Error(), "could not install Python 3 via winget") {
		t.Fatalf("error should mention failed Python install, got: %v", err)
	}
	// winget exited 0 — the error should not include a winget root cause.
	if strings.Contains(err.Error(), "winget install ok") {
		t.Fatalf("error should not wrap winget output when winget exited 0, got: %v", err)
	}
}

func TestDetectLinuxDistro(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	distro := detectLinuxDistro()
	if distro == "" {
		t.Fatal("expected non-empty distro string")
	}
	if distro != "debian" && distro != "ubuntu" && distro != "rhel" {
		t.Fatalf("unexpected distro value: %s", distro)
	}
}
