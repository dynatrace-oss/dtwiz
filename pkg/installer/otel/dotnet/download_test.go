package dotnet_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func TestDownload_WritesFile(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("zip-bytes")) }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "a.zip")

	// Act
	err := dotnet.Download(context.Background(), srv.Client(), srv.URL, dest)

	// Assert
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "zip-bytes" {
		t.Errorf("file content = %q", got)
	}
}

func TestDownload_HTTPErrorLeavesNoFile(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "a.zip")

	// Act
	err := dotnet.Download(context.Background(), srv.Client(), srv.URL, dest)

	// Assert
	if err == nil {
		t.Error("expected an error for HTTP 404")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("file should not exist, stat err = %v", statErr)
	}
}

func TestDownload_TruncatedResponseLeavesNoFile(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "a.zip")

	// Act
	err := dotnet.Download(context.Background(), srv.Client(), srv.URL, dest)

	// Assert
	if err == nil {
		t.Error("expected an error for a truncated body")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("partial file should be removed, stat err = %v", statErr)
	}
}

func TestDownload_CancelledContextLeavesNoFile(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dest := filepath.Join(t.TempDir(), "a.zip")

	// Act
	err := dotnet.Download(ctx, srv.Client(), srv.URL, dest)

	// Assert
	if err == nil {
		t.Error("expected an error for a cancelled context")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("file should not exist, stat err = %v", statErr)
	}
}
