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
	return AgentConfig{MonitoringMode: "fullstack"}
}

func ResolveAgentConfig(opts InstallOptions) AgentConfig {
	cfg := DefaultAgentConfig()
	cfg.MonitoringMode = opts.MonitoringMode
	logger.Debug("resolved agent config",
		"monitoring_mode", cfg.MonitoringMode,
		"override_set", opts.MonitoringMode != "fullstack",
	)
	return cfg
}

func InstallOneAgentV2(c *client.Client, opts InstallOptions) error {
	display.PrintStatusLine("oneagent", fmt.Sprintf("under development (monitoring-mode=%s) — set DTWIZ_ONEAGENT_POC=false to use the stable installer", opts.MonitoringMode), display.ColorWarning)
	return nil
}
