package installer

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type InstallOptions struct {
	DryRun                bool
	MonitoringMode        string
	NoVerifySignature     bool
	SkipConnectivityCheck bool
	ConnectivityCheckOnly bool
	PrintEndpoints        bool
	Quiet                 bool
}

type AgentConfig struct {
	MonitoringMode string
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MonitoringMode: string(InstallModeFullStack)}
}

func ResolveAgentConfig(opts InstallOptions) AgentConfig {
	cfg := DefaultAgentConfig()
	cfg.MonitoringMode = opts.MonitoringMode
	logger.Debug("resolved agent config",
		"monitoring-mode", cfg.MonitoringMode,
		"override_set", opts.MonitoringMode != string(InstallModeFullStack),
	)
	return cfg
}

func InstallOneAgentV2(c *client.Client, opts InstallOptions) error {
	display.PrintStatusLine("oneagent", fmt.Sprintf("under development (monitoring-mode=%s) — use --oneagent-poc=false or set DTWIZ_ONEAGENT_POC=false to use the stable installer", opts.MonitoringMode), display.ColorWarning)
	return nil
}
