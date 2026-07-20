package gcp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

func TestGCPUpdateExistingConfig(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{
		findConnRefs:    []connRef{{objectID: "conn-1", serviceAccountEmail: "dtwiz-gcp@my-project.iam.gserviceaccount.com"}},
		findMonConfigID: "mon-1",
	}
	err := captureStdoutErr(func() error {
		return updateGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, uninstallGcloudRunner("my-project"), dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dtc.updateMonConfigIDs) != 1 || dtc.updateMonConfigIDs[0] != "mon-1" {
		t.Errorf("expected updateMonitoring for mon-1, got %v", dtc.updateMonConfigIDs)
	}
	assertBefore(t, dtc.callSeq, "installExtension", "updateMonitoring")
	if dtc.monCalledWith.projectID != "my-project" {
		t.Errorf("projectID = %q, want my-project", dtc.monCalledWith.projectID)
	}
}

func TestGCPUpdateCreatesWhenNoConfig(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{
		findConnRefs: []connRef{{objectID: "conn-1", serviceAccountEmail: "dtwiz-gcp@my-project.iam.gserviceaccount.com"}},
		// no monitoring config present
	}
	err := captureStdoutErr(func() error {
		return updateGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, uninstallGcloudRunner("my-project"), dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dtc.createMonCalled {
		t.Error("expected createMonitoring to be called when no config exists")
	}
}

func TestGCPUpdateExtensionFailureStopsBeforeMonitoringConfig(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{
		findConnRefs:    []connRef{{objectID: "conn-1", serviceAccountEmail: "dtwiz-gcp@my-project.iam.gserviceaccount.com"}},
		findMonConfigID: "mon-1",
		installExtErr:   fmt.Errorf("extension install failed"),
	}
	err := captureStdoutErr(func() error {
		return updateGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, uninstallGcloudRunner("my-project"), dtc)
	})
	if err == nil {
		t.Fatal("expected extension install error, got nil")
	}
	if len(dtc.updateMonConfigIDs) != 0 || dtc.createMonCalled {
		t.Error("monitoring config must not be reconciled when extension installation fails")
	}
}

func TestGCPUpdateRejectsPartialConnection(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{
		// connection with no bound service account → not updatable
		findConnRefs: []connRef{{objectID: "conn-1", serviceAccountEmail: ""}},
	}
	err := captureStdoutErr(func() error {
		return updateGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, uninstallGcloudRunner("my-project"), dtc)
	})
	if err == nil {
		t.Fatal("expected error for partial connection, got nil")
	}
	if !strings.Contains(err.Error(), "no complete GCP connection") {
		t.Errorf("error %q does not explain the partial connection", err.Error())
	}
}

func TestGCPUpdateDryRun(t *testing.T) {
	defer stubExecLookPath(t)()

	dtc := &fakeDTClient{
		findConnRefs:    []connRef{{objectID: "conn-1", serviceAccountEmail: "dtwiz-gcp@my-project.iam.gserviceaccount.com"}},
		findMonConfigID: "mon-1",
	}
	out := captureStdout(t, func() {
		_ = updateGCPWithRunner("https://abc.live.dynatrace.com", "tok", true, time.Time{}, uninstallGcloudRunner("my-project"), dtc)
	})
	if len(dtc.updateMonConfigIDs) != 0 || dtc.createMonCalled {
		t.Error("dry-run must not change any monitoring config")
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected [dry-run] marker; got:\n%s", out)
	}
}
