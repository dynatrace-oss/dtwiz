package gcp

import (
	"errors"
	"testing"
)

// gcloudConfigRunner answers `gcloud config get-value account|project` with the given
// raw outputs and records which keys were read.
func gcloudConfigRunner(account, project string, reads *[]string) cmdRunner {
	return func(name string, args []string, _ []string) (string, error) {
		switch {
		case gcloudArgs(args, "config", "get-value", "account"):
			*reads = append(*reads, "account")
			return account, nil
		case gcloudArgs(args, "config", "get-value", "project"):
			*reads = append(*reads, "project")
			return project, nil
		}
		return "", errors.New("unexpected command")
	}
}

func TestGCPAccountInfo_AccountAndProjectSet(t *testing.T) {
	// Arrange
	defer stubExecLookPath(t)()
	var reads []string
	runner := gcloudConfigRunner("jane@example.com\n", "my-project\n", &reads)

	// Act
	projectID, account, err := gcpAccountInfo(runner)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projectID != "my-project" || account != "jane@example.com" {
		t.Errorf("got project=%q account=%q, want my-project / jane@example.com", projectID, account)
	}
}

func TestGCPAccountInfo_NoAccount(t *testing.T) {
	tests := []struct {
		name    string
		account string
		project string
	}{
		{name: "empty account, project set", account: "", project: "my-project"},
		{name: "unset account, project set", account: "(unset)", project: "my-project"},
		{name: "unset account and project", account: "(unset)", project: "(unset)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			defer stubExecLookPath(t)()
			var reads []string
			runner := gcloudConfigRunner(tt.account, tt.project, &reads)

			// Act
			_, _, err := gcpAccountInfo(runner)

			// Assert
			if !errors.Is(err, errNoGCloudAccount) {
				t.Fatalf("err = %v, want errNoGCloudAccount", err)
			}
			if errors.Is(err, errNoGCloudProject) {
				t.Error("account check must win over the project check")
			}
			for _, r := range reads {
				if r == "project" {
					t.Error("project was read although the account was missing")
				}
			}
		})
	}
}

func TestGCPAccountInfo_NoProjectCarriesAccount(t *testing.T) {
	tests := []struct {
		name    string
		project string
	}{
		{name: "empty", project: ""},
		{name: "unset", project: "(unset)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			defer stubExecLookPath(t)()
			var reads []string
			runner := gcloudConfigRunner("jane@example.com\n", tt.project, &reads)

			// Act
			_, _, err := gcpAccountInfo(runner)

			// Assert
			if !errors.Is(err, errNoGCloudProject) {
				t.Fatalf("err = %v, want errNoGCloudProject", err)
			}
			var npe *noProjectError
			if !errors.As(err, &npe) || npe.account != "jane@example.com" {
				t.Errorf("no-project error does not carry the account: %#v", err)
			}
		})
	}
}

func TestGCPAccountInfo_CloudShellNoticeStripped(t *testing.T) {
	// Arrange
	defer stubExecLookPath(t)()
	var reads []string
	notice := "Your active configuration is: [cloudshell-123]\n"
	runner := gcloudConfigRunner(notice+"jane@example.com\n", notice+"my-project\n", &reads)

	// Act
	projectID, account, err := gcpAccountInfo(runner)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projectID != "my-project" || account != "jane@example.com" {
		t.Errorf("got project=%q account=%q, want notice line stripped", projectID, account)
	}
}

func TestGCPAccountInfo_ConfigReadFailureIsNotGuidance(t *testing.T) {
	// Arrange
	defer stubExecLookPath(t)()
	cause := errors.New("exit status 1")
	runner := func(_ string, _ []string, _ []string) (string, error) { return "", cause }

	// Act
	_, _, err := gcpAccountInfo(runner)

	// Assert
	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want wrapped cause", err)
	}
	if errors.Is(err, errNoGCloudAccount) || errors.Is(err, errNoGCloudProject) {
		t.Errorf("a failing gcloud call must not be reported as missing account/project: %v", err)
	}
}
