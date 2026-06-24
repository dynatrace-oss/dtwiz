package azure

import (
	"fmt"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// buildUninstallAzRunner returns a runner that handles the 3 az uninstall steps.
func buildUninstallAzRunner(t *testing.T) *fakeAzureRunner {
	t.Helper()
	return &fakeAzureRunner{
		t: t,
		calls: []fakeCall{
			{name: "az", stdout: `{}`}, // federated-credential delete
			{name: "az", stdout: `{}`}, // role assignment delete
			{name: "az", stdout: `{}`}, // sp delete
		},
	}
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

	azCalls := 0
	runner := func(_ string, _ []string, _ []string) (string, error) {
		azCalls++
		return "{}", nil
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", true, runner, happyUninstallFakeDTClient())
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if azCalls != 0 {
		t.Errorf("dry-run: expected 0 az calls, got %d", azCalls)
	}
}

func TestUninstallAzureNothingFound(t *testing.T) {
	azCalls := 0
	runner := func(_ string, _ []string, _ []string) (string, error) {
		azCalls++
		return "{}", nil
	}
	dtc := &fakeDTClient{} // all find methods return empty / not found
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if azCalls != 0 {
		t.Errorf("nothing-found: expected 0 az calls, got %d", azCalls)
	}
}

func TestUninstallAzureNoClientID(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()

	// Connection exists but has no applicationId (e.g. install failed after step 1)
	dtc := &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "",
		findMonConfigID:  "mon-config-001",
	}
	azCalls := 0
	runner := func(_ string, _ []string, _ []string) (string, error) {
		azCalls++
		return "{}", nil
	}
	err := captureStdoutErr(func() error {
		return uninstallAzureWithRunner("https://abc.live.dynatrace.com", false, runner, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if azCalls != 0 {
		t.Errorf("no-client-id: expected 0 az calls (skip SP steps), got %d", azCalls)
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
		t.Fatal("expected error from findConnection failure, got nil")
	}
}
