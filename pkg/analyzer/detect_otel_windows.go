//go:build windows

package analyzer

import (
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// detectOtelCollector looks for a running OpenTelemetry Collector process on Windows.
// Searches by command-line pattern via Get-CimInstance.
// Returns (running, binaryPath, configPath).
func detectOtelCollector() (bool, string, string) {
	for _, pattern := range []string{"dynatrace-otel-collector", "otelcol"} {
		ok, output := runCmd("powershell", "-NoProfile", "-Command",
			"Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match '"+pattern+"' } | Select-Object -First 1 -ExpandProperty CommandLine")
		if ok && output != "" {
			logger.Debug("detectOtelCollector: found via PowerShell", "pattern", pattern)
			binPath, configPath := parseWindowsCommandLine(output)
			return true, binPath, configPath
		}
	}
	logger.Debug("detectOtelCollector: no collector found")
	return false, "", ""
}

// parseWindowsCommandLine extracts the binary path and OTel config path from a command line string.
func parseWindowsCommandLine(cmdline string) (binaryPath, configPath string) {
	fields := strings.Fields(cmdline)
	if len(fields) > 0 {
		binaryPath = strings.Trim(fields[0], "\"")
	}
	configPath = extractOtelConfigPath(cmdline)
	return binaryPath, configPath
}

// extractOtelConfigPath parses an otelcol cmdline to find the config path.
// Handles both "--config=<path>" and "--config <path>" forms.
func extractOtelConfigPath(cmdline string) string {
	fields := strings.Fields(cmdline)
	for i, part := range fields {
		if strings.HasPrefix(part, "--config=") {
			return strings.TrimPrefix(part, "--config=")
		}
		if part == "--config" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}
