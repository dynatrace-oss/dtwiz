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
	return info
}
