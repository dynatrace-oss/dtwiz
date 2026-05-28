//go:build windows

package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// processWorkingDirWindows returns the working directory of the given PID on Windows.
// Win32_Process does not expose the CWD directly; returns empty string (relative paths
// remain unresolved). ExecutablePath from WMI is always absolute, so this only matters
// when --config is passed with a relative path, which is uncommon on Windows.
func processWorkingDirWindows(_ int) string {
	return ""
}

// findRunningOtelCollectors returns only Dynatrace OTel Collector processes on Windows.
func findRunningOtelCollectors() []runningCollector {
	seen := map[int]bool{}
	var result []runningCollector
	lines, err := winProcessQuery(
		"$_.Name -match 'dynatrace-otel-collector'",
		"$_.ProcessId",
	)
	if err != nil {
		logger.Debug("findRunningOtelCollectors: PowerShell query failed", "err", err)
		return nil
	}
	for _, s := range lines {
		pid, err := strconv.Atoi(s)
		if err == nil && !seen[pid] {
			seen[pid] = true
			result = append(result, runningCollector{pid: pid})
			logger.Debug("findRunningOtelCollectors: found", "pid", pid)
		}
	}
	return result
}

// findAllRunningOtelCollectors returns all running OTel Collector processes on Windows,
// including both Dynatrace and non-Dynatrace distributions.
func findAllRunningOtelCollectors() []collectorInstance {
	currentPID := os.Getpid()

	// Query processes whose name or executable path contains an OTel collector pattern.
	// Patterns mirror otelCollectorBinaryPatterns (otel_collector_select.go).
	script := `Get-CimInstance Win32_Process | Where-Object {` +
		` $n = $_.Name.ToLower();` +
		` $e = if ($_.ExecutablePath) { $_.ExecutablePath.ToLower() } else { '' };` +
		` ($n -match 'dynatrace-otel-collector|otelcorecol|otelcol|opentelemetry-collector') -or` +
		` ($e -match 'dynatrace-otel-collector|otelcorecol|otelcol|opentelemetry-collector')` +
		` } | ForEach-Object { "$($_.ProcessId)|$($_.ExecutablePath)|$($_.CommandLine)" }`

	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		logger.Debug("findAllRunningOtelCollectors: PowerShell query failed", "err", err)
		return nil
	}

	var result []collectorInstance
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid == currentPID {
			continue
		}
		binaryPath := ""
		if len(parts) > 1 {
			binaryPath = strings.TrimSpace(parts[1])
		}
		cmdLine := ""
		if len(parts) > 2 {
			cmdLine = strings.TrimSpace(parts[2])
		}
		configPath := detectConfigFromArgs(cmdLine)
		// Resolve relative config paths against the process's working directory.
		if configPath != "" && !filepath.IsAbs(configPath) {
			if procCWD := processWorkingDirWindows(pid); procCWD != "" {
				configPath = filepath.Join(procCWD, configPath)
			}
		}
		logger.Debug("findAllRunningOtelCollectors: found", "pid", pid, "binary", binaryPath)
		result = append(result, collectorInstance{
			pid:         pid,
			binaryPath:  binaryPath,
			configPath:  configPath,
			isDynatrace: isDynatraceOtelCollector(binaryPath),
		})
	}
	return result
}
