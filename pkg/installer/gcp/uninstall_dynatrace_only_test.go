package gcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

const boundSAEmail = "dtwiz-gcp-abc@deleted-project.iam.gserviceaccount.com"

func dynatraceOnlyFakeDTClient() *fakeDTClient {
	return &fakeDTClient{
		findConnRefs:    []connRef{{objectID: "conn-1", serviceAccountEmail: boundSAEmail}},
		findMonConfigID: "mon-1",
	}
}

func TestUninstallDynatraceOnly_DeletesDynatraceSideWithoutGCloud(t *testing.T) {
	// Arrange
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	origLookPath := execLookPath
	execLookPath = func(string) (string, error) { return "", errors.New("gcloud must not be needed") }
	defer func() { execLookPath = origLookPath }()
	dtc := dynatraceOnlyFakeDTClient()
	var err error

	// Act
	out := captureStdout(t, func() { err = uninstallDynatraceOnly("https://abc.live.dynatrace.com", false, dtc) })

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dtc.deleteConnCalled {
		t.Error("expected the connection to be deleted")
	}
	// Colored lines (header, left-in-place warning) bypass the stdout capture.
	assertInOrder(t, out,
		"Service account "+boundSAEmail+" and its Viewer binding",
		"Step 1: DT Extensions API: delete monitoring configuration",
		"Step 2: DT Settings API: delete GCP connection",
		"Step 1/2: Delete GCP monitoring configuration",
		"Step 2/2: Delete Dynatrace GCP connection",
	)
	if strings.Contains(out, "gcloud ") {
		t.Errorf("no gcloud step may be previewed or run:\n%s", out)
	}
}

func TestUninstallDynatraceOnly_DryRunDeletesNothing(t *testing.T) {
	// Arrange
	dtc := dynatraceOnlyFakeDTClient()
	var err error

	// Act
	out := captureStdout(t, func() { err = uninstallDynatraceOnly("https://abc.live.dynatrace.com", true, dtc) })

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dtc.deleteConnCalled {
		t.Error("dry-run must not delete anything")
	}
	assertInOrder(t, out, "Step 2: DT Settings API: delete GCP connection", "[dry-run]")
}

func TestUninstallDynatraceOnly_NothingFound(t *testing.T) {
	// Arrange
	dtc := &fakeDTClient{}

	// Act
	out := captureStdout(t, func() { _ = uninstallDynatraceOnly("https://abc.live.dynatrace.com", false, dtc) })

	// Assert
	if !strings.Contains(out, "nothing to uninstall") {
		t.Errorf("expected 'nothing to uninstall'; got:\n%s", out)
	}
}

func TestBoundServiceAccounts_DeduplicatesAndSkipsUnbound(t *testing.T) {
	// Arrange
	conns := []connRef{
		{objectID: "a", serviceAccountEmail: "sa1@p.iam.gserviceaccount.com"},
		{objectID: "b"},
		{objectID: "c", serviceAccountEmail: "sa1@p.iam.gserviceaccount.com"},
		{objectID: "d", serviceAccountEmail: "sa2@p.iam.gserviceaccount.com"},
	}

	// Act
	got := boundServiceAccounts(conns)

	// Assert
	if strings.Join(got, ",") != "sa1@p.iam.gserviceaccount.com,sa2@p.iam.gserviceaccount.com" {
		t.Errorf("boundServiceAccounts() = %v", got)
	}
}
