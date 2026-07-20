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

// stubExecLookPath overrides execLookPath to pretend az is always available,
// and stubs hasSupportedAzVersion to return true (modern az, no legacy path).
// Returns a restore function to defer.
func stubExecLookPath(t *testing.T) func() {
	t.Helper()
	origLookPath := execLookPath
	origHasCP := hasSupportedAzVersion
	execLookPath = func(_ string) (string, error) {
		return "/usr/local/bin/az", nil
	}
	hasSupportedAzVersion = func() bool { return true }
	return func() {
		execLookPath = origLookPath
		hasSupportedAzVersion = origHasCP
	}
}

// ── mock dtclient implementations ─────────────────────────────────────────────

// noopDTClient is used in tests that never reach the DT API calls.
type noopDTClient struct{}

func (noopDTClient) installExtension() error {
	return fmt.Errorf("unexpected installExtension call")
}
func (noopDTClient) createConnection(string) (string, error) {
	return "", fmt.Errorf("unexpected createConnection call")
}
func (noopDTClient) updateConnection(string, string, string, string) error {
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
	connObjectID     string
	connErr          error
	createConnCalled bool
	installExtErr    error
	callSeq          []string // ordered record of installExtension / createMonitoring / updateMonitoring calls
	updateErr        error
	monErr           error

	// uninstall
	findConnObjectID string
	findConnClientID string
	findConnRefs     []connRef // when set, overrides findConnObjectID/findConnClientID
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

	updateCalledWith struct{ objectID, name, tenantID, clientID string }
	monCalledWith    struct{ configName, connObjectID, clientID, subscriptionID string }
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

func (f *fakeDTClient) installExtension() error {
	f.callSeq = append(f.callSeq, "installExtension")
	return f.installExtErr
}

func (f *fakeDTClient) createConnection(string) (string, error) {
	f.createConnCalled = true
	return f.connObjectID, f.connErr
}
func (f *fakeDTClient) updateConnection(objectID, name, tenantID, clientID string) error {
	f.updateCalledWith.objectID = objectID
	f.updateCalledWith.name = name
	f.updateCalledWith.tenantID = tenantID
	f.updateCalledWith.clientID = clientID
	return f.updateErr
}
func (f *fakeDTClient) createMonitoring(configName, connObjectID, clientID, subscriptionID string) error {
	f.createMonCalled = true
	f.callSeq = append(f.callSeq, "createMonitoring")
	f.monCalledWith.configName = configName
	f.monCalledWith.connObjectID = connObjectID
	f.monCalledWith.clientID = clientID
	f.monCalledWith.subscriptionID = subscriptionID
	return f.monErr
}
func (f *fakeDTClient) updateMonitoring(configID, configName, connObjectID, clientID, subscriptionID string) error {
	f.callSeq = append(f.callSeq, "updateMonitoring")
	f.updateMonConfigIDs = append(f.updateMonConfigIDs, configID)
	f.monCalledWith.configName = configName
	f.monCalledWith.connObjectID = connObjectID
	f.monCalledWith.clientID = clientID
	f.monCalledWith.subscriptionID = subscriptionID
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
	return []connRef{{objectID: f.findConnObjectID, clientID: f.findConnClientID}}, nil
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

const stockAccountJSON = `{"id":"sub-abc123","tenantId":"tenant-xyz","name":"my-sub"}`
const stockRBACJSON = `[{"actionId":"Microsoft.Authorization/roleAssignments/write","accessDecision":"Allowed"}]`
const stockSPJSON = `{"appId":"client-id-000","tenant":"tenant-xyz","displayName":"dtwiz-azure"}`
const stockSPShowJSON = `{"id":"object-id-111","appId":"client-id-000"}`

// ── helpers.go tests ──────────────────────────────────────────────────────────

func TestAzureIssuerURL(t *testing.T) {
	cases := []struct {
		name   string
		envURL string
		want   string
	}{
		{
			name:   "prod live",
			envURL: "https://abc.live.dynatrace.com",
			want:   "https://token.dynatrace.com",
		},
		{
			name:   "prod apps",
			envURL: "https://abc.apps.dynatrace.com",
			want:   "https://token.dynatrace.com",
		},
		{
			name:   "lab classic",
			envURL: "https://xyz.dynatracelabs.com",
			want:   "https://token.dynatracelabs.com",
		},
		{
			name:   "dev env",
			envURL: "https://rrx28105.dev.apps.dynatracelabs.com",
			want:   "https://dev.token.dynatracelabs.com",
		},
		{
			name:   "sprint env",
			envURL: "https://rrx28105.sprint.apps.dynatracelabs.com",
			want:   "https://sprint.token.dynatracelabs.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := azureIssuerURL(tc.envURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("azureIssuerURL(%q) = %q, want %q", tc.envURL, got, tc.want)
			}
		})
	}
}

func TestAzureIssuerURL_UnsupportedDomain(t *testing.T) {
	_, err := azureIssuerURL("https://managed.example.com/e/abc123")
	if err == nil {
		t.Fatal("expected error for unsupported domain, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported Dynatrace environment URL") {
		t.Fatalf("expected unsupported URL error, got: %v", err)
	}
}

func TestAzureBuildFederatedCredJSON(t *testing.T) {
	cases := []struct {
		name         string
		connID       string
		envURL       string
		wantSubject  string
		wantAudience string
		wantIssuer   string
	}{
		{
			name:         "prod",
			connID:       "conn-123",
			envURL:       "https://abc.live.dynatrace.com",
			wantSubject:  "dt:connection-id/conn-123",
			wantAudience: "abc.apps.dynatrace.com/svc-id/com.dynatrace.da",
			wantIssuer:   "https://token.dynatrace.com",
		},
		{
			name:         "lab",
			connID:       "conn-456",
			envURL:       "https://xyz.dynatracelabs.com",
			wantSubject:  "dt:connection-id/conn-456",
			wantAudience: "xyz.apps.dynatracelabs.com/svc-id/com.dynatrace.da",
			wantIssuer:   "https://token.dynatracelabs.com",
		},
		{
			name:         "dev env",
			connID:       "conn-789",
			envURL:       "https://rrx28105.dev.apps.dynatracelabs.com",
			wantSubject:  "dt:connection-id/conn-789",
			wantAudience: "rrx28105.dev.apps.dynatracelabs.com/svc-id/com.dynatrace.da",
			wantIssuer:   "https://dev.token.dynatracelabs.com",
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
			if !strings.Contains(got, tc.wantIssuer) {
				t.Errorf("issuer: want %q in output, got: %s", tc.wantIssuer, got)
			}
		})
	}
}

func TestAzureBuildFederatedCredJSON_UnsupportedDomain(t *testing.T) {
	_, err := azureBuildFedCredJSON("conn-123", "https://managed.example.com/e/abc123")
	if err == nil {
		t.Fatal("expected error for unsupported domain, got nil")
	}
}

// ─── azureDeleteFedCred ───────────────────────────────────────────────────────

func TestAzureDeleteFedCred_HappyPath(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) { return "{}", nil }
	if err := azureDeleteFedCred(runner, "client-id-000"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAzureDeleteFedCred_NotFoundTreatedAsSuccess(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("Resource 'dtwiz-azure-Federated-Credential' was not found")
	}
	if err := azureDeleteFedCred(runner, "client-id-000"); err != nil {
		t.Errorf("not-found should be treated as success, got: %v", err)
	}
}

func TestAzureDeleteFedCred_OtherErrorReturned(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("authorization denied")
	}
	if err := azureDeleteFedCred(runner, "client-id-000"); err == nil {
		t.Error("expected error for non-not-found failure, got nil")
	}
}

// ─── azureListAppIDsByName ───────────────────────────────────────────────────

func TestAzureListAppIDsByName_Found(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return `[{"appId":"found-app-id"},{"appId":"found-app-id-2"}]`, nil
	}
	ids, err := azureListAppIDsByName(runner, "dtwiz-azure")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "found-app-id" || ids[1] != "found-app-id-2" {
		t.Errorf("ids = %v, want [found-app-id found-app-id-2]", ids)
	}
}

func TestAzureListAppIDsByName_NotFound(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) { return `[]`, nil }
	ids, err := azureListAppIDsByName(runner, "dtwiz-azure")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no ids for not-found, got %v", ids)
	}
}

func TestAzureListAppIDsByName_RunnerError(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("az command failed")
	}
	if _, err := azureListAppIDsByName(runner, "dtwiz-azure"); err == nil {
		t.Error("expected error from runner failure, got nil")
	}
}

func TestAzureAppHasDtwizFedCred(t *testing.T) {
	const issuer = "https://token.dynatrace.com"
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{
			name: "matching name and issuer",
			out:  `[{"name":"dtwiz-azure-Federated-Credential","issuer":"https://token.dynatrace.com"}]`,
			want: true,
		},
		{
			name: "right name wrong issuer",
			out:  `[{"name":"dtwiz-azure-Federated-Credential","issuer":"https://attacker.example.com"}]`,
			want: false,
		},
		{
			name: "unrelated credential",
			out:  `[{"name":"some-other-cred","issuer":"https://token.dynatrace.com"}]`,
			want: false,
		},
		{
			name: "no credentials",
			out:  `[]`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := func(_ string, _ []string, _ []string) (string, error) { return tc.out, nil }
			got, err := azureAppHasDtwizFedCred(runner, "client-id-000", issuer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("azureAppHasDtwizFedCred = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAzureAppHasDtwizFedCred_RunnerError(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("az command failed")
	}
	if _, err := azureAppHasDtwizFedCred(runner, "client-id-000", "https://token.dynatrace.com"); err == nil {
		t.Error("expected error from runner failure, got nil")
	}
}

func TestAzureDeleteApp_NotFoundIsSuccess(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("Resource 'x' does not exist or one of its queried reference-property objects are not found")
	}
	if err := azureDeleteApp(runner, "client-id-000"); err != nil {
		t.Errorf("not-found should be treated as success, got: %v", err)
	}
}

func TestAzureDeleteApp_OtherErrorReturned(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("authorization denied")
	}
	if err := azureDeleteApp(runner, "client-id-000"); err == nil {
		t.Error("expected error for non-not-found failure, got nil")
	}
}

// ─── azureGetSPObjectID ───────────────────────────────────────────────────────

func TestAzureGetSPObjectID_ForbiddenNoRetry(t *testing.T) {
	callCount := 0
	runner := func(_ string, _ []string, _ []string) (string, error) {
		callCount++
		return "", fmt.Errorf("exit status 1: 403 Forbidden")
	}
	if _, err := azureGetSPObjectID(runner, "client-id-000", noSleep); err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry on 403), got %d", callCount)
	}
}

func TestAzureGetSPObjectID_ExhaustedRetries(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "", fmt.Errorf("Resource 'client-id-000' does not exist")
	}
	sleepCount := 0
	testSleeper := func(_ time.Duration) { sleepCount++ }

	_, err := azureGetSPObjectID(runner, "client-id-000", testSleeper)
	if err == nil {
		t.Fatal("expected error after exhausted retries, got nil")
	}
	if !strings.Contains(err.Error(), "exhausted retries") {
		t.Errorf("error %q does not mention exhausted retries", err.Error())
	}
	if sleepCount != 4 {
		t.Errorf("expected 4 sleeps for 5 attempts, got %d", sleepCount)
	}
}

func TestAzureGetSPObjectID_NotFoundSignalOnlyInStdoutStillRetries(t *testing.T) {
	// Some az CLI error shapes put the useful detail in stdout rather than in the
	// Go/stderr-derived error text. The retry classifier must see the same combined
	// signal the inline check does, or it stops after a single attempt instead of
	// retrying up to 5 times.
	callCount := 0
	runner := func(_ string, _ []string, _ []string) (string, error) {
		callCount++
		if callCount < 3 {
			return `{"error": "does not exist yet"}`, fmt.Errorf("exit status 1")
		}
		return `{"id":"object-id-111"}`, nil
	}
	id, err := azureGetSPObjectID(runner, "client-id-000", noSleep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "object-id-111" {
		t.Errorf("got id %q, want object-id-111", id)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (retries continue based on combined err+stdout signal), got %d", callCount)
	}
}

func TestAzureGetSPObjectID_JSONParseFail(t *testing.T) {
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return "not valid json{{", nil
	}
	if _, err := azureGetSPObjectID(runner, "client-id-000", noSleep); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAzureGetSPObjectID_EmptyIDRetriedThenSucceeds(t *testing.T) {
	// The SP can return HTTP 200 with an empty "id" field while Entra is still
	// propagating. The retry loop must treat this the same as not-found and keep
	// polling until a non-empty ID is returned.
	callCount := 0
	runner := func(_ string, _ []string, _ []string) (string, error) {
		callCount++
		if callCount == 1 {
			return `{"id":""}`, nil // successful response, but ID not yet populated
		}
		return `{"id":"object-id-111"}`, nil
	}
	id, err := azureGetSPObjectID(runner, "client-id-000", noSleep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "object-id-111" {
		t.Errorf("got id %q, want object-id-111", id)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (retry after empty ID), got %d", callCount)
	}
}

func TestAzureGetSPObjectID_EmptyIDAllAttemptsExhausted(t *testing.T) {
	// If every attempt returns a successful 200 with an empty ID, the loop must
	// exhaust all 5 retries and return an error rather than returning an empty string.
	runner := func(_ string, _ []string, _ []string) (string, error) {
		return `{"id":""}`, nil
	}
	sleepCount := 0
	testSleeper := func(_ time.Duration) { sleepCount++ }
	_, err := azureGetSPObjectID(runner, "client-id-000", testSleeper)
	if err == nil {
		t.Fatal("expected error after all attempts return empty ID, got nil")
	}
	if !strings.Contains(err.Error(), "exhausted retries") {
		t.Errorf("error %q does not mention exhausted retries", err.Error())
	}
	if sleepCount != 4 {
		t.Errorf("expected 4 sleeps for 5 attempts, got %d", sleepCount)
	}
}

// ─── retryingDTClient ─────────────────────────────────────────────────────────

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

// retryingDTClient wraps fakeDTClient and delegates updateConnection to a custom
// function, allowing tests to vary behaviour across successive calls.
type retryingDTClient struct {
	*fakeDTClient
	updateFn func(objectID, name, tenantID, clientID string) error
}

func (r *retryingDTClient) updateConnection(objectID, name, tenantID, clientID string) error {
	if r.updateFn != nil {
		return r.updateFn(objectID, name, tenantID, clientID)
	}
	return r.fakeDTClient.updateConnection(objectID, name, tenantID, clientID)
}
