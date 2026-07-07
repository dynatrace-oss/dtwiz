//go:build integration

package e2e_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/azure"
	"github.com/dynatrace-oss/dtwiz/test/integration"
)

// TestAzureLifecycle exercises the full Azure Monitor integration lifecycle
// against a real Azure subscription and a real Dynatrace tenant: install
// (Entra App Registration + federated credential + role assignment +
// Dynatrace connection/monitoring config) → uninstall (reverses all of it).
//
// It requires the `az` CLI to be logged in (`az login`) to a subscription
// where the signed-in account can create App Registrations and role
// assignments at subscription scope. This test does not attempt to log in or
// validate permissions beyond what the installer itself checks — pointing it
// at an appropriate subscription/tenant is the caller's responsibility.
func TestAzureLifecycle(t *testing.T) {
	if _, err := exec.LookPath("az"); err != nil {
		t.Skip("az CLI not found in PATH")
	}
	if out, err := exec.Command("az", "account", "show", "-o", "json").CombinedOutput(); err != nil {
		t.Fatalf("not logged in to Azure: run `az login` and retry (needs an active session with permission "+
			"to create App Registrations and role assignments at subscription scope)\n%s", out)
	}
	// `az account show` only proves the ARM-scoped token is valid. `az ad ...`
	// commands (used mid-install) need a separate Graph-scoped token, which can
	// expire independently under MFA/conditional-access policies. Catch that
	// here so we fail before creating any resources, not partway through.
	if out, err := exec.Command("az", "ad", "signed-in-user", "show", "-o", "json").CombinedOutput(); err != nil {
		t.Fatalf("Azure Graph session appears stale (MFA/token expired): run `az login` again and retry\n%s", out)
	}

	integration.Parallelize(t)
	env := integration.SetupIntegration(t)
	t.Logf("test ID: %s", env.TestID)

	// Safety-net cleanup: always attempt it, even on a partial failure — install
	// can create real Azure/Dynatrace resources (e.g. the DT connection) before
	// failing on a later step, and UninstallAzure is safe to call when there is
	// nothing (or only partial state) to remove.
	t.Cleanup(func() {
		if err := azure.UninstallAzure(env.EnvURL, env.PlatformToken, false); err != nil {
			t.Logf("cleanup: uninstall failed: %v", err)
		}
	})

	t.Log("installing Azure Monitor integration")
	if err := azure.InstallAzure(env.EnvURL, env.PlatformToken, false, time.Time{}); err != nil {
		t.Fatalf("InstallAzure: %v", err)
	}

	t.Log("verifying Dynatrace connection and monitoring configuration exist")
	requireAzureResources(t, env, true)

	t.Log("uninstalling Azure Monitor integration")
	if err := azure.UninstallAzure(env.EnvURL, env.PlatformToken, false); err != nil {
		t.Fatalf("UninstallAzure: %v", err)
	}

	t.Log("verifying Dynatrace connection and monitoring configuration are gone")
	requireAzureResources(t, env, false)
}

// requireAzureResources fatals/errors unless the Azure connection and
// monitoring configuration existence both match want.
func requireAzureResources(t *testing.T, env *integration.TestEnv, want bool) {
	t.Helper()

	connExists, err := azure.ConnectionExists(env.EnvURL, env.PlatformToken)
	if err != nil {
		t.Fatalf("ConnectionExists: %v", err)
	}
	if connExists != want {
		t.Errorf("azure connection exists = %v, want %v", connExists, want)
	}

	configExists, err := azure.MonitoringConfigExists(env.EnvURL, env.PlatformToken)
	if err != nil {
		t.Fatalf("MonitoringConfigExists: %v", err)
	}
	if configExists != want {
		t.Errorf("azure monitoring configuration exists = %v, want %v", configExists, want)
	}
}
