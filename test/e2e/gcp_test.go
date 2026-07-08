//go:build integration

package e2e_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/gcp"
	"github.com/dynatrace-oss/dtwiz/test/integration"
)

// TestGCPLifecycle exercises the full Google Cloud integration lifecycle
// against a real GCP project and a real Dynatrace tenant: install
// (service account + IAM bindings + Dynatrace connection/monitoring config)
// → uninstall (reverses all of it).
//
// It requires the `gcloud` CLI to be on PATH with an active project set.
// The signed-in account must be able to create service accounts and manage
// IAM policy bindings at the project level.
func TestGCPLifecycle(t *testing.T) {
	if _, err := exec.LookPath("gcloud"); err != nil {
		t.Skip("gcloud not found in PATH")
	}
	out, err := exec.Command("gcloud", "config", "get-value", "project").CombinedOutput()
	project := strings.TrimSpace(string(out))
	if err != nil || project == "" || strings.Contains(project, "(unset)") {
		t.Fatalf("no active Google Cloud project: run `gcloud auth login` and `gcloud config set project <id>`, then retry\n%s", out)
	}

	integration.Parallelize(t)
	env := integration.SetupIntegration(t)
	t.Logf("test ID: %s", env.TestID)
	t.Logf("target project: %s", project)

	// Safety-net cleanup: always attempt it, even on a partial failure — install
	// can create real GCP/Dynatrace resources before failing on a later step, and
	// UninstallGCP is safe to call when there is nothing (or only partial state) to remove.
	t.Cleanup(func() {
		if err := gcp.UninstallGCP(env.EnvURL, env.PlatformToken, false); err != nil {
			t.Logf("cleanup: uninstall failed: %v", err)
		}
	})

	t.Log("installing Google Cloud integration")
	if err := gcp.InstallGCP(env.EnvURL, env.PlatformToken, false, time.Time{}); err != nil {
		t.Fatalf("InstallGCP: %v", err)
	}

	t.Log("verifying Dynatrace connection and monitoring configuration exist")
	requireGCPResources(t, env, true)

	t.Log("uninstalling Google Cloud integration")
	if err := gcp.UninstallGCP(env.EnvURL, env.PlatformToken, false); err != nil {
		t.Fatalf("UninstallGCP: %v", err)
	}

	t.Log("verifying Dynatrace connection and monitoring configuration are gone")
	requireGCPResources(t, env, false)
}

// requireGCPResources fatals/errors unless the GCP connection and
// monitoring configuration existence both match want.
func requireGCPResources(t *testing.T, env *integration.TestEnv, want bool) {
	t.Helper()

	connExists, err := gcp.ConnectionExists(env.EnvURL, env.PlatformToken)
	if err != nil {
		t.Fatalf("ConnectionExists: %v", err)
	}
	if connExists != want {
		t.Errorf("GCP connection exists = %v, want %v", connExists, want)
	}

	configExists, err := gcp.MonitoringConfigExists(env.EnvURL, env.PlatformToken)
	if err != nil {
		t.Fatalf("MonitoringConfigExists: %v", err)
	}
	if configExists != want {
		t.Errorf("GCP monitoring configuration exists = %v, want %v", configExists, want)
	}
}
