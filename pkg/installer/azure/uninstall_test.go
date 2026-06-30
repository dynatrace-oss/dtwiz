package azure

import (
	"fmt"
	"strings"
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

func TestUninstallAzureBestEffortDeletesRemaining(t *testing.T) {
	// When monitoring delete fails, role/app/connection deletions must still run.
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	roleDeleted, appDeleted := false, false
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
		findMonConfigID:  "mon-config-001",
		deleteMonErr:     fmt.Errorf("transient monitoring API error"),
	}
	runner := func(_ string, args []string, _ []string) (string, error) {
		switch {
		case isAppList(args):
			return `[]`, nil
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
	if err == nil {
		t.Fatal("expected accumulated error, got nil")
	}
	if !roleDeleted {
		t.Error("role assignment delete must run even when monitoring delete fails")
	}
	if !appDeleted {
		t.Error("app registration delete must run even when monitoring delete fails")
	}
	if !dtc.deleteConnCalled {
		t.Error("connection delete must run even when monitoring delete fails")
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

// ── azureGatherClientIDs direct tests ─────────────────────────────────────────

func TestAzureGatherClientIDs_ConnectionBoundID(t *testing.T) {
	conns := []connRef{{objectID: "obj-001", clientID: "client-001"}}
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[]`, nil
		}
		return "{}", nil
	}
	ids := azureGatherClientIDs(runner, conns, "dtwiz-azure", "https://abc.live.dynatrace.com")
	if len(ids) != 1 || ids[0] != "client-001" {
		t.Errorf("expected [client-001], got %v", ids)
	}
}

func TestAzureGatherClientIDs_OrphanedAppIncluded(t *testing.T) {
	// No connection has a clientID; orphaned App Registration found by name
	// and carries dtwiz's federated credential fingerprint → must be included.
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[{"appId":"orphan-id"}]`, nil
		}
		if isFedCredList(args) {
			return `[{"name":"dtwiz-azure-Federated-Credential","issuer":"https://token.dynatrace.com"}]`, nil
		}
		return "{}", nil
	}
	ids := azureGatherClientIDs(runner, nil, "dtwiz-azure", "https://abc.live.dynatrace.com")
	if len(ids) != 1 || ids[0] != "orphan-id" {
		t.Errorf("expected [orphan-id], got %v", ids)
	}
}

func TestAzureGatherClientIDs_AlreadyTrustedNotDuplicated(t *testing.T) {
	// App found by name is the same clientID already trusted via a connection.
	// It must appear exactly once and must not trigger a fed-cred verification.
	conns := []connRef{{objectID: "obj-001", clientID: "trusted-id"}}
	fedCredCalled := false
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[{"appId":"trusted-id"}]`, nil
		}
		if isFedCredList(args) {
			fedCredCalled = true
			return `[{"name":"dtwiz-azure-Federated-Credential","issuer":"https://token.dynatrace.com"}]`, nil
		}
		return "{}", nil
	}
	ids := azureGatherClientIDs(runner, conns, "dtwiz-azure", "https://abc.live.dynatrace.com")
	if len(ids) != 1 || ids[0] != "trusted-id" {
		t.Errorf("expected exactly [trusted-id] (no duplicates), got %v", ids)
	}
	if fedCredCalled {
		t.Error("federated-credential list should not be called for already-trusted IDs")
	}
}

func TestAzureGatherClientIDs_UnrelatedAppSkipped(t *testing.T) {
	// App found by name lacks dtwiz's federated credential → must be skipped.
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[{"appId":"unrelated-id"}]`, nil
		}
		if isFedCredList(args) {
			return `[{"name":"other-cred","issuer":"https://example.com"}]`, nil
		}
		return "{}", nil
	}
	ids := azureGatherClientIDs(runner, nil, "dtwiz-azure", "https://abc.live.dynatrace.com")
	if len(ids) != 0 {
		t.Errorf("expected empty (unrelated app must be skipped), got %v", ids)
	}
}

func TestAzureGatherClientIDs_AzListFailureContinues(t *testing.T) {
	// az ad app list fails — must not block deletion of connection-bound resources.
	conns := []connRef{{objectID: "obj-001", clientID: "conn-client-id"}}
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return "", fmt.Errorf("az command failed")
		}
		return "{}", nil
	}
	ids := azureGatherClientIDs(runner, conns, "dtwiz-azure", "https://abc.live.dynatrace.com")
	if len(ids) != 1 || ids[0] != "conn-client-id" {
		t.Errorf("expected [conn-client-id] despite az list failure, got %v", ids)
	}
}

func TestAzureGatherClientIDs_VerificationFailureSkipsWithWarning(t *testing.T) {
	// fed-cred list fails → app is skipped with a warning, not silently.
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[{"appId":"unverifiable-id"}]`, nil
		}
		if isFedCredList(args) {
			return "", fmt.Errorf("permission denied")
		}
		return "{}", nil
	}
	var ids []string
	out := captureColorOutput(func() {
		ids = azureGatherClientIDs(runner, nil, "dtwiz-azure", "https://abc.live.dynatrace.com")
	})
	if len(ids) != 0 {
		t.Errorf("expected empty (unverifiable app skipped), got %v", ids)
	}
	if !strings.Contains(out, "Warning") {
		t.Errorf("expected a Warning for unverifiable app, got: %s", out)
	}
}

// ─── connectionExistsWithClient ──────────────────────────────────────────────

func TestConnectionExists_ReturnsTrueWhenConnectionFound(t *testing.T) {
	dtc := &fakeDTClient{findConnObjectID: "conn-obj-001", findConnClientID: "client-id-000"}
	ok, err := connectionExistsWithClient(dtc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true when connection exists, got false")
	}
}

func TestConnectionExists_ReturnsFalseWhenNoConnections(t *testing.T) {
	dtc := &fakeDTClient{} // findAllConnections returns nil
	ok, err := connectionExistsWithClient(dtc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false when no connections found, got true")
	}
}

func TestConnectionExists_PropagatesLookupError(t *testing.T) {
	dtc := &fakeDTClient{findConnErr: fmt.Errorf("api unavailable")}
	_, err := connectionExistsWithClient(dtc)
	if err == nil {
		t.Fatal("expected error from failing lookup, got nil")
	}
	if !strings.Contains(err.Error(), "api unavailable") {
		t.Errorf("error %q does not contain original message", err.Error())
	}
}

func TestConnectionExists_ReturnsTrueForMultipleConnections(t *testing.T) {
	dtc := &fakeDTClient{findConnRefs: []connRef{
		{objectID: "conn-obj-001", clientID: "client-id-001"},
		{objectID: "conn-obj-002", clientID: "client-id-002"},
	}}
	ok, err := connectionExistsWithClient(dtc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for multiple connections, got false")
	}
}

func TestAzureGatherClientIDs_Sorted(t *testing.T) {
	// Output must be deterministically sorted regardless of map iteration order.
	conns := []connRef{
		{objectID: "obj-002", clientID: "zzz-id"},
		{objectID: "obj-001", clientID: "aaa-id"},
	}
	runner := func(_ string, args []string, _ []string) (string, error) {
		if isAppList(args) {
			return `[]`, nil
		}
		return "{}", nil
	}
	ids := azureGatherClientIDs(runner, conns, "dtwiz-azure", "https://abc.live.dynatrace.com")
	if len(ids) != 2 || ids[0] != "aaa-id" || ids[1] != "zzz-id" {
		t.Errorf("expected sorted [aaa-id, zzz-id], got %v", ids)
	}
}
