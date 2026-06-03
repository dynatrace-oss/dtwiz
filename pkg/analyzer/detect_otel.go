//go:build !windows

package analyzer

import "strings"

// detectOtelCollector looks for a running OpenTelemetry Collector process.
// Returns (running, binaryPath, configPath).
func detectOtelCollector() (bool, string, string) {
	// First try exact process name matches for standard distributions.
	for _, bin := range otelCollectorNames {
		ok, pidStr := runCmd("pgrep", "-x", bin)
		if ok {
			binPath, configPath := otelInfoFromPID(strings.TrimSpace(pidStr))
			return true, binPath, configPath
		}
	}
	// Fall back to full command-line search to catch custom builds
	// like dynatrace-otel-collector, opentelemetry-collector, etc.
	for _, pattern := range otelCollectorNames {
		ok, pidStr := runCmd("pgrep", "-f", pattern)
		if ok {
			// pgrep may return multiple PIDs; use the first one.
			pid := strings.TrimSpace(strings.SplitN(pidStr, "\n", 2)[0])
			binPath, configPath := otelInfoFromPID(pid)
			return true, binPath, configPath
		}
	}
	// Fall back to container runtime detection (docker/podman/nerdctl).
	if found, image := detectOtelCollectorContainer(); found {
		return true, image, ""
	}
	return false, "", ""
}

// otelInfoFromPID returns the binary path and --config= path from a process's command line.
func otelInfoFromPID(pid string) (binaryPath, configPath string) {
	if pid == "" {
		return "", ""
	}
	ok, cmdline := runCmd("ps", "-p", pid, "-o", "args=")
	if !ok {
		return "", ""
	}
	if fields := strings.Fields(cmdline); len(fields) > 0 {
		binaryPath = fields[0]
	}
	configPath = extractOtelConfigPath(cmdline)
	return binaryPath, configPath
}

// extractOtelConfigPath parses an otelcol cmdline to find the config path.
// Handles both "--config=<path>" and "--config <path>" forms.
func extractOtelConfigPath(cmdline string) string {
	fields := strings.Fields(cmdline)
	for i, part := range fields {
		if v, ok := strings.CutPrefix(part, "--config="); ok {
			return v
		}
		if part == "--config" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}
