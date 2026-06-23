// Package azure implements the Dynatrace Azure Monitor integration installer.
package azure

import (
	"os"
	"os/exec"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// execLookPath is a variable alias for exec.LookPath, allowing tests to stub it.
var execLookPath = exec.LookPath

// cmdRunner is a function that runs a command and captures its stdout.
// It receives the executable name, argument slice, and optional environment
// variables (nil means inherit from the current process).
type cmdRunner func(name string, args []string, env []string) (stdout string, err error)

// azureConfig holds all configuration needed for the Azure Monitor integration.
type azureConfig struct {
	ConnectionName    string
	ConfigurationName string
	EnvURL            string
	PlatformToken     string
	TenantID          string
	SubscriptionID    string
	ManagementGroupID string
	ConnectionID      string // filled after step 1
	ClientID          string // filled after step 2
	ObjectID          string // filled after step 4
}

// fedCredName is the fixed name of the Entra federated credential.
const fedCredName = "dtwiz-azure-Federated-Credential"

// realRunner is the production cmdRunner implementation.
func realRunner(name string, args []string, env []string) (string, error) {
	cmd := exec.Command(name, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	return string(out), err
}

// dtctlEnv builds an environment slice that inherits the current process
// environment and appends DT_ENVIRONMENT and DT_PLATFORM_TOKEN for dtctl.
// The platform token is never logged in plain text.
func dtctlEnv(envURL, platformToken string) []string {
	env := os.Environ()
	env = append(env, "DT_ENVIRONMENT="+envURL)
	env = append(env, "DT_PLATFORM_TOKEN="+platformToken)
	logger.Debug("dtctl env", "DT_ENVIRONMENT", envURL, "DT_PLATFORM_TOKEN", "***")
	return env
}

// maskToken replaces all occurrences of token in s with ***.
func maskToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}
