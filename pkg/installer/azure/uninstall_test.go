package azure

import (
	"fmt"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// buildUninstallAzRunner returns a runner for the uninstall az calls:
// the app-list lookup, then the per-app role-assignment delete and app delete.
func buildUninstallAzRunner(t *testing.T) *fakeAzureRunner {
	t.Helper()
	return &fakeAzureRunner{
		t: t,
		calls: []fakeCall{
			{name: "az", stdout: `[]`}, // azureGatherClientIDs: az ad app list (no extra apps)
			{name: "az", stdout: `{}`}, // role assignment delete
			{name: "az", stdout: `{}`}, // app registration delete
		},
	}
}

// isAppList reports whether the az args are an `ad app list` lookup (read-only).
func isAppList(args []string) bool {
	return len(args) >= 3 && args[0] == "ad" && args[1] == "app" && args[2] == "list"
}

// isFedCredList reports whether the az args are an `ad app federated-credential list`
// lookup (read-only ownership-fingerprint check).
func isFedCredList(args []string) bool {
	return len(args) >= 4 && args[0] == "ad" && args[1] == "app" &&
		args[2] == "federated-credential" && args[3] == "list"
}

func TestUninstallAzureHappyPath(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	dtc := happyUninstallFakeDTClient()
	fr := buildUninstallAzRunner(t)
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, fr.run, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.idx != len(fr.calls) {
		t.Errorf("expected %d az calls, got %d", len(fr.calls), fr.idx)
	}
}

func TestUninstallAzureDryRun(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	mutating := 0
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[]`, nil
		}
		mutating++
		return "{}", nil
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", true, runner, happyUninstallFakeDTClient())
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mutating != 0 {
		t.Errorf("dry-run: expected 0 mutating az calls, got %d", mutating)
	}
}

func TestUninstallAzureNothingFound(t *testing.T) {
	mutating := 0
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[]`, nil
		}
		mutating++
		return "{}", nil
	}
	dtc := &fakeDTClient{} // all find methods return empty / not found
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mutating != 0 {
		t.Errorf("nothing-found: expected 0 mutating az calls, got %d", mutating)
	}
}

func TestUninstallAzureNoClientID(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	// Connection exists but has no applicationId, and no app of that name remains.
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "",
		findMonConfigID:  "mon-config-001",
	}
	mutating := 0
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[]`, nil
		}
		mutating++
		return "{}", nil
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mutating != 0 {
		t.Errorf("no-client-id: expected 0 mutating az calls (no app to clean), got %d", mutating)
	}
}

func TestUninstallAzureOrphanedAppCleaned(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	// DT connection has no stored clientID, but an orphaned App Registration of the
	// same name lingers — it must still be discovered and deleted.
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "",
	}
	roleDeleted, appDeleted := false, false
	runner := func(_ string, args []string, _ []string) (string, error) {
		switch {
		case isAppList(args):
			return `[{"appId":"orphan-app-id"}]`, nil
		case isFedCredList(args):
			// orphan carries dtwiz's fingerprint → safe to delete
			return `[{"name":"dtwiz-azure-Federated-Credential","issuer":"https://token.dynatrace.com"}]`, nil
		case len(args) > 2 && args[0] == "role" && args[1] == "assignment" && args[2] == "delete":
			roleDeleted = true
			return `{}`, nil
		case len(args) > 2 && args[0] == "ad" && args[1] == "app" && args[2] == "delete":
			appDeleted = true
			return `{}`, nil
		default:
			return `{}`, nil
		}
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !roleDeleted {
		t.Error("expected role assignment delete for orphaned app")
	}
	if !appDeleted {
		t.Error("expected app registration delete for orphaned app")
	}
}

func TestUninstallAzureUnrelatedAppNotDeleted(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	// An app of the same display name exists but was NOT created by dtwiz
	// (lacks the dtwiz federated credential) — it must be left untouched.
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "",
	}
	appDeleted := false
	runner := func(_ string, args []string, _ []string) (string, error) {
		switch {
		case isAppList(args):
			return `[{"appId":"someone-elses-app"}]`, nil
		case isFedCredList(args):
			return `[{"name":"unrelated-cred","issuer":"https://example.com"}]`, nil
		case len(args) > 2 && args[0] == "ad" && args[1] == "app" && args[2] == "delete":
			appDeleted = true
			return `{}`, nil
		default:
			return `{}`, nil
		}
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appDeleted {
		t.Error("must not delete an App Registration that lacks the dtwiz fingerprint")
	}
}

func TestUninstallAzureFindConnectionError(t *testing.T) {
	dtc := &fakeDTClient{
		findConnErr:     fmt.Errorf("API error"),
		findMonConfigID: "mon-config-001",
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, nil, dtc)
	})
	if err == nil {
		t.Fatal("expected error from findAllConnections failure, got nil")
	}
}

func TestUninstallAzureFindMonitoringError(t *testing.T) {
	dtc := &fakeDTClient{findMonErr: fmt.Errorf("monitoring API error")}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, nil, dtc)
	})
	if err == nil {
		t.Fatal("expected error from findAllMonitoringConfigs failure, got nil")
	}
}

func TestUninstallAzureDeleteMonitoringFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
		findMonConfigID:  "mon-config-001",
		deleteMonErr:     fmt.Errorf("delete monitoring: API error"),
	}
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[]`, nil
		}
		return "{}", nil
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, dtc)
	})
	if err == nil {
		t.Fatal("expected error from deleteMonitoring failure, got nil")
	}
}

func TestUninstallAzureRoleDeleteFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	runner := func(_ string, args []string, _ []string) (string, error) {
		switch {
		case isAppList(args):
			return `[]`, nil
		case len(args) > 1 && args[0] == "role" && args[1] == "assignment":
			return "", fmt.Errorf("insufficient permissions to delete role assignment")
		default:
			return "{}", nil
		}
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, happyUninstallFakeDTClient())
	})
	if err == nil {
		t.Fatal("expected error from role assignment delete failure, got nil")
	}
}

func TestUninstallAzureAppDeleteFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	runner := func(_ string, args []string, _ []string) (string, error) {
		switch {
		case isAppList(args):
			return `[]`, nil
		case len(args) > 2 && args[0] == "ad" && args[1] == "app" && args[2] == "delete":
			return "", fmt.Errorf("app delete failed: not authorized")
		default:
			return "{}", nil
		}
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, happyUninstallFakeDTClient())
	})
	if err == nil {
		t.Fatal("expected error from app registration delete failure, got nil")
	}
}

func TestUninstallAzureDeleteConnectionFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
		findMonConfigID:  "mon-config-001",
		deleteConnErr:    fmt.Errorf("delete connection: API error"),
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, buildUninstallAzRunner(t).run, dtc)
	})
	if err == nil {
		t.Fatal("expected error from deleteConnection failure, got nil")
	}
}
