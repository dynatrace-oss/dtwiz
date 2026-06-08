package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// otelContainerImagePattern matches container image names or container names that look
// like an OTel Collector: "otel…collector" or "opentelemetry…collector" (case-insensitive).
var otelContainerImagePattern = regexp.MustCompile(`(?i)(otel.+collector|opentelemetry.+collector)`)

// containerRuntimes is the ordered list of container CLIs to probe.
var containerRuntimes = []string{"docker", "podman", "nerdctl"}

// findContainerOtelCollectors checks all known container runtimes for running
// OTel Collector containers identified by their image name or container name.
func findContainerOtelCollectors() []collectorInstance {
	var result []collectorInstance
	for _, cli := range containerRuntimes {
		result = append(result, containersFromRuntime(cli)...)
	}
	return result
}

// containersFromRuntime queries a single container CLI for running OTel Collector
// containers, matching by image name or container name.
func containersFromRuntime(cli string) []collectorInstance {
	out, err := exec.Command(cli, "ps", "--format", "{{.ID}}\t{{.Image}}\t{{.Names}}").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return nil
	}

	var result []collectorInstance
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		id := parts[0]
		image := parts[1]
		name := ""
		if len(parts) == 3 {
			name = parts[2]
		}

		if !otelContainerImagePattern.MatchString(image) && !otelContainerImagePattern.MatchString(name) {
			continue
		}

		logger.Debug("containersFromRuntime: found OTel collector container",
			"cli", cli, "image", image, "name", name)

		hostPath, containerPath := resolveContainerPaths(cli, id)
		result = append(result, collectorInstance{
			binaryPath:          image,
			configPath:          hostPath,
			containerConfigPath: containerPath,
			isDynatrace:         isDynatraceOtelCollector(image),
			containerRuntime:    cli,
			containerName:       name,
		})
	}
	return result
}

// containerInspect holds the fields we need from `<cli> inspect` JSON output.
type containerInspect struct {
	Config struct {
		Entrypoint []string `json:"Entrypoint"`
		Cmd        []string `json:"Cmd"`
	} `json:"Config"`
	Mounts []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

// resolveContainerPaths inspects a running container and returns:
//   - hostPath: the host-side path of the config file (non-empty only when the config
//     is bind-mounted from the host, making it directly patchable)
//   - containerPath: the container-internal config path from --config (always set when
//     the flag is present, used for display even when no host mount exists)
func resolveContainerPaths(cli, id string) (hostPath, containerPath string) {
	out, err := exec.Command(cli, "inspect", id).Output()
	if err != nil {
		logger.Debug("resolveContainerPaths: inspect failed", "cli", cli, "id", id, "err", err)
		return "", ""
	}

	var infos []containerInspect
	if err := json.Unmarshal(out, &infos); err != nil || len(infos) == 0 {
		logger.Debug("resolveContainerPaths: failed to parse inspect output", "cli", cli, "id", id)
		return "", ""
	}
	info := infos[0]

	allArgs := append(info.Config.Entrypoint, info.Config.Cmd...) //nolint:gocritic
	containerPath = detectConfigFromArgs(strings.Join(allArgs, " "))
	if containerPath == "" {
		return "", ""
	}

	// Try to resolve container path to a host path via bind mounts.
	for _, m := range info.Mounts {
		if m.Type != "bind" && m.Type != "volume" {
			continue
		}
		if strings.HasPrefix(containerPath, m.Destination) {
			hostPath = m.Source + containerPath[len(m.Destination):]
			logger.Debug("resolveContainerPaths: resolved to host path",
				"container", containerPath, "host", hostPath)
			return hostPath, containerPath
		}
	}

	logger.Debug("resolveContainerPaths: config not host-mounted, showing container path",
		"containerPath", containerPath)
	return "", containerPath
}

// extractContainerConfig copies a config file from inside a container to a
// temporary host file. The caller must delete the returned path when done.
func extractContainerConfig(cli, name, containerPath string) (string, error) {
	tmp, err := os.CreateTemp("", "dtwiz-otel-config-*.yaml")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()

	out, err := exec.Command(cli, "cp", name+":"+containerPath, tmpPath).CombinedOutput()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("%s cp %s:%s failed: %w\n%s", cli, name, containerPath, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("extractContainerConfig: extracted", "cli", cli, "container", name, "containerPath", containerPath, "tmpPath", tmpPath)
	return tmpPath, nil
}

// copyFileToContainer copies a host file into a running container at the given path.
func copyFileToContainer(cli, name, hostPath, containerPath string) error {
	out, err := exec.Command(cli, "cp", hostPath, name+":"+containerPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s cp to %s:%s failed: %w\n%s", cli, name, containerPath, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("copyFileToContainer: copied", "cli", cli, "container", name, "hostPath", hostPath, "containerPath", containerPath)
	return nil
}

// restartContainer restarts a running container by name using the given CLI.
func restartContainer(cli, name string) error {
	out, err := exec.Command(cli, "restart", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s restart %s failed: %w\n%s", cli, name, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("restartContainer: restarted", "cli", cli, "container", name)
	return nil
}
