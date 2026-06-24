package azure

import (
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
			// preflight
			{name: "az", stdout: stockAccountJSON},
			{name: "az", stdout: stockMgmtGroupJSON},
			{name: "az", stdout: stockRBACJSON},
			// uninstall phase: fedcred delete, role delete, sp delete
			{name: "az", stdout: `{}`},
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
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
