package gcp

import (
	"fmt"
	"io"
	"os"
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

// stubExecLookPath overrides execLookPath to pretend gcloud is always available.
func stubExecLookPath(t *testing.T) func() {
	t.Helper()
	orig := execLookPath
	execLookPath = func(_ string) (string, error) {
		return "/usr/local/bin/gcloud", nil
	}
	return func() { execLookPath = orig }
}

// ── mock dtclient implementations ─────────────────────────────────────────────

// noopDTClient is used in tests that never reach the DT API calls.
type noopDTClient struct{}

func (noopDTClient) installExtension() (bool, error) {
	return false, fmt.Errorf("unexpected installExtension call")
}
func (noopDTClient) isExtensionActive() (bool, error) {
	return false, fmt.Errorf("unexpected isExtensionActive call")
}
func (noopDTClient) createConnection(string) (string, error) {
	return "", fmt.Errorf("unexpected createConnection call")
}
func (noopDTClient) dtServiceAccount() (string, error) {
	return "dt-monitor@dynatrace.iam.gserviceaccount.com", nil
}
func (noopDTClient) updateConnection(string, string, string) error {
	return fmt.Errorf("unexpected updateConnection call")
}
func (noopDTClient) createMonitoring(string, string, string, string) error {
	return fmt.Errorf("unexpected createMonitoring call")
}
func (noopDTClient) updateMonitoring(string, string, string, string, string) error {
	return fmt.Errorf("unexpected updateMonitoring call")
}
func (noopDTClient) findAllConnections(string) ([]connRef, error)      { return nil, nil }
func (noopDTClient) deleteConnection(string) error                     { return nil }
func (noopDTClient) findAllMonitoringConfigs(string) ([]string, error) { return nil, nil }
func (noopDTClient) deleteMonitoring(string) error                     { return nil }

// fakeDTClient records calls for assertion.
type fakeDTClient struct {
	connObjectID  string
	connErr       error
	dtSAEmail     string
	dtSAErr       error
	installExtErr error
	callSeq       []string // ordered record of installExtension / createMonitoring / updateMonitoring calls
	updateErr     error
	monErr        error

	// uninstall
	findConnObjectID string
	findConnSAEmail  string
	findConnRefs     []connRef // when set, overrides findConnObjectID/findConnSAEmail
	findConnErr      error
	deleteConnErr    error
	deleteConnCalled bool
	findMonConfigID  string
	findMonConfigIDs []string // when set, overrides findMonConfigID for multi-config cases
	findMonErr       error
	deleteMonErr     error

	// in-place update
	updateMonErr       error
	createMonCalled    bool
	updateMonConfigIDs []string

	updateCalledWith struct{ objectID, name, serviceAccountEmail string }
	monCalledWith    struct{ configName, connObjectID, serviceAccountEmail, projectID string }
	createConnCalled bool
}

func happyFakeDTClient() *fakeDTClient {
	return &fakeDTClient{
		connObjectID: "a1b2c3d4-0000-0000-0000-000000000001",
		dtSAEmail:    "dt-monitor@dynatrace-prod.iam.gserviceaccount.com",
	}
}

func happyUninstallFakeDTClient() *fakeDTClient {
	return &fakeDTClient{
		findConnObjectID: "conn-obj-001",
		findConnSAEmail:  "dtwiz-gcp@my-project.iam.gserviceaccount.com",
		findMonConfigID:  "mon-config-001",
	}
}

func (f *fakeDTClient) installExtension() (bool, error) {
	f.callSeq = append(f.callSeq, "installExtension")
	return false, f.installExtErr // false = already installed; no wait loop in tests
}

func (f *fakeDTClient) isExtensionActive() (bool, error) {
	return true, nil
}

func (f *fakeDTClient) createConnection(string) (string, error) {
	f.createConnCalled = true
	return f.connObjectID, f.connErr
}
func (f *fakeDTClient) dtServiceAccount() (string, error) {
	if f.dtSAErr != nil {
		return "", f.dtSAErr
	}
	return f.dtSAEmail, nil
}
func (f *fakeDTClient) updateConnection(objectID, name, serviceAccountEmail string) error {
	f.updateCalledWith.objectID = objectID
	f.updateCalledWith.name = name
	f.updateCalledWith.serviceAccountEmail = serviceAccountEmail
	return f.updateErr
}
func (f *fakeDTClient) createMonitoring(configName, connObjectID, serviceAccountEmail, projectID string) error {
	f.createMonCalled = true
	f.callSeq = append(f.callSeq, "createMonitoring")
	f.monCalledWith.configName = configName
	f.monCalledWith.connObjectID = connObjectID
	f.monCalledWith.serviceAccountEmail = serviceAccountEmail
	f.monCalledWith.projectID = projectID
	return f.monErr
}
func (f *fakeDTClient) updateMonitoring(configID, configName, connObjectID, serviceAccountEmail, projectID string) error {
	f.callSeq = append(f.callSeq, "updateMonitoring")
	f.updateMonConfigIDs = append(f.updateMonConfigIDs, configID)
	f.monCalledWith.configName = configName
	f.monCalledWith.connObjectID = connObjectID
	f.monCalledWith.serviceAccountEmail = serviceAccountEmail
	f.monCalledWith.projectID = projectID
	return f.updateMonErr
}
func (f *fakeDTClient) findAllConnections(string) ([]connRef, error) {
	if f.findConnErr != nil {
		return nil, f.findConnErr
	}
	if f.findConnRefs != nil {
		return f.findConnRefs, nil
	}
	if f.findConnObjectID == "" {
		return nil, nil
	}
	return []connRef{{objectID: f.findConnObjectID, serviceAccountEmail: f.findConnSAEmail}}, nil
}
func (f *fakeDTClient) deleteConnection(string) error {
	f.deleteConnCalled = true
	return f.deleteConnErr
}
func (f *fakeDTClient) findAllMonitoringConfigs(string) ([]string, error) {
	if f.findMonErr != nil {
		return nil, f.findMonErr
	}
	if len(f.findMonConfigIDs) > 0 {
		return f.findMonConfigIDs, nil
	}
	if f.findMonConfigID == "" {
		return nil, nil
	}
	return []string{f.findMonConfigID}, nil
}
func (f *fakeDTClient) deleteMonitoring(string) error { return f.deleteMonErr }

// ── stock test fixtures ───────────────────────────────────────────────────────

const stockSACreateJSON = `{"email":"dtwiz-gcp@my-project.iam.gserviceaccount.com","name":"projects/my-project/serviceAccounts/dtwiz-gcp@my-project.iam.gserviceaccount.com"}`

// ── helpers.go tests ──────────────────────────────────────────────────────────

func TestGCPServiceAccountEmail(t *testing.T) {
	got := gcpServiceAccountEmail("dtwiz-gcp", "my-project")
	want := "dtwiz-gcp@my-project.iam.gserviceaccount.com"
	if got != want {
		t.Errorf("gcpServiceAccountEmail = %q, want %q", got, want)
	}
}

func TestFindServiceAccountEmail(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want string
	}{
		{
			name: "nested in map",
			val:  map[string]any{"foo": "bar", "principal": map[string]any{"email": "dt-monitor@x.iam.gserviceaccount.com"}},
			want: "dt-monitor@x.iam.gserviceaccount.com",
		},
		{
			name: "in slice",
			val:  map[string]any{"items": []any{"not-an-email", "sa@p.iam.gserviceaccount.com"}},
			want: "sa@p.iam.gserviceaccount.com",
		},
		{
			name: "none present",
			val:  map[string]any{"name": "dtwiz-gcp", "type": "serviceAccountImpersonation"},
			want: "",
		},
		{
			name: "plain string is not an email",
			val:  "dtwiz-gcp",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := findServiceAccountEmail(tc.val); got != tc.want {
				t.Errorf("findServiceAccountEmail = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseServiceAccountEmail(t *testing.T) {
	if got := parseServiceAccountEmail(stockSACreateJSON); got != "dtwiz-gcp@my-project.iam.gserviceaccount.com" {
		t.Errorf("parseServiceAccountEmail = %q", got)
	}
	if got := parseServiceAccountEmail("not json"); got != "" {
		t.Errorf("expected empty for invalid JSON, got %q", got)
	}
}

func TestGCPCreateServiceAccount_AlreadyExistsReuses(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("Service account dtwiz-gcp already exists within project my-project")
	}
	email, err := gcpCreateServiceAccount(runner, "dtwiz-gcp", "my-project")
	if err != nil {
		t.Fatalf("already-exists must be tolerated, got: %v", err)
	}
	if email != "dtwiz-gcp@my-project.iam.gserviceaccount.com" {
		t.Errorf("email = %q, want deterministic address", email)
	}
}

func TestGCPCreateServiceAccount_ParsesEmail(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) { return stockSACreateJSON, nil }
	email, err := gcpCreateServiceAccount(runner, "dtwiz-gcp", "my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email != "dtwiz-gcp@my-project.iam.gserviceaccount.com" {
		t.Errorf("email = %q", email)
	}
}

func TestGCPCreateServiceAccount_OtherErrorPropagates(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("PERMISSION_DENIED: caller lacks iam.serviceAccounts.create")
	}
	if _, err := gcpCreateServiceAccount(runner, "dtwiz-gcp", "my-project"); err == nil {
		t.Error("expected error to propagate, got nil")
	}
}

func TestGCPDeleteServiceAccount_NotFoundIsSuccess(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("NOT_FOUND: service account does not exist")
	}
	if err := gcpDeleteServiceAccount(runner, "x@p.iam.gserviceaccount.com", "p"); err != nil {
		t.Errorf("not-found should be success, got: %v", err)
	}
}

// assertBefore checks that first appears before second in seq, failing the test if either is absent or out of order.
func assertBefore(t *testing.T, seq []string, first, second string) {
	t.Helper()
	fi, si := -1, -1
	for i, s := range seq {
		if s == first && fi == -1 {
			fi = i
		}
		if s == second && si == -1 {
			si = i
		}
	}
	if fi == -1 {
		t.Errorf("call sequence: %q was never called; sequence: %v", first, seq)
		return
	}
	if si == -1 {
		t.Errorf("call sequence: %q was never called; sequence: %v", second, seq)
		return
	}
	if fi > si {
		t.Errorf("call sequence: want %q before %q, got order: %v", first, second, seq)
	}
}

func TestGCPRemoveProjectBinding_NotFoundIsSuccess(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("Policy binding not found")
	}
	if err := gcpRemoveProjectBinding(runner, "p", serviceAccountMember("x@p.iam.gserviceaccount.com"), viewerRole); err != nil {
		t.Errorf("not-found should be success, got: %v", err)
	}
}
