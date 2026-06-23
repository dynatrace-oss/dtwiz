package azure

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

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

func TestAzurePreflightRBACDenied(t *testing.T) {
	defer stubExecLookPath(t)()

	mutatingCalls := 0
	runner := func(name string, args []string, _ []string) (string, error) {
		switch {
		case name == "az" && len(args) > 0 && args[0] == "account" && args[1] == "show":
			return stockAccountJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "account" && args[1] == "management-group":
			return stockMgmtGroupJSON, nil
		case name == "az" && len(args) > 0 && args[0] == "rest":
			return `[{"actionId":"Microsoft.Authorization/roleAssignments/write","accessDecision":"Denied"}]`, nil
		default:
			mutatingCalls++
			return "{}", nil
		}
	}

	err := captureStdoutErr(func() error {
		return installAzureWithRunner("https://abc.live.dynatrace.com", "tok", false, time.Time{}, runner, noSleep, &noopDTClient{})
	})
	if err == nil {
		t.Fatal("expected RBAC denied error, got nil")
	}
	if !strings.Contains(err.Error(), "RBAC") && !strings.Contains(err.Error(), "permissions") {
		t.Errorf("expected RBAC/permissions error, got: %v", err)
	}
	if mutatingCalls != 0 {
		t.Errorf("expected no mutating calls after RBAC denial, got %d", mutatingCalls)
	}
}
