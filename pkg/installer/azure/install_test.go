package azure

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// ── fake az runner ────────────────────────────────────────────────────────────

type fakeCall struct {
	name   string
	stdout string
	err    error
}

type fakeAzureRunner struct {
	t     *testing.T
	calls []fakeCall
	idx   int
}

func (f *fakeAzureRunner) run(name string, args []string, _ []string) (string, error) {
	if f.idx >= len(f.calls) {
		f.t.Errorf("unexpected call #%d: %s %v", f.idx+1, name, args)
		return "", fmt.Errorf("unexpected call: %s", name)
	}
	call := f.calls[f.idx]
	f.idx++
	if call.name != "" && call.name != name {
		f.t.Errorf("call #%d: want name %q, got %q", f.idx, call.name, name)
	}
	return call.stdout, call.err
}

// buildHappyPathAzRunner returns a runner that handles the az steps for the install flow.
func buildHappyPathAzRunner(t *testing.T) *fakeAzureRunner {
	t.Helper()
	return &fakeAzureRunner{
		t: t,
		calls: []fakeCall{
			{name: "az", stdout: stockAccountJSON}, // preflight: account show
			{name: "az", stdout: stockRBACJSON},    // preflight: signed-in-user (fails parse → RBAC skipped)
			{name: "az", stdout: stockSPJSON},      // step 2: create-for-rbac
			{name: "az", stdout: `{}`},             // step 3
			{name: "az", stdout: stockSPShowJSON},  // step 4
			{name: "az", stdout: `{}`},             // step 5
		},
	}
}

// ── flow tests ────────────────────────────────────────────────────────────────

func TestAzureHappyPath(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyFakeDTClient()
	fr := buildHappyPathAzRunner(t)
	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "dt0s16.fake.token", false, time.Time{}, fr.run, noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.idx != len(fr.calls) {
		t.Errorf("expected %d az calls, got %d", len(fr.calls), fr.idx)
	}
	if dtc.updateCalledWith.clientID == "" {
		t.Error("expected updateConnection to be called with clientID")
	}
	if dtc.monCalledWith.connObjectID == "" {
		t.Error("expected createMonitoring to be called")
	}
	if !dtc.installExtCalled {
		t.Error("expected extension to be installed before createMonitoring")
	}
}

func TestAzureInstallExtensionFailureStopsBeforeMonitoringConfig(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyFakeDTClient()
	dtc.installExtErr = fmt.Errorf("extension install failed")
	fr := buildHappyPathAzRunner(t)

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, fr.run, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected extension install error, got nil")
	}
	if !strings.Contains(err.Error(), "installing extension") {
		t.Errorf("expected extension context in error, got: %v", err)
	}
	if dtc.createMonCalled {
		t.Error("createMonitoring must not run when extension installation fails")
	}
}

func TestAzureDryRun(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	mutatingCalls := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user":
			return `{"id":"user-object-id"}`, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			mutatingCalls++
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "dt0s16.fake.token", true, time.Time{}, runner, noSleep, &noopDTClient{})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mutatingCalls != 0 {
		t.Errorf("dry-run: expected 0 mutating az calls, got %d", mutatingCalls)
	}
}

func TestAzureInstallRequiresSupportedAzVersion(t *testing.T) {
	defer stubExecLookPath(t)()
	hasSupportedAzVersion = func() bool { return false }

	azCallsAfterAccountShow := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		default:
			azCallsAfterAccountShow++
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, &noopDTClient{})
	})
	if err == nil {
		t.Fatal("expected unsupported Azure CLI version error, got nil")
	}
	if !strings.Contains(err.Error(), "update Azure CLI") {
		t.Errorf("expected Azure CLI update guidance, got: %v", err)
	}
	if azCallsAfterAccountShow != 0 {
		t.Errorf("expected no az calls after account preflight, got %d", azCallsAfterAccountShow)
	}
}

func TestAzureConnectionIDFlowsToStep3(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	const wantConnID = "a1b2c3d4-0000-0000-0000-000000000001"
	var step3Args []string

	dtc := happyFakeDTClient()
	fr := buildHappyPathAzRunner(t)
	capturingRunner := func(name string, args []string, env []string) (string, error) {
		out, err := fr.run(name, args, env)
		if name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "app" && args[2] == "federated-credential" {
			step3Args = args
		}
		return out, err
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, capturingRunner, noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, a := range step3Args {
		if strings.Contains(a, wantConnID) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("connection ID %q not found in step 3 args: %v", wantConnID, step3Args)
	}
}

func TestAzureClientIDFlowsToStep6(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	const wantClientID = "client-id-000"

	dtc := happyFakeDTClient()
	fr := buildHappyPathAzRunner(t)

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, fr.run, noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dtc.updateCalledWith.clientID != wantClientID {
		t.Errorf("expected updateConnection clientID=%q, got %q", wantClientID, dtc.updateCalledWith.clientID)
	}
}

func TestAzureObjectIDFlowsToStep5(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	const wantObjectID = "object-id-111"
	var step5Args []string

	dtc := happyFakeDTClient()
	fr := buildHappyPathAzRunner(t)
	capturingRunner := func(name string, args []string, env []string) (string, error) {
		out, err := fr.run(name, args, env)
		if name == "az" && len(args) > 0 && args[0] == "role" {
			step5Args = args
		}
		return out, err
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, capturingRunner, noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, a := range step5Args {
		if a == wantObjectID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("objectID %q not found in step 5 args: %v", wantObjectID, step5Args)
	}
}

func TestAzureConnectionAlreadyExistsIsRejected(t *testing.T) {
	defer stubExecLookPath(t)()

	azMutatingCalls := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user":
			return `{"id":"user-object-id"}`, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			azMutatingCalls++
			return "{}", nil
		}
	}

	dtc := &fakeDTClient{
		connObjectID:     "a1b2c3d4-0000-0000-0000-000000000001",
		findConnObjectID: "existing-conn-id", // simulate existing connection
	}
	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error for existing connection, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
	if azMutatingCalls != 0 {
		t.Errorf("expected 0 az mutating calls, got %d", azMutatingCalls)
	}
}

func TestAzureInstallWithMultipleConnectionsIsRejected(t *testing.T) {
	defer stubExecLookPath(t)()

	azMutatingCalls := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user":
			return `{"id":"user-object-id"}`, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			azMutatingCalls++
			return "{}", nil
		}
	}

	dtc := &fakeDTClient{
		connObjectID: "a1b2c3d4-0000-0000-0000-000000000001",
		findConnRefs: []connRef{
			{objectID: "conn-1", clientID: "client-1"},
			{objectID: "conn-2", clientID: "client-2"},
		},
	}
	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error for ambiguous multiple connections, got nil")
	}
	if !strings.Contains(err.Error(), "uninstall azure") {
		t.Errorf("expected guidance to uninstall+install, got: %v", err)
	}
	if dtc.createConnCalled {
		t.Error("install must not create a connection when duplicates already exist")
	}
	if azMutatingCalls != 0 {
		t.Errorf("expected 0 az mutating calls, got %d", azMutatingCalls)
	}
}

func TestAzureInstallWithCompleteConnectionDelegatesToUpdate(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()
	hasSupportedAzVersion = func() bool { return false }

	azMutatingCalls := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user":
			return `{"id":"user-object-id"}`, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			azMutatingCalls++
			return "{}", nil
		}
	}

	dtc := happyUninstallFakeDTClient() // complete connection + one monitoring config
	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dtc.createConnCalled {
		t.Error("install must not recreate the connection when a complete one already exists")
	}
	if azMutatingCalls != 0 {
		t.Errorf("expected 0 az mutating calls (no SP/federated-credential/role recreation), got %d", azMutatingCalls)
	}
	if got := dtc.updateMonConfigIDs; len(got) != 1 || got[0] != "mon-config-001" {
		t.Errorf("expected the existing monitoring config to be reconciled in place, got %v", got)
	}
	if dtc.createMonCalled {
		t.Error("createMonitoring must not be called when a config already exists")
	}
}

// ── failure injection tests ───────────────────────────────────────────────────

func TestAzureStep1FailsNoAzMutations(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	azMutatingCalls := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user":
			return `{"id":"user-object-id"}`, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			azMutatingCalls++
			return "", nil
		}
	}

	failDTC := &fakeDTClient{connErr: fmt.Errorf("DT API: create connection failed")}
	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, failDTC)
	})
	if err == nil {
		t.Fatal("expected error from step 1, got nil")
	}
	if azMutatingCalls != 0 {
		t.Errorf("expected 0 az mutating calls after step 1 failure, got %d", azMutatingCalls)
	}
}

func TestAzureStep2FailsMentionsDTConnection(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "create-for-rbac":
			return "", fmt.Errorf("az ad sp create-for-rbac: permission denied")
		default:
			return "{}", nil
		}
	}

	var out string
	err := func() error {
		r, w, _ := os.Pipe()
		old := os.Stdout
		os.Stdout = w
		runErr := installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyFakeDTClient())
		os.Stdout = old
		w.Close()
		b, _ := io.ReadAll(r)
		out = string(b)
		return runErr
	}()

	if err == nil {
		t.Fatal("expected error from step 2, got nil")
	}
	if !strings.Contains(out, "dtwiz-azure") {
		t.Errorf("expected cleanup hint for DT connection in output; got:\n%s", out)
	}
}

func TestAzureStep2EmptyAppIDStopsBeforeStep3(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	step3Called := false
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "create-for-rbac":
			return `{"appId":"","tenant":"tenant-xyz","displayName":"dtwiz-azure"}`, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "app" && args[2] == "federated-credential":
			step3Called = true
		}
		return `{}`, nil
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyFakeDTClient())
	})
	if err == nil {
		t.Fatal("expected error from empty appId, got nil")
	}
	if !strings.Contains(err.Error(), "step 2: az returned empty appId") {
		t.Errorf("expected empty appId error, got: %v", err)
	}
	if step3Called {
		t.Fatal("expected install to stop before federated credential creation")
	}
}

func TestAzureStep5FailsAllCleanupHints(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "create-for-rbac":
			return stockSPJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "app":
			return `{}`, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "show":
			return stockSPShowJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "role":
			return "", fmt.Errorf("az role assignment create: insufficient permissions")
		default:
			return "{}", nil
		}
	}

	var out string
	err := func() error {
		r, w, _ := os.Pipe()
		old := os.Stdout
		os.Stdout = w
		runErr := installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyFakeDTClient())
		os.Stdout = old
		w.Close()
		b, _ := io.ReadAll(r)
		out = string(b)
		return runErr
	}()

	if err == nil {
		t.Fatal("expected error from step 5, got nil")
	}
	if !strings.Contains(out, "dtctl delete azure connection") {
		t.Errorf("expected DT connection cleanup hint; got:\n%s", out)
	}
	if !strings.Contains(out, "az ad app delete") {
		t.Errorf("expected App Registration cleanup hint; got:\n%s", out)
	}
	if !strings.Contains(out, "federated credential") {
		t.Errorf("expected fedcred mention in App Registration hint; got:\n%s", out)
	}
}

func TestAzureInstallCancelled(t *testing.T) {
	// AutoConfirm is false (default): ConfirmProceed reads from stdin; EOF -> cancelled.
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user":
			return `{"id":"user-object-id"}`, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		default:
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, &noopDTClient{})
	})
	if !isErrInstallCancelled(err) {
		t.Errorf("expected ErrInstallCancelled, got: %v", err)
	}
}

func TestAzureStep3StaleFedCredReplaced(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	fedCredCreateAttempts := 0
	fedCredDeleteCalled := false

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "create-for-rbac":
			return stockSPJSON, nil
		case name == "az" && len(args) > 3 && args[0] == "ad" && args[1] == "app" &&
			args[2] == "federated-credential" && args[3] == "create":
			fedCredCreateAttempts++
			if fedCredCreateAttempts == 1 {
				return "", fmt.Errorf("A federated identity credential with the name '%s' already exists", fedCredName)
			}
			return `{}`, nil
		case name == "az" && len(args) > 3 && args[0] == "ad" && args[1] == "app" &&
			args[2] == "federated-credential" && args[3] == "delete":
			fedCredDeleteCalled = true
			return `{}`, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "show":
			return stockSPShowJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "role":
			return `{}`, nil
		default:
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyFakeDTClient())
	})
	if err != nil {
		t.Fatalf("expected success after stale fedcred replacement, got: %v", err)
	}
	if !fedCredDeleteCalled {
		t.Error("expected stale fedcred to be deleted before retry")
	}
	if fedCredCreateAttempts != 2 {
		t.Errorf("expected 2 fedcred create attempts (1 fail + 1 success), got %d", fedCredCreateAttempts)
	}
}

func TestAzureStep6AADSTS70025Retried(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	updateAttempts := 0
	sleepCount := 0

	dtc := &retryingDTClient{
		fakeDTClient: happyFakeDTClient(),
		updateFn: func(_, _, _, _ string) error {
			updateAttempts++
			if updateAttempts < 3 {
				return fmt.Errorf("update failed: AADSTS70025: no federated credentials configured")
			}
			return nil
		},
	}

	fr := buildHappyPathAzRunner(t)
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, fr.run, testSleeper, dtc)
	})
	if err != nil {
		t.Fatalf("expected success after AADSTS70025 retries, got: %v", err)
	}
	if updateAttempts != 3 {
		t.Errorf("expected 3 update attempts (2 fail + 1 success), got %d", updateAttempts)
	}
	if sleepCount < 2 {
		t.Errorf("expected at least 2 sleeps between retries, got %d", sleepCount)
	}
}

func TestAzureStep6ConstraintsViolatedRetried(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	updateAttempts := 0
	sleepCount := 0

	dtc := &retryingDTClient{
		fakeDTClient: happyFakeDTClient(),
		updateFn: func(_, _, _, _ string) error {
			updateAttempts++
			if updateAttempts < 3 {
				return fmt.Errorf("update settings object \"obj-abc\": API error (400): Constraints violated.")
			}
			return nil
		},
	}

	fr := buildHappyPathAzRunner(t)
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, fr.run, testSleeper, dtc)
	})
	if err != nil {
		t.Fatalf("expected success after Constraints violated retries, got: %v", err)
	}
	if updateAttempts != 3 {
		t.Errorf("expected 3 update attempts (2 fail + 1 success), got %d", updateAttempts)
	}
	if sleepCount < 2 {
		t.Errorf("expected at least 2 sleeps between retries, got %d", sleepCount)
	}
}

func TestAzureStep6Fails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyFakeDTClient()
	dtc.updateErr = fmt.Errorf("update connection: API error")

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, buildHappyPathAzRunner(t).run, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from step 6, got nil")
	}
	if !strings.Contains(err.Error(), "step 6") {
		t.Errorf("error %q does not mention step 6", err.Error())
	}
}

func TestAzureStep6ExhaustsAllRetries(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	updateAttempts := 0
	dtc := &retryingDTClient{
		fakeDTClient: happyFakeDTClient(),
		updateFn: func(_, _, _, _ string) error {
			updateAttempts++
			return fmt.Errorf("update settings object: API error (400): Constraints violated.")
		},
	}

	sleepCount := 0
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, buildHappyPathAzRunner(t).run, testSleeper, dtc)
	})
	if err == nil {
		t.Fatal("expected error after exhausting all retries, got nil")
	}
	if !strings.Contains(err.Error(), "step 6") {
		t.Errorf("error %q does not mention step 6", err.Error())
	}
	if updateAttempts != 10 {
		t.Errorf("expected 10 update attempts (all retries exhausted), got %d", updateAttempts)
	}
	if sleepCount != 9 {
		t.Errorf("expected 9 sleeps between 10 retries, got %d", sleepCount)
	}
}

// ── azureBuildStepCommands tests ─────────────────────────────────────────────

func TestAzureBuildStepCommands_StepCount(t *testing.T) {
	cfg := azureConfig{
		ConnectionName:    "dtwiz-azure",
		ConfigurationName: "dtwiz-azure",
		EnvURL:            "https://abc.live.dynatrace.com",
		Scope:             "/subscriptions/sub-abc123",
	}
	steps, err := azureBuildStepCommands(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(steps); got != 7 {
		t.Errorf("expected 7 steps, got %d", got)
	}
}

func TestAzureBuildStepCommands_PlaceholdersWhenEmpty(t *testing.T) {
	cfg := azureConfig{
		ConnectionName:    "dtwiz-azure",
		ConfigurationName: "dtwiz-azure",
		EnvURL:            "https://abc.live.dynatrace.com",
		Scope:             "/subscriptions/sub-abc123",
		// ClientID, ConnectionID, ObjectID intentionally empty
	}
	steps, err := azureBuildStepCommands(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(steps[2], "<client-id>") {
		t.Errorf("step 3: want <client-id> placeholder; got: %s", steps[2])
	}
	if !strings.Contains(steps[2], "<connection-id>") {
		t.Errorf("step 3: want <connection-id> placeholder; got: %s", steps[2])
	}
	if !strings.Contains(steps[3], "<client-id>") {
		t.Errorf("step 4: want <client-id> placeholder; got: %s", steps[3])
	}
	if !strings.Contains(steps[4], "<object-id>") {
		t.Errorf("step 5: want <object-id> placeholder; got: %s", steps[4])
	}
}

func TestAzureBuildStepCommands_RealValues(t *testing.T) {
	cfg := azureConfig{
		ConnectionName:    "dtwiz-azure",
		ConfigurationName: "dtwiz-azure",
		EnvURL:            "https://abc.live.dynatrace.com",
		TenantID:          "tenant-xyz",
		Scope:             "/subscriptions/sub-abc123",
		ConnectionID:      "conn-id-001",
		ClientID:          "client-id-000",
		ObjectID:          "object-id-111",
	}
	steps, err := azureBuildStepCommands(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		step int
		want string
	}{
		{1, "dtwiz-azure"},               // connection name
		{2, "dtwiz-azure"},               // sp create-for-rbac --name
		{3, "client-id-000"},             // fed-cred create --id
		{3, "conn-id-001"},               // subject dt:connection-id/<connID>
		{3, fedCredName},                 // fed cred name constant
		{4, "client-id-000"},             // sp show --id
		{5, "object-id-111"},             // role assignment --assignee-object-id
		{5, "/subscriptions/sub-abc123"}, // scope
		{6, "tenant-xyz"},                // update connection tenantId
		{6, "client-id-000"},             // update connection applicationId
		{7, "dtwiz-azure"},               // monitoring configuration name
	}
	for _, tc := range checks {
		if !strings.Contains(steps[tc.step-1], tc.want) {
			t.Errorf("step %d: want %q; got: %s", tc.step, tc.want, steps[tc.step-1])
		}
	}
}

func TestAzureStep7Fails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyFakeDTClient()
	dtc.monErr = fmt.Errorf("create monitoring: API error")

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, buildHappyPathAzRunner(t).run, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from step 7, got nil")
	}
	if !strings.Contains(err.Error(), "step 7") {
		t.Errorf("error %q does not mention step 7", err.Error())
	}
}

func TestAzureStep4RetrySucceeds(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	spShowAttempts := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "create-for-rbac":
			return stockSPJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "app":
			return `{}`, nil
		case name == "az" && len(args) > 2 && args[0] == "ad" && args[1] == "sp" && args[2] == "show":
			spShowAttempts++
			if spShowAttempts < 3 {
				return "", fmt.Errorf("Resource 'client-id-000' does not exist")
			}
			return stockSPShowJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "role":
			return `{}`, nil
		default:
			return "{}", nil
		}
	}

	sleepCount := 0
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, testSleeper, happyFakeDTClient())
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if spShowAttempts != 3 {
		t.Errorf("expected 3 sp show attempts (2 fail + 1 success), got %d", spShowAttempts)
	}
	if sleepCount != 2 {
		t.Errorf("expected 2 sleep calls between retries, got %d", sleepCount)
	}
}
