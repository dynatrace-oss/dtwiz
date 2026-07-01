// Package gcp implements the Dynatrace Google Cloud Platform integration installer.
//
// The flow mirrors pkg/installer/azure but uses Google Cloud service-account
// impersonation instead of Entra federated credentials:
//
//  1. Enable the required Google Cloud APIs (gcloud).
//  2. Create the Dynatrace GCP connection (DT Settings API).
//  3. Create a Google Cloud service account for Dynatrace monitoring (gcloud).
//  4. Grant that service account roles/viewer on the active project (gcloud).
//  5. Grant the Dynatrace principal roles/iam.serviceAccountTokenCreator on the
//     new service account — the impersonation trust (gcloud).
//  6. Update the Dynatrace connection with the service-account email (DT Settings API).
//  7. Create the da-gcp monitoring configuration (DT Extensions API).
//
// VERIFY: the GCP-specific Dynatrace schema identifiers and value shapes live in
// dtapi.go and are isolated behind named constants so they can be corrected
// against a live environment without touching the install flow.
package gcp

import (
	"fmt"
	"os/exec"
	"strings"
)

// execLookPath is a variable alias for exec.LookPath, allowing tests to stub it.
var execLookPath = exec.LookPath

// cmdRunner runs a command and captures its stdout. It receives the executable
// name, argument slice, and optional environment variables (nil = inherit).
type cmdRunner func(name string, args []string, env []string) (stdout string, err error)

// gcpConfig holds all configuration needed for the GCP integration.
type gcpConfig struct {
	ConnectionName      string
	ConfigurationName   string
	EnvURL              string
	PlatformToken       string
	ProjectID           string
	Account             string
	ServiceAccountName  string // GCP service-account ID (local part of the email)
	ServiceAccountEmail string // filled after step 3 (or derived)
	DTServiceAccount    string // Dynatrace principal granted impersonation; resolved before steps
	ConnectionID        string // filled after step 1
}

const (
	// integrationName is the fixed name shared by the DT connection, the DT
	// monitoring configuration, and the Google Cloud service account.
	// TEMPORARY: bumped to dtwiz-gcp-2 to work around a name conflict on the
	// dtwiz-gcp DT connection that neither dtwiz nor the tenant UI can locate.
	integrationName = "dtwiz-gcp-2"

	// serviceAccountName is the fixed Google Cloud service-account ID (6-30 chars,
	// lowercase letters, digits and hyphens). Its email is
	// <serviceAccountName>@<project>.iam.gserviceaccount.com.
	serviceAccountName = "dtwiz-gcp"

	// serviceAccountDisplayName is the human-readable display name of the SA.
	serviceAccountDisplayName = "Dynatrace monitoring account"

	// viewerRole is granted to the monitoring service account on the project.
	viewerRole = "roles/viewer"

	// tokenCreatorRole is granted to the Dynatrace principal on the SA (impersonation).
	tokenCreatorRole = "roles/iam.serviceAccountTokenCreator"
)

// requiredAPIs are enabled on the active project before monitoring can collect data.
var requiredAPIs = []string{
	"compute.googleapis.com",
	"cloudresourcemanager.googleapis.com",
	"cloudasset.googleapis.com",
	"monitoring.googleapis.com",
}

// realRunner is the production cmdRunner implementation.
func realRunner(name string, args []string, env []string) (string, error) {
	cmd := exec.Command(name, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
	}
	return string(out), err
}
