package azure

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

func buildUpdateAzRunner(t *testing.T) *fakeAzureRunner {
	t.Helper()
	return &fakeAzureRunner{
		t: t,
		calls: []fakeCall{
			// preflight: account show, signed-in-user (parse fails → RBAC skipped)
			{name: "az", stdout: stockAccountJSON},
			{name: "az", stdout: stockRBACJSON},
			// cleanup lookup: az ad app list (no extra apps)
			{name: "az", stdout: `[]`},
			// uninstall phase: role delete, app delete
			{name: "az", stdout: `{}`},
			{name: "az", stdout: `{}`},
			// install phase: sp create, fedcred create, sp show, role create
			{name: "az", stdout: stockSPJSON},
			{name: "az", stdout: `{}`},
			{name: "az", stdout: stockSPShowJSON},
			{name: "az", stdout: `{}`},
		},
	}
}

func TestUpdateAzureHappyPath(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()
	dtc.connObjectID = "new-conn-obj-001"
	fr := buildUpdateAzRunner(t)

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "dt0s16.fake.token", false, time.Time{}, fr.run, noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.idx != len(fr.calls) {
		t.Errorf("expected %d az calls, got %d", len(fr.calls), fr.idx)
	}
}

func TestUpdateAzureDryRun(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	mutating := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user":
			// read-only preflight call — not a mutation
			return `{"id":"user-object-id"}`, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		case name == "az" && isAppList(args):
			// read-only cleanup lookup — not a mutation
			return `[]`, nil
		default:
			mutating++
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", true, time.Time{}, runner, noSleep, happyUninstallFakeDTClient())
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mutating != 0 {
		t.Errorf("dry-run: expected 0 mutating az calls, got %d", mutating)
	}
}

func TestUpdateAzureCancelled(t *testing.T) {
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyUninstallFakeDTClient())
	})
	if !isErrInstallCancelled(err) {
		t.Errorf("expected ErrInstallCancelled, got: %v", err)
	}
}

func TestUpdateAzurePreviewShowsBothPhases(t *testing.T) {
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			return "{}", nil
		}
	}

	out := captureStdout(t, func() {
		_ = updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", true, time.Time{}, runner, noSleep, happyUninstallFakeDTClient())
	})

	// Phase 1 steps are printed with fmt.Printf (step descriptions from azureUninstallBuildSteps)
	if !strings.Contains(out, "delete monitoring configuration") {
		t.Errorf("expected uninstall step in preview output; got:\n%s", out)
	}
	// Phase 2 steps are printed with fmt.Printf (step descriptions from azureBuildStepCommands)
	if !strings.Contains(out, "create Azure connection") {
		t.Errorf("expected install step in preview output; got:\n%s", out)
	}
}

func isErrInstallCancelled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "install cancelled") ||
		err == installer.ErrInstallCancelled
}

func minimalAzRunner(name string, args []string, _ []string) (string, error) {
	if name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show" {
		return stockAccountJSON, nil
	}
	return "{}", nil
}

func TestUpdateAzureFindMonitoringError(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{findMonErr: fmt.Errorf("monitoring API error")}
	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, minimalAzRunner, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from findMonitoringConfig failure, got nil")
	}
}

func TestUpdateAzureFindConnectionError(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{findConnErr: fmt.Errorf("connection API error")}
	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, minimalAzRunner, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from findConnection failure, got nil")
	}
}

func TestUpdateAzurePreflightFails(t *testing.T) {
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		if name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show" {
			return "", fmt.Errorf("az login required")
		}
		return "{}", nil
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyUninstallFakeDTClient())
	})
	if err == nil {
		t.Fatal("expected error from preflight failure, got nil")
	}
	if !strings.Contains(err.Error(), "az login") {
		t.Errorf("expected az login hint, got: %v", err)
	}
}

func TestUpdateAzureUninstallPhaseFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()
	dtc.deleteMonErr = fmt.Errorf("delete monitoring: API error")

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from uninstall phase failure, got nil")
	}
	if !strings.Contains(err.Error(), "uninstall phase") {
		t.Errorf("expected 'uninstall phase' in error, got: %v", err)
	}
}

func TestUpdateAzureInstallPhaseFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()
	// connObjectID is used as the return value of createConnection; set connErr to fail step 1 of install.
	dtc.connErr = fmt.Errorf("create connection: API error")
	dtc.connObjectID = ""

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			// All az calls succeed — uninstall phase uses runner for fedcred/role/sp deletions.
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from install phase failure, got nil")
	}
	if !strings.Contains(err.Error(), "install phase") {
		t.Errorf("expected 'install phase' in error, got: %v", err)
	}
}

func TestUpdateAzureEmptyClientIDLookupByName(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	// Connection exists but has no stored clientID (install failed before step 6).
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "",
		findMonConfigID:  "mon-config-001",
		connObjectID:     "new-conn-obj-001",
	}

	fr := &fakeAzureRunner{
		t: t,
		calls: []fakeCall{
			{name: "az", stdout: stockAccountJSON},             // preflight: account show
			{name: "az", stdout: stockRBACJSON},                // preflight: signed-in-user (parse fails)
			{name: "az", stdout: `[{"appId":"found-app-id"}]`}, // cleanup: az ad app list fallback
			// cleanup: verify ownership fingerprint of the name-only app
			{name: "az", stdout: `[{"name":"dtwiz-azure-Federated-Credential","issuer":"https://token.dynatrace.com"}]`},
			{name: "az", stdout: `{}`},            // uninstall: role delete
			{name: "az", stdout: `{}`},            // uninstall: app delete
			{name: "az", stdout: stockSPJSON},     // install: sp create
			{name: "az", stdout: `{}`},            // install: fedcred create
			{name: "az", stdout: stockSPShowJSON}, // install: sp show
			{name: "az", stdout: `{}`},            // install: role create
		},
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, fr.run, noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.idx != len(fr.calls) {
		t.Errorf("expected %d az calls, got %d", len(fr.calls), fr.idx)
	}
}
