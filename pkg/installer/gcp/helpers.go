package gcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// gcpServiceAccountEmail builds the deterministic email of the monitoring SA.
func gcpServiceAccountEmail(name, projectID string) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com", name, projectID)
}

// findServiceAccountEmail recursively scans a settings value for the first string
// that looks like a Google Cloud service-account email. Field names in the GCP
// connection schemas are environment-managed, so matching the value shape is more
// robust than hard-coding a key.
func findServiceAccountEmail(v any) string {
	switch t := v.(type) {
	case string:
		if isServiceAccountEmail(t) {
			return t
		}
	case map[string]any:
		for _, val := range t {
			if email := findServiceAccountEmail(val); email != "" {
				return email
			}
		}
	case []any:
		for _, val := range t {
			if email := findServiceAccountEmail(val); email != "" {
				return email
			}
		}
	}
	return ""
}

func isServiceAccountEmail(s string) bool {
	return strings.Contains(s, "@") && strings.HasSuffix(s, ".gserviceaccount.com")
}

// gcpCreateServiceAccount creates the monitoring service account and returns its email.
// An "already exists" error from a previous partial install is tolerated: the email
// is deterministic, so the existing account is reused.
func gcpCreateServiceAccount(runner cmdRunner, name, projectID string) (string, error) {
	out, err := runner("gcloud", []string{
		"iam", "service-accounts", "create", name,
		"--display-name", serviceAccountDisplayName,
		"--project", projectID,
		"--format", "json",
	}, nil)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()+out), "already exists") {
			logger.Debug("service account already exists, reusing", "name", name)
			return gcpServiceAccountEmail(name, projectID), nil
		}
		return "", err
	}
	email := parseServiceAccountEmail(out)
	if email == "" {
		// Fall back to the deterministic email when gcloud emits no JSON.
		email = gcpServiceAccountEmail(name, projectID)
	}
	return email, nil
}

func parseServiceAccountEmail(out string) string {
	var sa struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &sa); err != nil {
		return ""
	}
	return sa.Email
}

// gcpDeleteServiceAccount deletes the monitoring service account; not-found is success.
func gcpDeleteServiceAccount(runner cmdRunner, email, projectID string) error {
	_, err := runner("gcloud", []string{
		"iam", "service-accounts", "delete", email,
		"--project", projectID,
		"--quiet",
	}, nil)
	if err != nil && installer.IsNotFoundErr(err) {
		return nil
	}
	return err
}

// gcpRemoveProjectBinding removes an IAM policy binding from the project; not-found is success.
func gcpRemoveProjectBinding(runner cmdRunner, projectID, member, role string) error {
	_, err := runner("gcloud", []string{
		"projects", "remove-iam-policy-binding", projectID,
		"--member", member,
		"--role", role,
		"--condition=None",
		"--quiet",
	}, nil)
	if err != nil && installer.IsNotFoundErr(err) {
		return nil
	}
	return err
}

// serviceAccountMember formats a service-account email as a gcloud IAM member.
func serviceAccountMember(email string) string {
	return "serviceAccount:" + email
}
