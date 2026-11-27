package gcp

import (
	"errors"
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// errNoGCloudAccount is returned by gcpAccountInfo when gcloud has no active account.
var errNoGCloudAccount = errors.New("no active gcloud account")

// errNoGCloudProject is matched (via errors.Is) by the error gcpAccountInfo returns
// when gcloud has an active account but no active project.
var errNoGCloudProject = errors.New("no active gcloud project")

// noProjectError carries the active account so the guidance box can name it:
// `gcloud projects list` only shows projects visible to that account.
type noProjectError struct {
	account string
}

func (e *noProjectError) Error() string { return errNoGCloudProject.Error() }
func (e *noProjectError) Unwrap() error { return errNoGCloudProject }

// gcpAccountInfo resolves the active gcloud account and project, in that order, through
// analyzer.ReadGCloudIdentity, the same read `dtwiz setup` uses for detection.
// It never prints: callers render missing-account/project errors via reportPreflight.
// It is also used by the in-place update path, which never creates resources.
func gcpAccountInfo(runner cmdRunner) (projectID, account string, err error) {
	if _, err = execLookPath("gcloud"); err != nil {
		return "", "", fmt.Errorf("Google Cloud CLI (gcloud) not found: install it from https://cloud.google.com/sdk/docs/install: %w", &installer.DependencyMissingError{Name: "gcloud"}) //nolint:staticcheck // ST1005: "Google Cloud CLI" is a product name
	}

	account, projectID, err = analyzer.ReadGCloudIdentity(func(args ...string) (string, error) {
		return runner("gcloud", args, nil)
	})
	switch {
	case err != nil:
		return "", "", fmt.Errorf("failed to read gcloud configuration: %w", err)
	case account == "":
		return "", "", errNoGCloudAccount
	case projectID == "":
		return "", "", &noProjectError{account: account}
	}

	logger.Debug("gcloud config", "project", projectID, "account", account)
	return projectID, account, nil
}
