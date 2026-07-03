package azure

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// updateAzRunner serves the only az call the in-place update makes: account show.
// Any other az invocation is a test failure (update must not mutate Azure).
func updateAzRunner(t *testing.T) cmdRunner {
	t.Helper()
	return func(name string, args []string, _ []string) (string, error) {
		if name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show" {
			return stockAccountJSON, nil
		}
		t.Errorf("unexpected az call during in-place update: %v", args)
		return "", fmt.Errorf("unexpected az call: %v", args)
	}
}

func TestUpdateAzureHappyPath_UpdatesInPlace(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient() // conn with clientID + one monitoring config

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "dt0s16.fake.token", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The existing config is reconciled in place; nothing is created or deleted.
	if got := dtc.updateMonConfigIDs; len(got) != 1 || got[0] != "mon-config-001" {
		t.Errorf("expected updateMonitoring on [mon-config-001], got %v", got)
	}
	if dtc.createMonCalled {
		t.Error("createMonitoring must not be called when a config already exists")
	}
	if dtc.deleteConnCalled {
		t.Error("update must not delete the connection (auth chain is untouched)")
	}
	if dtc.monCalledWith.connObjectID != "conn-obj-001" || dtc.monCalledWith.clientID != "client-id-000" {
		t.Errorf("monitoring config bound to wrong connection/client: %+v", dtc.monCalledWith)
	}
	if dtc.monCalledWith.subscriptionID != "sub-abc123" {
		t.Errorf("expected subscription from az account show, got %q", dtc.monCalledWith.subscriptionID)
	}
}

func TestUpdateAzureCreatesConfigWhenMissing(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	// Connection exists (with clientID) but no monitoring configuration.
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dtc.createMonCalled {
		t.Error("expected createMonitoring when no configuration exists")
	}
	if len(dtc.updateMonConfigIDs) != 0 {
		t.Errorf("expected no updateMonitoring calls, got %v", dtc.updateMonConfigIDs)
	}
}

func TestUpdateAzureReconcilesAllDuplicateConfigs(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
		findMonConfigIDs: []string{"mon-1", "mon-2"},
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dtc.updateMonConfigIDs) != 2 {
		t.Errorf("expected both duplicate configs reconciled, got %v", dtc.updateMonConfigIDs)
	}
}

func TestUpdateAzureDryRun(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()

	out := captureStdout(t, func() {
		if err := updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", true, time.Time{}, updateAzRunner(t), dtc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if len(dtc.updateMonConfigIDs) != 0 || dtc.createMonCalled {
		t.Error("dry-run must not mutate any monitoring configuration")
	}
	if !strings.Contains(out, "[dry-run] No changes were made.") {
		t.Errorf("expected dry-run notice; got:\n%s", out)
	}
}

func TestUpdateAzurePreviewLeavesAuthUnchanged(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()
	out := captureStdout(t, func() {
		_ = updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", true, time.Time{}, updateAzRunner(t), dtc)
	})

	if !strings.Contains(out, "update Azure monitoring configuration") {
		t.Errorf("expected an update step in preview; got:\n%s", out)
	}
	if !strings.Contains(out, "(unchanged)") {
		t.Errorf("expected preview to mark the connection as unchanged; got:\n%s", out)
	}
}

func TestUpdateAzureCancelled(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()
	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if !isErrInstallCancelled(err) {
		t.Errorf("expected ErrInstallCancelled, got: %v", err)
	}
	if len(dtc.updateMonConfigIDs) != 0 || dtc.createMonCalled {
		t.Error("declining must not mutate anything")
	}
}

func TestUpdateAzureNoUsableConnection(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	// Connection found but missing its bound application ID (incomplete install).
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "",
		findMonConfigID:  "mon-config-001",
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err == nil {
		t.Fatal("expected error when no complete connection exists, got nil")
	}
	if !strings.Contains(err.Error(), "install azure") {
		t.Errorf("expected guidance to run install, got: %v", err)
	}
}

func TestUpdateAzureMultipleConnections(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{
		findConnRefs: []connRef{
			{objectID: "conn-1", clientID: "client-1"},
			{objectID: "conn-2", clientID: "client-2"},
		},
		findMonConfigID: "mon-config-001",
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err == nil {
		t.Fatal("expected error for ambiguous multiple connections, got nil")
	}
	if !strings.Contains(err.Error(), "uninstall azure") {
		t.Errorf("expected guidance to uninstall+install, got: %v", err)
	}
}

func TestUpdateAzureFindMonitoringError(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{findMonErr: fmt.Errorf("monitoring API error")}
	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err == nil {
		t.Fatal("expected error from findMonitoringConfig failure, got nil")
	}
}

func TestUpdateAzureFindConnectionError(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{findConnErr: fmt.Errorf("connection API error")}
	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err == nil {
		t.Fatal("expected error from findConnection failure, got nil")
	}
}

func TestUpdateAzureAccountInfoFails(t *testing.T) {
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		if name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show" {
			return "", fmt.Errorf("az login required")
		}
		return "{}", nil
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, happyUninstallFakeDTClient())
	})
	if err == nil {
		t.Fatal("expected error from account lookup failure, got nil")
	}
	if !strings.Contains(err.Error(), "az login") {
		t.Errorf("expected az login hint, got: %v", err)
	}
}

func TestUpdateAzureMonitoringUpdateFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()
	dtc.updateMonErr = fmt.Errorf("update monitoring: API error")

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err == nil {
		t.Fatal("expected error from monitoring update failure, got nil")
	}
	if dtc.deleteConnCalled {
		t.Error("a failed config update must not have touched the connection")
	}
}

func TestUpdateAzureMonitoringCreateFails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	// Connection exists but no monitoring config; createMonitoring will fail.
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
		monErr:           fmt.Errorf("extensions API: 403 Forbidden"),
	}

	err := captureStdoutErr(func() error {
		return updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, updateAzRunner(t), dtc)
	})
	if err == nil {
		t.Fatal("expected error from createMonitoring failure, got nil")
	}
	if !strings.Contains(err.Error(), "create monitoring configuration") {
		t.Errorf("expected wrapped create error, got: %v", err)
	}
	if len(dtc.updateMonConfigIDs) != 0 {
		t.Error("updateMonitoring must not be called when there is nothing to update")
	}
}

func TestUpdateAzurePreviewShowsCreateStep(t *testing.T) {
	defer stubExecLookPath(t)()

	// No monitoring config → preview must describe a create, not an update.
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
	}
	out := captureStdout(t, func() {
		_ = updateAzureWithRunner("https://abc.live.dynatrace.com", "tok", true, time.Time{}, updateAzRunner(t), dtc)
	})

	if !strings.Contains(out, "create Azure monitoring configuration") {
		t.Errorf("expected create step in preview; got:\n%s", out)
	}
	if strings.Contains(out, "update Azure monitoring configuration") {
		t.Errorf("preview must not show an update step when no config exists; got:\n%s", out)
	}
}

func TestUpdateAzureEntryPoint_ClientInitError(t *testing.T) {
	// An empty envURL causes httpclient.New to return "base URL is required".
	err := UpdateAzure("", "dt0s16.fake.token", false, time.Time{})
	if err == nil {
		t.Fatal("expected error for empty envURL, got nil")
	}
}

func isErrInstallCancelled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "install cancelled") ||
		err == installer.ErrInstallCancelled
}
