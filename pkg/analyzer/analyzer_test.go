package analyzer_test

import (
	"runtime"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
)

func TestAnalyzeSystem_ReturnsPlatform(t *testing.T) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		t.Fatalf("AnalyzeSystem() returned error: %v", err)
	}

	switch runtime.GOOS {
	case "linux":
		if info.Platform != analyzer.PlatformLinux {
			t.Errorf("expected platform %q, got %q", analyzer.PlatformLinux, info.Platform)
		}
	case "darwin":
		if info.Platform != analyzer.PlatformDarwin {
			t.Errorf("expected platform %q, got %q", analyzer.PlatformDarwin, info.Platform)
		}
	case "windows":
		if info.Platform != analyzer.PlatformWindows {
			t.Errorf("expected platform %q, got %q", analyzer.PlatformWindows, info.Platform)
		}
	}
}

func TestAnalyzeSystem_ReturnsArch(t *testing.T) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		t.Fatalf("AnalyzeSystem() returned error: %v", err)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("expected arch %q, got %q", runtime.GOARCH, info.Arch)
	}
}

func TestAnalyzeSystem_SummaryNotEmpty(t *testing.T) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		t.Fatalf("AnalyzeSystem() returned error: %v", err)
	}
	s := info.Summary()
	if s == "" {
		t.Error("Summary() returned empty string")
	}
}
