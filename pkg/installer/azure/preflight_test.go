package azure

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// captureColorOutput captures text written via fatih/color (display.Color* helpers).
func captureColorOutput(fn func()) string {
	var buf bytes.Buffer
	orig := color.Output
	color.Output = &buf
	defer func() { color.Output = orig }()
	fn()
	return buf.String()
}

func TestAzurePreflightAzNotFound(t *testing.T) {
	origLookPath := execLookPath
	execLookPath = func(name string) (string, error) {
		if name == "az" {
			return "", fmt.Errorf("exec: %q: executable file not found in $PATH", name)
		}
		return "/usr/local/bin/" + name, nil
	}
	defer func() { execLookPath = origLookPath }()

	runner := func(_ string, _ []string, _ []string) (string, error) { return "", nil }

	err := installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, &noopDTClient{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Azure CLI") {
		t.Errorf("expected 'Azure CLI' in error, got: %v", err)
	}
}

func TestAzurePreflightNotLoggedIn(t *testing.T) {
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		if name == "az" && len(args) > 0 && args[0] == "account" {
			return "", fmt.Errorf("az login required")
		}
		return "", nil
	}

	err := installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, &noopDTClient{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "az login") {
		t.Errorf("expected az login hint in error, got: %v", err)
	}
}

// TestAzureCheckRBACAdvisory verifies the RBAC check is advisory: it warns but
// never blocks, whether the check call fails or reports insufficient access.
func TestAzureCheckRBACAdvisory(t *testing.T) {
	t.Run("denied — warns at subscription scope, does not block", func(t *testing.T) {
		var checkAccessURL string
		runner := func(_ string, args []string, _ []string) (string, error) {
			// First call is az ad signed-in-user show — return a valid user.
			if len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user" {
				return `{"id":"user-object-id"}`, nil
			}
			// Second call is az rest (checkAccess) — return denied.
			for i, a := range args {
				if a == "--url" && i+1 < len(args) {
					checkAccessURL = args[i+1]
				}
			}
			return `[{"actionId":"Microsoft.Authorization/roleAssignments/write","accessDecision":"Denied"}]`, nil
		}
		out := captureColorOutput(func() {
			azureCheckRBAC(runner, "/subscriptions/sub-abc123")
		})
		if !strings.Contains(out, "Warning") {
			t.Errorf("expected a warning on denied access, got: %s", out)
		}
		if !strings.Contains(checkAccessURL, "/subscriptions/sub-abc123/providers/Microsoft.Authorization/checkAccess") {
			t.Errorf("expected checkAccess at subscription scope, got URL: %q", checkAccessURL)
		}
	})

	t.Run("check call fails — warns 'could not validate', does not block", func(t *testing.T) {
		runner := func(_ string, args []string, _ []string) (string, error) {
			// First call is az ad signed-in-user show — return a valid user.
			if len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user" {
				return `{"id":"user-object-id"}`, nil
			}
			// Second call is az rest (checkAccess) — simulate failure.
			return "", fmt.Errorf("exit status 1: InvalidResourceType")
		}
		out := captureColorOutput(func() {
			azureCheckRBAC(runner, "/subscriptions/sub-abc123")
		})
		if !strings.Contains(out, "could not validate") {
			t.Errorf("expected 'could not validate' warning, got: %s", out)
		}
	})
}

// TestAzureCheckRBACSkipsWhenSignedInUserFails verifies that a failure to
// resolve the signed-in user causes the RBAC check to be skipped silently
// — no warning printed, no checkAccess call made.
func TestAzureCheckRBACSkipsWhenSignedInUserFails(t *testing.T) {
	checkCalled := false
	runner := func(_ string, args []string, _ []string) (string, error) {
		if len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user" {
			return "", fmt.Errorf("not logged in")
		}
		checkCalled = true
		return "{}", nil
	}
	captureColorOutput(func() {
		azureCheckRBAC(runner, "/subscriptions/sub-abc123")
	})
	if checkCalled {
		t.Error("checkAccess should not be called when signed-in-user lookup fails")
	}
}

// TestAzureCheckRBACSkipsWhenSignedInUserHasEmptyID verifies that an empty
// object ID in the signed-in-user response causes the RBAC check to be skipped.
func TestAzureCheckRBACSkipsWhenSignedInUserHasEmptyID(t *testing.T) {
	checkCalled := false
	runner := func(_ string, args []string, _ []string) (string, error) {
		if len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user" {
			return `{"id":""}`, nil
		}
		checkCalled = true
		return "{}", nil
	}
	captureColorOutput(func() {
		azureCheckRBAC(runner, "/subscriptions/sub-abc123")
	})
	if checkCalled {
		t.Error("checkAccess should not be called when signed-in-user returns empty ID")
	}
}

// TestAzureCheckRBACNoWarningOnAllowed verifies that an Allowed decision
// produces no warning output — the happy path must be silent.
func TestAzureCheckRBACNoWarningOnAllowed(t *testing.T) {
	runner := func(_ string, args []string, _ []string) (string, error) {
		if len(args) > 1 && args[0] == "ad" && args[1] == "signed-in-user" {
			return `{"id":"user-object-id"}`, nil
		}
		return `[{"actionId":"Microsoft.Authorization/roleAssignments/write","accessDecision":"Allowed"}]`, nil
	}
	out := captureColorOutput(func() {
		azureCheckRBAC(runner, "/subscriptions/sub-abc123")
	})
	if strings.Contains(out, "Warning") {
		t.Errorf("expected no warning for Allowed RBAC check, got: %s", out)
	}
}

// TestAzurePreflightContinuesPastRBACDenial verifies that a denied RBAC check
// does not abort the install — the flow proceeds to the first mutating step.
func TestAzurePreflightContinuesPastRBACDenial(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	defer func() { installer.AutoConfirm = old }()
	defer stubExecLookPath(t)()

	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 0 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return `[{"actionId":"Microsoft.Authorization/roleAssignments/write","accessDecision":"Denied"}]`, nil
		default:
			return "{}", nil
		}
	}

	// createConnection (step 1) errors — but only reachable if we get past preflight.
	dtc := &fakeDTClient{connErr: fmt.Errorf("boom from step 1")}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, dtc)
	})
	if err == nil {
		t.Fatal("expected step 1 error, got nil")
	}
	if strings.Contains(err.Error(), "RBAC") || strings.Contains(err.Error(), "checkAccess") {
		t.Errorf("preflight should not block on RBAC denial; got: %v", err)
	}
	if !strings.Contains(err.Error(), "boom from step 1") {
		t.Errorf("expected to reach step 1 past advisory RBAC check, got: %v", err)
	}
}
