package installer

import (
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// newTestClassicClient creates a ClassicClient pointing at the given test server URL.
func newTestClassicClient(t *testing.T, serverURL string) *client.ClassicClient {
	t.Helper()
	c, err := client.New(serverURL, "dt0c01.test", serverURL, "dt0s16.test", 0)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	return c.Classic
}

func TestCheckOneAgentConnectivity_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiV1TimePath {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := checkOneAgentConnectivity(newTestClassicClient(t, srv.URL)); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestCheckOneAgentConnectivity_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := checkOneAgentConnectivity(newTestClassicClient(t, srv.URL))
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if want := "invalid credentials"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

func TestCheckOneAgentConnectivity_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	err := checkOneAgentConnectivity(newTestClassicClient(t, srv.URL))
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if want := "400"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain status %q", err.Error(), want)
	}
}

func TestDownloadOneAgentInstaller_WritesContentToDisk(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OneAgent installer download not supported on macOS")
	}
	content := []byte("fake-installer-binary-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content) //nolint:errcheck
	}))
	defer srv.Close()

	path, err := downloadOneAgentInstaller(newTestClassicClient(t, srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("file content = %q, want %q", got, content)
	}
}

func TestDownloadOneAgentInstaller_NonOKStatus(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OneAgent installer download not supported on macOS")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := downloadOneAgentInstaller(newTestClassicClient(t, srv.URL))
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if want := "403"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain status %q", err.Error(), want)
	}
}
