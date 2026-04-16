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
	// Patterns to search for in the process list.
	processNames := []string{
		"otelcol.exe",
		"otelcol-contrib.exe",
		"dynatrace-otel-collector.exe",
	}

	// First try Get-Process via powershell for a quick name-based check.
	for _, name := range processNames {
		// Strip the .exe suffix for Get-Process -Name which doesn't want it.
		baseName := strings.TrimSuffix(name, ".exe")
		ok, pidOutput := runCmd("powershell", "-NoProfile", "-Command",
			"Get-Process -Name '"+baseName+"' -ErrorAction SilentlyContinue | Select-Object -First 1 | ForEach-Object { $_.Id }")
		if ok && strings.TrimSpace(pidOutput) != "" {
			// Get the executable path directly from Get-Process (doesn't need elevation).
			binPath := ""
			okPath, pathOutput := runCmd("powershell", "-NoProfile", "-Command",
				"Get-Process -Name '"+baseName+"' -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Path")
			if okPath && strings.TrimSpace(pathOutput) != "" {
				binPath = strings.TrimSpace(pathOutput)
			}
			// Try to get the command line for config path extraction.
			_, configPath := otelInfoFromProcessName(baseName)
			if binPath == "" {
				binPath = baseName + ".exe"
			}
			return true, binPath, configPath
		}
	}

	// Fall back to WMIC full command line search for custom-named builds.
	// Exclude shell processes (powershell, pwsh, cmd) and the current process
	// to avoid matching dtwiz's own detection commands whose arguments contain
	// the search patterns.
	for _, pattern := range []string{"otel-collector", "otelcol"} {
		ok, output := runCmd("powershell", "-NoProfile", "-Command",
			"Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match '"+pattern+"' -and $_.Name -notmatch 'powershell|pwsh|cmd' -and $_.ProcessId -ne $PID } | Select-Object -First 1 -ExpandProperty CommandLine")
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
