package gcp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// gcloudArgs reports whether the gcloud args start with the given prefix tokens.
func gcloudArgs(args []string, prefix ...string) bool {
	if len(args) < len(prefix) {
		return false
	}
	for i, p := range prefix {
		if args[i] != p {
			return false
		}
	}
	return true
}

// happyGcloudRunner handles every gcloud invocation in the install flow.
// mutating counts calls that change cloud state (everything but config reads).
func happyGcloudRunner(mutating *int) cmdRunner {
	return func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "gcloud" && gcloudArgs(args, "config", "get-value", "project"):
			return "my-project\n", nil
		case name == "gcloud" && gcloudArgs(args, "config", "get-value", "account"):
			return "user@example.com\n", nil
		case name == "gcloud" && gcloudArgs(args, "iam", "service-accounts", "create"):
			if mutating != nil {
				*mutating++
			}
			return stockSACreateJSON, nil
		default:
			if mutating != nil {
				*mutating++
			}
			return "{}", nil
		}
	}
}

// ── flow tests ────────────────────────────────────────────────────────────────

func TestGCPHappyPath(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyFakeDTClient()
	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "dt0s16.fake.token", false, time.Time{}, happyGcloudRunner(nil), noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dtc.updateCalledWith.serviceAccountEmail != "dtwiz-gcp@my-project.iam.gserviceaccount.com" {
		t.Errorf("expected updateConnection with SA email, got %q", dtc.updateCalledWith.serviceAccountEmail)
	}
	if dtc.monCalledWith.connObjectID == "" {
		t.Error("expected createMonitoring to be called")
	}
	if dtc.monCalledWith.projectID != "my-project" {
		t.Errorf("createMonitoring projectID = %q, want my-project", dtc.monCalledWith.projectID)
	}
}

func TestGCPDryRun(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	mutating := 0
	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "dt0s16.fake.token", true, time.Time{}, happyGcloudRunner(&mutating), noSleep, &noopDTClient{})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mutating != 0 {
		t.Errorf("dry-run: expected 0 mutating gcloud calls, got %d", mutating)
	}
}

func TestGCPConnectionAlreadyExistsIsRejected(t *testing.T) {
	defer stubExecLookPath(t)()

	mutating := 0
	dtc := &fakeDTClient{
		connObjectID:     "a1b2c3d4-0000-0000-0000-000000000001",
		dtSAEmail:        "dt-monitor@dynatrace-prod.iam.gserviceaccount.com",
		findConnObjectID: "existing-conn-id",
		findConnSAEmail:  "dtwiz-gcp@my-project.iam.gserviceaccount.com",
	}
	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(&mutating), noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error for existing connection, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
	if mutating != 0 {
		t.Errorf("expected 0 mutating gcloud calls, got %d", mutating)
	}
}

func TestGCPMultipleIncompleteConnectionsAreRejected(t *testing.T) {
	defer stubExecLookPath(t)()

	mutating := 0
	dtc := &fakeDTClient{
		connObjectID: "a1b2c3d4-0000-0000-0000-000000000001",
		dtSAEmail:    "dt-monitor@dynatrace-prod.iam.gserviceaccount.com",
		findConnRefs: []connRef{{objectID: "partial-1"}, {objectID: "partial-2"}},
	}
	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(&mutating), noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error for ambiguous incomplete connections, got nil")
	}
	if !strings.Contains(err.Error(), "2 incomplete GCP connections") {
		t.Errorf("expected 'incomplete GCP connections' in error, got: %v", err)
	}
	if mutating != 0 {
		t.Errorf("expected 0 mutating gcloud calls, got %d", mutating)
	}
}

// TestGCPPartialInstallResumesExistingConnection covers a connection left behind by an
// install that failed between step 2 (create connection) and step 6 (bind service account):
// re-running install must reuse that connection object instead of creating a duplicate.
func TestGCPPartialInstallResumesExistingConnection(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyFakeDTClient()
	dtc.findConnObjectID = "partial-conn-id"
	dtc.findConnSAEmail = ""

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "dt0s16.fake.token", false, time.Time{}, happyGcloudRunner(nil), noSleep, dtc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dtc.createConnCalled {
		t.Error("expected createConnection not to be called when resuming a partial connection")
	}
	if dtc.updateCalledWith.objectID != "partial-conn-id" {
		t.Errorf("expected updateConnection to reuse existing connection id, got %q", dtc.updateCalledWith.objectID)
	}
	if dtc.monCalledWith.connObjectID != "partial-conn-id" {
		t.Errorf("expected createMonitoring to reuse existing connection id, got %q", dtc.monCalledWith.connObjectID)
	}
}

func TestGCPDTPrincipalUnresolvedFailsBeforeMutations(t *testing.T) {
	defer stubExecLookPath(t)()

	mutating := 0
	dtc := happyFakeDTClient()
	dtc.dtSAErr = fmt.Errorf("no Dynatrace GCP principal found")

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(&mutating), noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error when DT principal cannot be resolved, got nil")
	}
	if mutating != 0 {
		t.Errorf("expected 0 mutating gcloud calls, got %d", mutating)
	}
}

func TestGCPStep2ConnectionFailsAfterAPIEnable(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	mutating := 0
	failDTC := happyFakeDTClient()
	failDTC.connErr = fmt.Errorf("DT API: create connection failed")

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(&mutating), noSleep, failDTC)
	})
	if err == nil {
		t.Fatal("expected error from step 2, got nil")
	}
	if !strings.Contains(err.Error(), "step 2") {
		t.Errorf("error %q does not mention step 2", err.Error())
	}
	if mutating != 1 {
		t.Errorf("expected only API enable before connection failure, got %d mutating gcloud calls", mutating)
	}
}

func TestGCPStep1PermissionDeniedIncludesRoleHint(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case gcloudArgs(args, "config", "get-value", "project"):
			return "my-project\n", nil
		case gcloudArgs(args, "config", "get-value", "account"):
			return "user@example.com\n", nil
		case gcloudArgs(args, "services", "enable"):
			return "", fmt.Errorf("AUTH_PERMISSION_DENIED: Permission denied to enable service")
		default:
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyFakeDTClient())
	})
	if err == nil {
		t.Fatal("expected error from step 1, got nil")
	}
	if !strings.Contains(err.Error(), "step 1") {
		t.Errorf("error %q does not mention step 1", err.Error())
	}
	if !strings.Contains(err.Error(), "roles/serviceusage.serviceUsageAdmin") {
		t.Errorf("expected service usage role hint, got: %v", err)
	}
}

func TestGCPStep3PermissionDeniedIncludesRoleHint(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case gcloudArgs(args, "config", "get-value", "project"):
			return "my-project\n", nil
		case gcloudArgs(args, "config", "get-value", "account"):
			return "user@example.com\n", nil
		case gcloudArgs(args, "iam", "service-accounts", "create"):
			return "", fmt.Errorf("AUTH_PERMISSION_DENIED: Permission denied to create service account")
		default:
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyFakeDTClient())
	})
	if err == nil {
		t.Fatal("expected error from step 3, got nil")
	}
	if !strings.Contains(err.Error(), "step 3") {
		t.Errorf("error %q does not mention step 3", err.Error())
	}
	if !strings.Contains(err.Error(), "roles/iam.serviceAccountAdmin") {
		t.Errorf("expected service-account-admin role hint, got: %v", err)
	}
}

func TestGCPStep4RetriesServiceAccountPropagation(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	step4Attempts := 0
	sleepCount := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case gcloudArgs(args, "config", "get-value", "project"):
			return "my-project\n", nil
		case gcloudArgs(args, "config", "get-value", "account"):
			return "user@example.com\n", nil
		case gcloudArgs(args, "iam", "service-accounts", "create"):
			return stockSACreateJSON, nil
		case gcloudArgs(args, "projects", "add-iam-policy-binding"):
			step4Attempts++
			// Freshly created SA not yet visible to IAM on the first two tries.
			if step4Attempts < 3 {
				return "", fmt.Errorf("INVALID_ARGUMENT: Service account dtwiz-gcp@my-project.iam.gserviceaccount.com does not exist.")
			}
			return "{}", nil
		default:
			return "{}", nil
		}
	}
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, testSleeper, happyFakeDTClient())
	})
	if err != nil {
		t.Fatalf("expected success after SA propagation retries, got: %v", err)
	}
	if step4Attempts != 3 {
		t.Errorf("expected 3 step-4 attempts, got %d", step4Attempts)
	}
	if sleepCount < 2 {
		t.Errorf("expected at least 2 sleeps between step-4 retries, got %d", sleepCount)
	}
}

func TestGCPStep5FailsAllCleanupHints(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case gcloudArgs(args, "config", "get-value", "project"):
			return "my-project\n", nil
		case gcloudArgs(args, "config", "get-value", "account"):
			return "user@example.com\n", nil
		case gcloudArgs(args, "iam", "service-accounts", "create"):
			return stockSACreateJSON, nil
		case gcloudArgs(args, "iam", "service-accounts", "add-iam-policy-binding"):
			return "", fmt.Errorf("PERMISSION_DENIED: cannot set IAM policy")
		default:
			return "{}", nil
		}
	}

	var out string
	err := func() error {
		r, w, _ := os.Pipe()
		old := os.Stdout
		os.Stdout = w
		runErr := installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, happyFakeDTClient())
		os.Stdout = old
		w.Close()
		b, _ := io.ReadAll(r)
		out = string(b)
		return runErr
	}()

	if err == nil {
		t.Fatal("expected error from step 5, got nil")
	}
	if !strings.Contains(out, "dtctl delete gcp connection") {
		t.Errorf("expected DT connection cleanup hint; got:\n%s", out)
	}
	if !strings.Contains(out, "gcloud iam service-accounts delete") {
		t.Errorf("expected service account cleanup hint; got:\n%s", out)
	}
	if !strings.Contains(out, "remove-iam-policy-binding") {
		t.Errorf("expected project binding cleanup hint; got:\n%s", out)
	}
}

func TestGCPInstallCancelled(t *testing.T) {
	// AutoConfirm is false (default): ConfirmProceed reads from stdin; EOF -> cancelled.
	defer stubExecLookPath(t)()

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(nil), noSleep, happyFakeDTClient())
	})
	if err != installer.ErrInstallCancelled {
		t.Errorf("expected ErrInstallCancelled, got: %v", err)
	}
}

func TestGCPStep6Retried(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	updateAttempts := 0
	sleepCount := 0
	dtc := &retryingDTClient{
		fakeDTClient: happyFakeDTClient(),
		updateFn: func(_, _, _ string) error {
			updateAttempts++
			if updateAttempts < 3 {
				return fmt.Errorf("update settings object: API error (400): Constraints violated.")
			}
			return nil
		},
	}
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(nil), testSleeper, dtc)
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if updateAttempts != 3 {
		t.Errorf("expected 3 update attempts, got %d", updateAttempts)
	}
	if sleepCount < 2 {
		t.Errorf("expected at least 2 sleeps between retries, got %d", sleepCount)
	}
}

func TestGCPStep6FailsFastOnUnknownProperty(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	updateAttempts := 0
	sleepCount := 0
	dtc := &retryingDTClient{
		fakeDTClient: happyFakeDTClient(),
		updateFn: func(_, _, _ string) error {
			updateAttempts++
			return fmt.Errorf(`update settings object "obj-001": API error (400): Constraints violated. (builtin:hyperscaler-authentication.connections.gcp/0/serviceAccountImpersonation/serviceAccount: Unknown property)`)
		},
	}
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(nil), testSleeper, dtc)
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if updateAttempts != 1 {
		t.Errorf("expected exactly 1 attempt (no retry on a permanent schema mismatch), got %d", updateAttempts)
	}
	if sleepCount != 0 {
		t.Errorf("expected no sleeps, got %d", sleepCount)
	}
}

func TestGCPStep6FailsFastOnPermanentPermissionError(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	updateAttempts := 0
	sleepCount := 0
	dtc := &retryingDTClient{
		fakeDTClient: happyFakeDTClient(),
		updateFn: func(_, _, _ string) error {
			updateAttempts++
			return fmt.Errorf(`update settings object "obj-001": API error (403): insufficient permission to write settings`)
		},
	}
	testSleeper := func(_ time.Duration) { sleepCount++ }

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(nil), testSleeper, dtc)
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if updateAttempts != 1 {
		t.Errorf("expected exactly 1 attempt (no retry on a permanent Dynatrace-side permission error), got %d", updateAttempts)
	}
	if sleepCount != 0 {
		t.Errorf("expected no sleeps, got %d", sleepCount)
	}
}

func TestGCPStep2ConnectionConflictHint(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := &retryingDTClient{
		fakeDTClient: happyFakeDTClient(),
		createFn: func(_ string) (string, error) {
			return "", fmt.Errorf(`create connection: create settings object for schema "builtin:hyperscaler-authentication.connections.gcp": API error (400): 400 Bad Request - [{"code":400,"error":{"code":400,"message":"Constraints violated.","constraintViolations":[{"path":"builtin:hyperscaler-authentication.connections.gcp/5/name","message":"There is another connection defined under this name."}]}}]`)
		},
	}

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(nil), noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from step 2, got nil")
	}
	if !strings.Contains(err.Error(), "step 2") {
		t.Errorf("error %q does not mention step 2", err.Error())
	}
	if !strings.Contains(err.Error(), "hidden from this token's view") {
		t.Errorf("expected connection-conflict hint in error; got: %v", err)
	}
}

func TestGCPStep7Fails(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	dtc := happyFakeDTClient()
	dtc.monErr = fmt.Errorf("create monitoring: API error")

	err := captureStdoutErr(func() error {
		return installGCPWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, happyGcloudRunner(nil), noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected error from step 7, got nil")
	}
	if !strings.Contains(err.Error(), "step 7") {
		t.Errorf("error %q does not mention step 7", err.Error())
	}
}

// ── gcpBuildStepCommands tests ────────────────────────────────────────────────

func TestGCPBuildStepCommands_StepCount(t *testing.T) {
	cfg := gcpConfig{
		ConnectionName:     integrationName,
		ConfigurationName:  integrationName,
		EnvURL:             "https://abc.live.dynatrace.com",
		ProjectID:          "my-project",
		ServiceAccountName: serviceAccountName,
	}
	if got := len(gcpBuildStepCommands(cfg)); got != 7 {
		t.Errorf("expected 7 steps, got %d", got)
	}
}

func TestGCPBuildStepCommands_PlaceholderWhenPrincipalEmpty(t *testing.T) {
	cfg := gcpConfig{
		ConnectionName:     integrationName,
		ConfigurationName:  integrationName,
		EnvURL:             "https://abc.live.dynatrace.com",
		ProjectID:          "my-project",
		ServiceAccountName: serviceAccountName,
		// DTServiceAccount intentionally empty
	}
	steps := gcpBuildStepCommands(cfg)
	if !strings.Contains(steps[4], "<dynatrace-principal>") {
		t.Errorf("step 5: want <dynatrace-principal> placeholder; got: %s", steps[4])
	}
}

func TestGCPBuildStepCommands_RealValues(t *testing.T) {
	cfg := gcpConfig{
		ConnectionName:      integrationName,
		ConfigurationName:   integrationName,
		EnvURL:              "https://abc.live.dynatrace.com",
		ProjectID:           "my-project",
		ServiceAccountName:  serviceAccountName,
		ServiceAccountEmail: "dtwiz-gcp@my-project.iam.gserviceaccount.com",
		DTServiceAccount:    "dt-monitor@dynatrace-prod.iam.gserviceaccount.com",
	}
	steps := gcpBuildStepCommands(cfg)

	checks := []struct {
		step int
		want string
	}{
		{1, "compute.googleapis.com"},
		{2, integrationName},
		{3, serviceAccountName},
		{4, "roles/viewer"},
		{5, "dt-monitor@dynatrace-prod.iam.gserviceaccount.com"},
		{5, "roles/iam.serviceAccountTokenCreator"},
		{6, "dtwiz-gcp@my-project.iam.gserviceaccount.com"},
		{7, integrationName},
	}
	for _, tc := range checks {
		if !strings.Contains(steps[tc.step-1], tc.want) {
			t.Errorf("step %d: want %q; got: %s", tc.step, tc.want, steps[tc.step-1])
		}
	}
}

// ── token masking ─────────────────────────────────────────────────────────────

func TestGCPMaskToken(t *testing.T) {
	const secret = "dt0s16.verysecrettoken.abc"
	preview := captureStdout(t, func() {
		gcpPrintPreview(gcpConfig{
			ConnectionName:     integrationName,
			ConfigurationName:  integrationName,
			EnvURL:             "https://abc.live.dynatrace.com",
			PlatformToken:      secret,
			ProjectID:          "my-project",
			ServiceAccountName: serviceAccountName,
			DTServiceAccount:   "dt-monitor@dynatrace-prod.iam.gserviceaccount.com",
		})
	})
	if strings.Contains(preview, secret) {
		t.Errorf("platform token must not appear in preview output; got:\n%s", preview)
	}
	if !strings.Contains(preview, "***") {
		t.Errorf("expected *** placeholder in preview output; got:\n%s", preview)
	}
}

// ── retryingDTClient ──────────────────────────────────────────────────────────

// retryingDTClient wraps fakeDTClient and delegates createConnection/updateConnection to custom functions.
type retryingDTClient struct {
	*fakeDTClient
	createFn func(name string) (string, error)
	updateFn func(objectID, name, serviceAccountEmail string) error
}

func (r *retryingDTClient) createConnection(name string) (string, error) {
	if r.createFn != nil {
		return r.createFn(name)
	}
	return r.fakeDTClient.createConnection(name)
}

func (r *retryingDTClient) updateConnection(objectID, name, serviceAccountEmail string) error {
	if r.updateFn != nil {
		return r.updateFn(objectID, name, serviceAccountEmail)
	}
	return r.fakeDTClient.updateConnection(objectID, name, serviceAccountEmail)
}
