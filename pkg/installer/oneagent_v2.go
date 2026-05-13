package installer

import (
	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// InstallOptions carries all CLI-derived inputs for the v2 installer flow.
type InstallOptions struct {
	DryRun                bool
	MonitoringMode        string
	NoVerifySignature     bool
	SkipConnectivityCheck bool
	ConnectivityCheckOnly bool
	PrintEndpoints        bool
	Quiet                 bool
}

// AgentConfig holds OneAgent configuration resolved from CLI options.
type AgentConfig struct {
	MonitoringMode string // installer flag: --set-monitoring-mode
}

// DefaultAgentConfig returns the canonical zero-config defaults for a OneAgent install.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MonitoringMode: "fullstack"}
}

// ResolveAgentConfig builds the agent configuration from CLI options.
// Callers should always go through this function rather than constructing AgentConfig directly.
func ResolveAgentConfig(opts InstallOptions) AgentConfig {
	cfg := DefaultAgentConfig()
	cfg.MonitoringMode = opts.MonitoringMode
	logger.Debug("resolved agent config",
		"monitoring_mode", cfg.MonitoringMode,
		"override_set", opts.MonitoringMode != "fullstack",
	)
	return cfg
}

// InstallOneAgentV2 is the entry point for the new OneAgent installer flow.
func InstallOneAgentV2(c *client.Client, opts InstallOptions) error {
	display.PrintStatusLine("oneagent", "under development — set DTWIZ_ONEAGENT_POC=false to use the stable installer", display.ColorWarning)
	return nil
}
