package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsAWSCLIInstalled returns true when the `aws` binary is on PATH.
func IsAWSCLIInstalled() bool {
	_, err := exec.LookPath("aws")
	return err == nil
}

// GetAWSCallerInfo returns the AWS account ID and the configured default region.
func GetAWSCallerInfo() (accountID, region string, err error) {
	// Account ID from STS
	var sterr strings.Builder
	cmd := exec.Command("aws", "sts", "get-caller-identity", "--output", "json")
	cmd.Stderr = &sterr
	out, runErr := cmd.Output()
	if runErr != nil {
		msg := strings.TrimSpace(sterr.String())
		if strings.Contains(msg, "ExpiredToken") {
			return "", "", fmt.Errorf("AWS credentials are expired — run `aws sso login` or refresh your credentials")
		}
		if msg != "" {
			return "", "", fmt.Errorf("aws sts get-caller-identity: %s", msg)
		}
		return "", "", fmt.Errorf("aws sts get-caller-identity: %w", runErr)
	}
	var identity struct {
		Account string `json:"Account"`
	}
	if err := json.Unmarshal(out, &identity); err != nil {
		return "", "", fmt.Errorf("parsing sts identity: %w", err)
	}
	accountID = identity.Account

	// Region: env vars first, then aws configure
	if r := os.Getenv("AWS_DEFAULT_REGION"); r != "" {
		return accountID, r, nil
	}
	if r := os.Getenv("AWS_REGION"); r != "" {
		return accountID, r, nil
	}
	rc, _ := exec.Command("aws", "configure", "get", "region").Output() //nolint:errcheck
	if region = strings.TrimSpace(string(rc)); region != "" {
		return accountID, region, nil
	}
	return accountID, "us-east-1", nil // safe default
}
