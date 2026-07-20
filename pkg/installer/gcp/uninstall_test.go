package gcp

import (
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// uninstallGcloudRunner answers config reads and treats all else as success.
func uninstallGcloudRunner(project string) cmdRunner {
	return func(name string, args []string, _ []string) (string, error) {
		switch {
		case gcloudArgs(args, "config", "get-value", "project"):
			return project + "\n", nil
		case gcloudArgs(args, "config", "get-value", "account"):
			return "user@example.com\n", nil
		default:
			return "{}", nil
		}
	}
}

func TestConnectionExistsWithClient(t *testing.T) {
	tests := []struct {
		name string
		dtc  *fakeDTClient
		want bool
	}{
		{"no connection", &fakeDTClient{}, false},
		{"incomplete connection", &fakeDTClient{findConnObjectID: "partial-conn-id"}, false},
		{
			"complete connection",
			&fakeDTClient{findConnObjectID: "conn-id", findConnSAEmail: "dtwiz-gcp@my-project.iam.gserviceaccount.com"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := connectionExistsWithClient(tt.dtc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("connectionExistsWithClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGCPUninstallHappyPath(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyUninstallFakeDTClient()
	err := captureStdoutErr(func() error {
		return uninstallGCPWithRunner("https://abc.live.dynatrace.com", false, uninstallGcloudRunner("my-project"), dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dtc.deleteConnCalled {
		t.Error("expected deleteConnection to be called")
	}
}

func TestGCPUninstallNothingFound(t *testing.T) {
	defer stubExecLookPath(t)()

	out := captureStdout(t, func() {
		_ = uninstallGCPWithRunner("https://abc.live.dynatrace.com", false, uninstallGcloudRunner("my-project"), &fakeDTClient{})
	})
	if !strings.Contains(out, "nothing to uninstall") {
		t.Errorf("expected 'nothing to uninstall' message; got:\n%s", out)
	}
}

func TestGCPUninstallDryRun(t *testing.T) {
	defer stubExecLookPath(t)()

	deleteCalled := false
	dtc := happyUninstallFakeDTClient()
	out := captureStdout(t, func() {
		_ = uninstallGCPWithRunner("https://abc.live.dynatrace.com", true, uninstallGcloudRunner("my-project"), dtc)
	})
	if dtc.deleteConnCalled || deleteCalled {
		t.Error("dry-run must not delete anything")
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected [dry-run] marker; got:\n%s", out)
	}
}

func TestGCPUninstallStepCount(t *testing.T) {
	conns := []connRef{{objectID: "c1", serviceAccountEmail: "dtwiz-gcp@my-project.iam.gserviceaccount.com"}}
	// 1 mon config + 1 connection + (1 current SA * 2 steps) + (0 legacy * 2) = 4
	if got := uninstallStepCount([]string{"m1"}, conns, []string{"dtwiz-gcp@my-project.iam.gserviceaccount.com"}, nil, "my-project"); got != 4 {
		t.Errorf("step count = %d, want 4", got)
	}
	// without a project, SA steps are skipped: 1 mon + 1 conn = 2
	if got := uninstallStepCount([]string{"m1"}, conns, []string{"dtwiz-gcp@my-project.iam.gserviceaccount.com"}, nil, ""); got != 2 {
		t.Errorf("step count (no project) = %d, want 2", got)
	}
}

func TestGCPGatherServiceAccounts(t *testing.T) {
	conns := []connRef{{objectID: "c1", serviceAccountEmail: "custom@p.iam.gserviceaccount.com"}}
	currentSAs, legacySAs := gcpGatherServiceAccounts(conns, "dtwiz-gcp-test", "my-project")
	// current: connection-bound SA + deterministic from "dtwiz-gcp-test"
	if len(currentSAs) != 2 {
		t.Fatalf("expected 2 current SA emails, got %v", currentSAs)
	}
	wantCurrentDeterministic := gcpServiceAccountEmail("dtwiz-gcp-test", "my-project")
	found := false
	for _, e := range currentSAs {
		if e == wantCurrentDeterministic {
			found = true
		}
	}
	if !found {
		t.Errorf("expected deterministic SA %q in current %v", wantCurrentDeterministic, currentSAs)
	}
	// legacy: deterministic from integrationPrefix (not in current set)
	if len(legacySAs) != 1 {
		t.Fatalf("expected 1 legacy SA email, got %v", legacySAs)
	}
	wantLegacy := gcpServiceAccountEmail(integrationPrefix, "my-project")
	if legacySAs[0] != wantLegacy {
		t.Errorf("legacy SA = %q, want %q", legacySAs[0], wantLegacy)
	}
}
