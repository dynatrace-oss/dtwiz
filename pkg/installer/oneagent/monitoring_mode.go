package oneagent

type InstallMode string

const (
	InstallModeFullStack InstallMode = "fullstack"
	InstallModeInfraOnly InstallMode = "infra"
	InstallModeDiscovery InstallMode = "discovery"
)
