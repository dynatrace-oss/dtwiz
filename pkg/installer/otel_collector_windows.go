//go:build windows

package installer

import (
	"strconv"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// findRunningOtelCollectors searches for known OTel Collector processes on Windows
// by name pattern via Get-CimInstance.
func findRunningOtelCollectors() []runningCollector {
	seen := map[int]bool{}
	var result []runningCollector
	for _, pattern := range []string{"dynatrace-otel-collector", "otelcol"} {
		lines := winProcessQuery(
			"$_.Name -match '"+pattern+"'",
			"$_.ProcessId",
		)
		if lines == nil {
			logger.Debug("findRunningOtelCollectors: PowerShell query failed", "pattern", pattern)
			continue
		}
		for _, s := range lines {
			pid, err := strconv.Atoi(s)
			if err == nil && !seen[pid] {
				seen[pid] = true
				result = append(result, runningCollector{pid: pid})
				logger.Debug("findRunningOtelCollectors: found", "pid", pid, "pattern", pattern)
			}
		}
	}
	return result
}
