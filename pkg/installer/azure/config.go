// Package azure implements the Dynatrace Azure Monitor integration installer.
package azure

import (
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// execLookPath is a variable alias for exec.LookPath, allowing tests to stub it.
var execLookPath = installer.ExecLookPath

// cmdRunner is a function that runs a command and captures its stdout.
// It receives the executable name, argument slice, and optional environment
// variables (nil means inherit from the current process).
type cmdRunner = installer.CmdRunner

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
	// integrationPrefix is the base name prefix for all dtwiz Azure resources.
	// The full name includes the DT tenant ID suffix — use integrationNameForEnv.
	integrationPrefix = "dtwiz-azure"

	// fedCredName is the fixed name of the Entra federated credential within the App Registration.
	fedCredName = "dtwiz-azure-Federated-Credential"
)

// integrationNameForEnv returns the integration name for the given Dynatrace
// environment URL. The DT tenant ID (first DNS label of the environment URL,
// e.g. "fds1499d" from "fds1499d.apps.dynatracelabs.com") is appended so
// the name is stable, human-readable, and unique per DT environment.
func integrationNameForEnv(envURL string) string {
	return integrationPrefix + "-" + installer.ExtractTenantID(envURL)
}

// realRunner is the production cmdRunner implementation.
var realRunner = installer.RealRunner
