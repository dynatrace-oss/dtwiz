package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOtelCollectorInstallDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() failed: %v", err)
	}

	got, err := otelCollectorInstallDir()
	if err != nil {
		t.Fatalf("otelCollectorInstallDir() returned error: %v", err)
	}

	want := filepath.Join(home, "opentelemetry")
	if got != want {
		t.Errorf("otelCollectorInstallDir() = %q, want %q", got, want)
	}
}
