package analyzer

import "strings"

// detectDocker checks for a running Docker daemon.
func detectDocker() *DockerInfo {
	info := &DockerInfo{}

	ok, _ := runCmd("docker", "version", "--format", "{{.Server.Version}}")
	if !ok {
		return info
	}
	info.Available = true

	_, ver := runCmd("docker", "version", "--format", "{{.Server.Version}}")
	info.ServerVersion = ver

	_, psOut := runCmd("docker", "ps", "-q")
	if psOut != "" {
		info.RunningContainerCount = len(strings.Split(strings.TrimSpace(psOut), "\n"))
	}

	info.Variant = detectDockerVariant()
	return info
}

// detectDockerVariant identifies the Docker distribution (Desktop, Rancher, OrbStack, Colima).
func detectDockerVariant() string {
	_, osInfo := runCmd("docker", "info", "--format", "{{.OperatingSystem}}")
	_, ctx := runCmd("docker", "context", "show")
	osLower := strings.ToLower(osInfo)
	ctxLower := strings.ToLower(ctx)

	switch {
	case strings.Contains(osLower, "docker desktop") || strings.Contains(ctxLower, "desktop-linux"):
		return "Docker Desktop"
	case strings.Contains(osLower, "rancher") || strings.Contains(ctxLower, "rancher-desktop"):
		return "Rancher Desktop"
	case strings.Contains(osLower, "orbstack") || strings.Contains(ctxLower, "orbstack"):
		return "OrbStack"
	case strings.Contains(ctxLower, "colima"):
		return "Colima"
	default:
		return ""
	}
}
