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

// buildHappyPathAzRunner returns a runner that handles only the az steps (2–5).
func buildHappyPathAzRunner(t *testing.T) *fakeAzureRunner {
	t.Helper()
	return &fakeAzureRunner{
		t: t,
		calls: []fakeCall{
			{name: "az", stdout: stockAccountJSON},  // preflight: account show
			{name: "az", stdout: stockMgmtGroupJSON}, // preflight: mgmt group list
			{name: "az", stdout: stockRBACJSON},      // preflight: checkAccess
			{name: "az", stdout: stockSPJSON},        // step 2
			{name: "az", stdout: `{}`},               // step 3
			{name: "az", stdout: stockSPShowJSON},    // step 4
			{name: "az", stdout: `{}`},               // step 5
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return stockRBACJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "ad" && args[1] == "sp":
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

func TestAzureStep5FailsAllCleanupHints(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
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
	if !strings.Contains(out, "az ad sp delete") {
		t.Errorf("expected SP cleanup hint; got:\n%s", out)
	}
	if !strings.Contains(out, "federated-credential delete") {
		t.Errorf("expected fedcred cleanup hint; got:\n%s", out)
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
		case name == "az" && len(args) > 1 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
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
