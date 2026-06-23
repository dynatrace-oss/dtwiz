package azure

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── test helpers ──────────────────────────────────────────────────────────────

var stdoutMu sync.Mutex

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	r.Close() //nolint:errcheck
	return string(out)
}

func captureStdoutErr(fn func() error) error {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	err := fn()
	os.Stdout = old
	w.Close()
	io.ReadAll(r) //nolint:errcheck
	return err
}

func captureStdoutReturn(fn func() (string, error)) (string, error) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	val, err := fn()
	os.Stdout = old
	w.Close()
	io.ReadAll(r) //nolint:errcheck
	return val, err
}

func noSleep(_ time.Duration) {}

// stubExecLookPath overrides execLookPath to pretend dtctl and az are always
// available. Returns a restore function to defer.
func stubExecLookPath(t *testing.T) func() {
	t.Helper()
	orig := execLookPath
	execLookPath = func(name string) (string, error) {
		return "/usr/local/bin/" + name, nil
	}
	return func() { execLookPath = orig }
}

// ── stock test fixtures ───────────────────────────────────────────────────────

const stockAccountJSON = `{"id":"sub-abc123","tenantId":"tenant-xyz","name":"my-sub"}`
const stockMgmtGroupJSON = `[{"id":"/providers/Microsoft.Management/managementGroups/tenant-xyz","tenantId":"tenant-xyz","name":"root"}]`
const stockRBACJSON = `[{"actionId":"Microsoft.Authorization/roleAssignments/write","accessDecision":"Allowed"}]`
const stockConnectionJSON = `{"id":"a1b2c3d4-0000-0000-0000-000000000001","name":"dtwiz-azure"}`
const stockSPJSON = `{"appId":"client-id-000","tenant":"tenant-xyz","displayName":"dtwiz-azure"}`
const stockSPShowJSON = `{"id":"object-id-111","appId":"client-id-000"}`

// ── helpers.go tests ──────────────────────────────────────────────────────────

func TestAzureBuildFederatedCredJSON(t *testing.T) {
	cases := []struct {
		name         string
		connID       string
		envURL       string
		wantSubject  string
		wantAudience string
	}{
		{
			name:         "prod",
			connID:       "conn-123",
			envURL:       "https://abc.live.dynatrace.com",
			wantSubject:  "dt:connection-id/conn-123",
			wantAudience: "abc.apps.dynatrace.com/svc-id/com.dynatrace.da",
		},
		{
			name:         "lab",
			connID:       "conn-456",
			envURL:       "https://xyz.dynatracelabs.com",
			wantSubject:  "dt:connection-id/conn-456",
			wantAudience: "xyz.apps.dynatracelabs.com/svc-id/com.dynatrace.da",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := azureBuildFedCredJSON(tc.connID, tc.envURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(got, tc.wantSubject) {
				t.Errorf("subject: want %q in output, got: %s", tc.wantSubject, got)
			}
			if !strings.Contains(got, tc.wantAudience) {
				t.Errorf("audience: want %q in output, got: %s", tc.wantAudience, got)
			}
		})
	}
}

func TestAzureParseConnectionID(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "JSON id field",
			input: `{"id":"a1b2c3d4-0000-0000-0000-000000000000","name":"dtwiz-azure"}`,
			want:  "a1b2c3d4-0000-0000-0000-000000000000",
		},
		{
			name:  "table output fallback UUID",
			input: "Connection created: 11223344-5566-7788-aabb-ccddeeff0011",
			want:  "11223344-5566-7788-aabb-ccddeeff0011",
		},
		{
			name:    "malformed — no UUID",
			input:   "error: something went wrong",
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := azureParseConnectionID(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestAzureMgmtGroupSelection(t *testing.T) {
	t.Run("tenant root present — select it", func(t *testing.T) {
		runner := func(_ string, _ []string, _ []string) (string, error) {
			return `[
				{"id":"/providers/Microsoft.Management/managementGroups/other-group","tenantId":"tenant-abc","name":"other"},
				{"id":"/providers/Microsoft.Management/managementGroups/tenant-abc","tenantId":"tenant-abc","name":"root"}
			]`, nil
		}
		got, err := azureDetectMgmtGroup(runner, "sub-123", "tenant-abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/providers/Microsoft.Management/managementGroups/tenant-abc" {
			t.Errorf("unexpected group: %s", got)
		}
	})

	t.Run("single group — use it", func(t *testing.T) {
		runner := func(_ string, _ []string, _ []string) (string, error) {
			return `[{"id":"/providers/Microsoft.Management/managementGroups/only-group","tenantId":"tenant-xyz","name":"only"}]`, nil
		}
		got, err := azureDetectMgmtGroup(runner, "sub-123", "tenant-xyz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/providers/Microsoft.Management/managementGroups/only-group" {
			t.Errorf("unexpected group: %s", got)
		}
	})

	t.Run("MG list fails — subscription fallback", func(t *testing.T) {
		runner := func(_ string, _ []string, _ []string) (string, error) {
			return "", fmt.Errorf("az command failed")
		}
		got, err := captureStdoutReturn(func() (string, error) {
			return azureDetectMgmtGroup(runner, "sub-fallback-123", "tenant-abc")
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/subscriptions/sub-fallback-123" {
			t.Errorf("expected subscription scope fallback, got: %s", got)
		}
	})
}
