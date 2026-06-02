package analyzer

import (
	"regexp"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// otelContainerPattern matches any image name or container name that looks like an
// OpenTelemetry Collector: "otel…collector" or "opentelemetry…collector" (case-insensitive).
var otelContainerPattern = regexp.MustCompile(`(?i)(otel.+collector|opentelemetry.+collector)`)

// detectOtelCollectorContainer checks docker, podman, and nerdctl for a running container
// whose image or name matches an OTel Collector pattern.
// Returns (found, image) where image is the matched container image string.
func detectOtelCollectorContainer() (bool, string) {
	for _, cli := range []string{"docker", "podman", "nerdctl"} {
		found, image := otelContainerFromRuntime(cli)
		if found {
			logger.Debug("detectOtelCollectorContainer: found via "+cli, "image", image)
			return true, image
		}
	}
	return false, ""
}

// otelContainerFromRuntime queries a single container CLI for running OTel Collector containers.
func otelContainerFromRuntime(cli string) (bool, string) {
	ok, out := runCmd(cli, "ps", "--format", "{{.Image}}\t{{.Names}}")
	if !ok || strings.TrimSpace(out) == "" {
		return false, ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		image := parts[0]
		name := ""
		if len(parts) == 2 {
			name = parts[1]
		}
		if otelContainerPattern.MatchString(image) || otelContainerPattern.MatchString(name) {
			return true, image
		}
	}
	return false, ""
}
