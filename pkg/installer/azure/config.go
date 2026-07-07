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
	// integrationName is the fixed name shared by the DT connection, the DT
	// monitoring configuration, and the Azure App Registration / Service Principal.
	integrationName = "dtwiz-azure"

	// fedCredName is the fixed name of the Entra federated credential.
	fedCredName = "dtwiz-azure-Federated-Credential"
)

// realRunner is the production cmdRunner implementation.
var realRunner = installer.RealRunner
