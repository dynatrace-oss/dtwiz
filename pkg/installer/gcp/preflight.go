package gcp

import (
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// gcpAccountInfo resolves the active gcloud project and account.
// It is also used by the in-place update path, which never creates resources.
func gcpAccountInfo(runner cmdRunner) (projectID, account string, err error) {
	if _, err = execLookPath("gcloud"); err != nil {
		return "", "", fmt.Errorf("Google Cloud CLI (gcloud) not found: install it from https://cloud.google.com/sdk/docs/install") //nolint:staticcheck // ST1005: "Google Cloud CLI" is a product name
	}

	projOut, err := runner("gcloud", []string{"config", "get-value", "project"}, nil)
	if err != nil {
		return "", "", fmt.Errorf("Not logged in to Google Cloud: run `gcloud auth login` and `gcloud config set project <id>`, then retry") //nolint:staticcheck // ST1005: user-facing message
	}
	projectID = analyzer.CleanGCloudConfigValue(projOut)
	if projectID == "" || strings.Contains(projectID, "(unset)") {
		return "", "", fmt.Errorf("no active Google Cloud project: run `gcloud config set project <id>` and retry") //nolint:staticcheck // ST1005: user-facing message
	}

	acctOut, _ := runner("gcloud", []string{"config", "get-value", "account"}, nil)
	account = analyzer.CleanGCloudConfigValue(acctOut)

	logger.Debug("gcloud config", "project", projectID, "account", account)
	return projectID, account, nil
}
