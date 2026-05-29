package analyzer

import "strings"

// detectPodman checks for a running Podman daemon.
func detectPodman() *PodmanInfo {
	info := &PodmanInfo{}

	ok, _ := runCmd("podman", "version", "--format", "{{.Server.Version}}")
	if !ok {
		return info
	}
	info.Available = true

	_, ver := runCmd("podman", "version", "--format", "{{.Server.Version}}")
	info.ServerVersion = ver

	_, psOut := runCmd("podman", "ps", "-q")
	if psOut != "" {
		info.RunningContainerCount = len(strings.Split(strings.TrimSpace(psOut), "\n"))
	}

	info.Variant = detectPodmanVariant()
	return info
}

// detectPodmanVariant identifies the Podman distribution (Desktop, Machine).
func detectPodmanVariant() string {
	_, osInfo := runCmd("podman", "info", "--format", "{{.Host.Distribution.Distribution}}")
	_, ver := runCmd("podman", "version", "--format", "{{.Client.Version}}")
	return podmanVariantFromStrings(osInfo, ver)
}

func podmanVariantFromStrings(osInfo, ver string) string {
	osLower := strings.ToLower(osInfo)
	verLower := strings.ToLower(ver)

	switch {
	case strings.Contains(osLower, "podman desktop") || strings.Contains(verLower, "desktop"):
		return "Podman Desktop"
	case strings.Contains(osLower, "fedora") && strings.Contains(osLower, "wsl"):
		return "Podman Machine"
	default:
		return ""
	}
}
