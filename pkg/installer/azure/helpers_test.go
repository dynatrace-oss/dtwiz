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

func noSleep(_ time.Duration) {}

// stubExecLookPath overrides execLookPath to pretend az is always available.
// Returns a restore function to defer.
func stubExecLookPath(t *testing.T) func() {
	t.Helper()
	orig := execLookPath
	execLookPath = func(_ string) (string, error) {
		return "/usr/local/bin/az", nil
	}
	return func() { execLookPath = orig }
}

// ── mock dtclient implementations ─────────────────────────────────────────────

// noopDTClient is used in tests that never reach the DT API calls.
type noopDTClient struct{}

func (noopDTClient) createConnection(string) (string, error) {
	return "", fmt.Errorf("unexpected createConnection call")
}
func (noopDTClient) updateConnection(string, string, string, string) error {
	return fmt.Errorf("unexpected updateConnection call")
}
func (noopDTClient) createMonitoring(string, string) error {
	return fmt.Errorf("unexpected createMonitoring call")
}
func (noopDTClient) findConnection(string) (string, string, error) { return "", "", nil }
func (noopDTClient) deleteConnection(string) error                 { return nil }
func (noopDTClient) findMonitoringConfig(string) (string, error)   { return "", nil }
func (noopDTClient) deleteMonitoring(string) error                 { return nil }

// fakeDTClient records calls for assertion.
type fakeDTClient struct {
	connObjectID string
	connErr      error
	updateErr    error
	monErr       error

	// uninstall
	findConnObjectID string
	findConnClientID string
	findConnErr      error
	deleteConnErr    error
	findMonConfigID  string
	findMonErr       error
	deleteMonErr     error

	updateCalledWith struct{ objectID, name, tenantID, clientID string }
	monCalledWith    struct{ configName, connObjectID string }
}

func happyFakeDTClient() *fakeDTClient {
	return &fakeDTClient{connObjectID: "a1b2c3d4-0000-0000-0000-000000000001"}
}

func happyUninstallFakeDTClient() *fakeDTClient {
	return &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnClientID: "client-id-000",
		findMonConfigID:  "mon-config-001",
	}
}

func (f *fakeDTClient) createConnection(string) (string, error) {
	return f.connObjectID, f.connErr
}
func (f *fakeDTClient) updateConnection(objectID, name, tenantID, clientID string) error {
	f.updateCalledWith.objectID = objectID
	f.updateCalledWith.name = name
	f.updateCalledWith.tenantID = tenantID
	f.updateCalledWith.clientID = clientID
	return f.updateErr
}
func (f *fakeDTClient) createMonitoring(configName, connObjectID string) error {
	f.monCalledWith.configName = configName
	f.monCalledWith.connObjectID = connObjectID
	return f.monErr
}
func (f *fakeDTClient) findConnection(string) (string, string, error) {
	return f.findConnObjectID, f.findConnClientID, f.findConnErr
}
func (f *fakeDTClient) deleteConnection(string) error { return f.deleteConnErr }
func (f *fakeDTClient) findMonitoringConfig(string) (string, error) {
	return f.findMonConfigID, f.findMonErr
}
func (f *fakeDTClient) deleteMonitoring(string) error { return f.deleteMonErr }

// ── stock test fixtures ───────────────────────────────────────────────────────

const stockAccountJSON = `{"id":"sub-abc123","tenantId":"tenant-xyz","name":"my-sub"}`
const stockRBACJSON = `[{"actionId":"Microsoft.Authorization/roleAssignments/write","accessDecision":"Allowed"}]`
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
