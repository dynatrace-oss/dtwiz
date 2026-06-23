//go:build integration

package e2e_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/oneagent"
	"github.com/dynatrace-oss/dtwiz/test/integration"
	"github.com/dynatrace-oss/dtwiz/test/integration/grail"
)

// oneAgentInstallDir returns OneAgent's well-known install location for the
// current OS. The package-internal variable of the same name is not exported,
// so the e2e test asserts against the literal path — which is where a real
// install lands.
func oneAgentInstallDir() string {
	if runtime.GOOS == "windows" {
		pf := os.Getenv("ProgramFiles")
		if pf == "" {
			pf = `C:\Program Files`
		}
		return filepath.Join(pf, "dynatrace", "oneagent")
	}
	return "/opt/dynatrace/oneagent"
}

// TestOneAgentLifecycle exercises the full OneAgent V2 lifecycle against a real
// Dynatrace tenant: install → the host registers in Smartscape topology (Grail)
// → uninstall → install dir removed.
//
// It mirrors TestOTelAutoInstrumentation but, because OneAgent installs
// system-wide and there is exactly one agent per host, it runs as a single
// serial case rather than a parallel table, and gates on elevated privileges.
// On Linux it requires root (non-root triggers an interactive sudo prompt that
// hangs CI). On Windows the installer exe handles UAC elevation itself via its
// embedded manifest — no admin pre-check is needed.
func TestOneAgentLifecycle(t *testing.T) {
	switch runtime.GOOS {
	case "linux", "windows":
		// supported
	default:
		t.Skipf("OneAgent install is not supported on %s", runtime.GOOS)
	}
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		t.Skip("OneAgent install requires root; re-run as root (interactive sudo would hang)")
	}

	installDir := oneAgentInstallDir()

	env := integration.SetupIntegration(t)
	t.Logf("test ID: %s", env.TestID)

	originalAutoConfirm := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = originalAutoConfirm })

	// Always attempt to remove the agent, even if a later assertion fails, so the
	// runner is never left permanently monitored. The explicit uninstall below is
	// still the asserted path; this is a safety net for early failures.
	t.Cleanup(func() {
		if _, err := os.Stat(installDir); err != nil {
			return // already gone (or never installed)
		}
		if uerr := oneagent.UninstallOneAgentV2(oneagent.UninstallOptions{}); uerr != nil {
			t.Logf("cleanup: uninstall failed: %v", uerr)
		}
	})

	t.Log("installing OneAgent (fullstack)")
	if err := oneagent.InstallOneAgentV2(env.Client, oneagent.InstallOptions{
		HostGroup: env.TestID, // unique tag to aid correlation and manual debugging
		Quiet:     true,
	}); err != nil {
		t.Fatalf("InstallOneAgentV2: %v", err)
	}
	if _, err := os.Stat(installDir); err != nil {
		t.Fatalf("expected install dir %s to exist after install: %v", installDir, err)
	}

	hostName, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname: %v", err)
	}

	t.Logf("waiting for host %q to register in topology", hostName)
	hosts := grail.RequireHost(t, env.Client, hostName,
		grail.WithTimeout(5*time.Minute),
		grail.WithInterval(20*time.Second),
	)
	t.Logf("found %d host record(s) for %q", len(hosts), hostName)

	t.Log("uninstalling OneAgent")
	if err := oneagent.UninstallOneAgentV2(oneagent.UninstallOptions{}); err != nil {
		t.Fatalf("UninstallOneAgentV2: %v", err)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Errorf("expected install dir %s to be gone after uninstall, stat err = %v", installDir, err)
	}
}

