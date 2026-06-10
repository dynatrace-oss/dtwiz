package oneagent

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withRootCertURL overrides the package-level CA URL for the duration of the
// test, restoring it on cleanup.
func withRootCertURL(t *testing.T, url string) {
	t.Helper()
	orig := dtRootCertURL
	dtRootCertURL = url
	t.Cleanup(func() { dtRootCertURL = orig })
}

func TestVerifyInstallerSignature_SkipFlag(t *testing.T) {
	if err := VerifyInstallerSignature(Environment{OS: "linux"}, "/nonexistent", true); err != nil {
		t.Errorf("skip=true should return nil, got %v", err)
	}
}

func TestVerifyInstallerSignature_NonLinux(t *testing.T) {
	cases := []string{"darwin", "windows", ""}
	for _, os := range cases {
		if err := VerifyInstallerSignature(Environment{OS: os}, "/nonexistent", false); err != nil {
			t.Errorf("os=%q should return nil, got %v", os, err)
		}
	}
}

func TestVerifyInstallerSignature_MissingOpenSSL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH-based openssl shadowing test relies on Unix semantics")
	}
	t.Setenv("PATH", t.TempDir())

	err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, "/nonexistent", false)
	if err == nil {
		t.Fatal("expected missing-openssl error")
	}
	if !strings.Contains(err.Error(), "openssl is required") {
		t.Errorf("error = %q, want missing-openssl message", err)
	}
	if !strings.Contains(err.Error(), "--no-verify-signature") {
		t.Errorf("error = %q, want --no-verify-signature hint", err)
	}
}

func TestVerifyInstallerSignature_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	createStubFile(t, filepath.Join(binDir, "openssl"), "#!/bin/sh\ncat >/dev/null\nexit 0\n", 0o755)
	t.Setenv("PATH", binDir)

	installer := filepath.Join(t.TempDir(), "fake-installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	caSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PEM"))
	}))
	defer caSrv.Close()
	withRootCertURL(t, caSrv.URL)

	if err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, installer, false); err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestVerifyInstallerSignature_FailureWrapsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	createStubFile(t, filepath.Join(binDir, "openssl"),
		"#!/bin/sh\ncat >/dev/null\necho 'Verification failure' >&2\nexit 4\n", 0o755)
	t.Setenv("PATH", binDir)

	installer := filepath.Join(t.TempDir(), "fake-installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	caSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PEM"))
	}))
	defer caSrv.Close()
	withRootCertURL(t, caSrv.URL)

	err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, installer, false)
	if err == nil {
		t.Fatal("expected verification failure")
	}
	if !strings.Contains(err.Error(), "Verification failure") {
		t.Errorf("error = %q, missing openssl stderr", err)
	}
	if !strings.Contains(err.Error(), "exit 4") {
		t.Errorf("error = %q, missing exit code", err)
	}
}

func TestVerifyInstallerSignature_CADownloadFailsAborts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	// openssl should never be invoked because CA fetch fails first.
	createStubFile(t, filepath.Join(binDir, "openssl"),
		"#!/bin/sh\necho 'should not run' >&2\nexit 99\n", 0o755)
	t.Setenv("PATH", binDir)

	caSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer caSrv.Close()
	withRootCertURL(t, caSrv.URL)

	installer := filepath.Join(t.TempDir(), "fake-installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, installer, false)
	if err == nil {
		t.Fatal("expected CA download failure")
	}
	if !strings.Contains(err.Error(), "Dynatrace root CA") {
		t.Errorf("error = %q, want CA download error", err)
	}
}

func TestFetchDynatraceRootCA_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PEM-CONTENT"))
	}))
	defer srv.Close()

	path, err := fetchDynatraceRootCA(srv.URL)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	defer os.Remove(path)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read CA file: %v", err)
	}
	if string(content) != "PEM-CONTENT" {
		t.Errorf("content = %q, want PEM-CONTENT", content)
	}
}

func TestFetchDynatraceRootCA_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchDynatraceRootCA(srv.URL)
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "Dynatrace root CA") {
		t.Errorf("error = %q, want CA error message", err)
	}
}

func TestFetchDynatraceRootCA_NetworkError(t *testing.T) {
	// Close the server before the request so resty gets a connection-refused error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := fetchDynatraceRootCA(url)
	if err == nil {
		t.Fatal("expected error on network failure")
	}
	if !strings.Contains(err.Error(), "Dynatrace root CA") {
		t.Errorf("error = %q, want CA error message", err)
	}
}

func TestRunOpensslVerify_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	opensslPath := filepath.Join(binDir, "openssl")
	createStubFile(t, opensslPath, "#!/bin/sh\ncat >/dev/null\nexit 0\n", 0o755)

	installer := filepath.Join(t.TempDir(), "installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	cert := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(cert, []byte("PEM"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stderr, err := runOpensslVerify(opensslPath, installer, cert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
}

func TestRunOpensslVerify_BinaryMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses Unix paths")
	}
	installer := filepath.Join(t.TempDir(), "installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	cert := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(cert, []byte("PEM"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A non-existent binary causes cmd.Run() to return a non-ExitError (PathError).
	code, _, err := runOpensslVerify("/nonexistent/openssl-dtwiz-test-sentinel", installer, cert)
	if err == nil {
		t.Fatal("expected error when binary is missing")
	}
	if code != 0 {
		t.Errorf("expected code 0 on process-start failure, got %d", code)
	}
}

func TestRunOpensslVerify_ExitError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	opensslPath := filepath.Join(binDir, "openssl")
	createStubFile(t, opensslPath, "#!/bin/sh\ncat >/dev/null\necho 'bad sig' >&2\nexit 7\n", 0o755)

	installer := filepath.Join(t.TempDir(), "installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	cert := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(cert, []byte("PEM"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stderr, err := runOpensslVerify(opensslPath, installer, cert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
	if !strings.Contains(stderr, "bad sig") {
		t.Errorf("stderr = %q, want 'bad sig'", stderr)
	}
}
