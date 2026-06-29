// Package azure implements the Dynatrace Azure Monitor integration installer.
package azure

import (
	"fmt"
	"os/exec"
	"strings"
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
	Scope             string // subscription scope: /subscriptions/<id>
	ConnectionID      string // filled after step 1
	ClientID          string // filled after step 2
	ObjectID          string // filled after step 4
}

const (
	// integrationName is the fixed name shared by the DT connection, the DT
	// monitoring configuration, and the Azure App Registration / Service Principal.
	integrationName = "dtwiz-azure"

	// fedCredName is the fixed name of the Entra federated credential.
	fedCredName = "dtwiz-azure-Federated-Credential"
)

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

// maskToken replaces all occurrences of token in s with ***.
func maskToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}
