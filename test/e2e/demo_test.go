//go:build integration

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
	"github.com/dynatrace-oss/dtwiz/test/integration"
	"github.com/dynatrace-oss/dtwiz/test/integration/grail"
)

// TestInstallDemo (tasks 8.1 + 8.2): with ~/.dtwiz/examples/schnitzel/ absent,
// verifies that:
//   - the dry-run plan includes a download step referencing the demo path
//   - a real install downloads the release asset, starts the demo services,
//     and delivers traces to the tenant
//
// Requires TEST_DT_ENVIRONMENT and TEST_DT_PLATFORM_TOKEN.
// Skipped when the demo dir already exists or Python is not available.
func TestInstallDemo(t *testing.T) {
	env := integration.SetupIntegration(t)

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	if _, err := exec.LookPath("python3"); err != nil {
		if _, err2 := exec.LookPath("python"); err2 != nil {
			t.Skip("python3/python not found in PATH")
		}
	}

	demoPath, err := otel.BundledDemoPath()
	if err != nil {
		t.Fatalf("BundledDemoPath: %v", err)
	}
	if _, err := os.Stat(demoPath); err == nil {
		t.Skipf("~/.dtwiz/examples/schnitzel already exists (%s); remove it to run this test", demoPath)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(demoPath); err != nil {
			t.Logf("cleanup: remove demo dir %s: %v", demoPath, err)
		}
	})

	// 8.1: dry-run plan must include the download step.
	t.Run("dry-run plan includes download step", func(t *testing.T) {
		output := helpers.CaptureStdout(t, func() {
			_ = otel.InstallDemo(env.EnvURL, env.ClassicToken, env.PlatformToken, true)
		})
		if !strings.Contains(output, "Download schnitzel") {
			t.Fatalf("expected download step in plan, got:\n%s", output)
		}
		if !strings.Contains(output, demoPath) {
			t.Fatalf("expected demo path %q in plan, got:\n%s", demoPath, output)
		}
		if !strings.Contains(output, "dry-run") {
			t.Fatalf("expected dry-run notice in output, got:\n%s", output)
		}
	})

	// 8.2: real install must download the release asset, start the demo services,
	// and deliver traces to the tenant.
	t.Run("installs demo and delivers traces to tenant", func(t *testing.T) {
		installer.AutoConfirm = true
		t.Cleanup(func() { installer.AutoConfirm = false })
		// Stop any lingering OTel processes from previous runs before starting
		// fresh (avoids "Address already in use" on schnitzel's hardcoded ports).
		// AutoConfirm is already true, so UninstallOtelCollector runs without prompts.
		_ = otel.UninstallOtelCollector(false)
		// Register the same cleanup so the processes started by THIS run are
		// stopped after the test, keeping ports free for the next run.
		t.Cleanup(func() {
			installer.AutoConfirm = true
			_ = otel.UninstallOtelCollector(false)
			installer.AutoConfirm = false
		})
		if err := otel.InstallDemo(env.EnvURL, env.ClassicToken, env.PlatformToken, false); err != nil {
			t.Fatalf("InstallDemo: %v", err)
		}

		readme := filepath.Join(demoPath, "README.md")
		if _, err := os.Stat(readme); err != nil {
			t.Fatalf("expected %s to exist after download: %v", readme, err)
		}

		t.Logf("waiting for traces from schnitzel-frontend in Grail")
		traces := grail.RequireTraces(t, env.Client, "schnitzel-frontend",
			grail.WithTimeout(180*time.Second),
			grail.WithInterval(20*time.Second),
		)
		t.Logf("found %d trace(s) for schnitzel-frontend", len(traces))
	})
}

// TestInstallDemo_DryRun_WithDemoDir (task 8.3): with
// ~/.dtwiz/examples/schnitzel/ already present the dry-run plan must omit the
// download step.
func TestInstallDemo_DryRun_WithDemoDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	demoPath, err := otel.BundledDemoPath()
	if err != nil {
		t.Fatalf("BundledDemoPath: %v", err)
	}

	// Create the demo dir if it doesn't already exist; remove it afterwards
	// only if we created it (don't clobber a real installation).
	if _, err := os.Stat(demoPath); os.IsNotExist(err) {
		if err := os.MkdirAll(demoPath, 0755); err != nil {
			t.Fatalf("create demo dir %s: %v", demoPath, err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(demoPath); err != nil {
				t.Logf("cleanup: remove demo dir %s: %v", demoPath, err)
			}
		})
	}

	output := helpers.CaptureStdout(t, func() {
		_ = otel.InstallDemo("https://fake.live.dynatrace.com", "tok", "ptok", true)
	})

	if strings.Contains(output, "Download schnitzel") {
		t.Fatalf("download step must be absent from plan when demo dir exists, got:\n%s", output)
	}
	if !strings.Contains(output, "dry-run") {
		t.Fatalf("expected dry-run notice in output, got:\n%s", output)
	}
}
