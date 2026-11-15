//go:build windows

package otel

import (
	"os"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// listProcesses returns every process with a command line. Windows does not
// expose working directories through Win32_Process, so relative executable
// paths cannot be resolved there.
func listProcesses() []DetectedProcess {
	lines, err := winProcessQuery(
		"$_.CommandLine",
		"\"$($_.ProcessId)|$($_.CommandLine)\"",
	)
	if err != nil {
		logger.Debug("listProcesses: PowerShell query failed", "err", err)
		return nil
	}
	return parseWinProcessLines(lines, os.Getpid())
}

func parseWinProcessLines(lines []string, selfPID int) []DetectedProcess {
	processes := make([]DetectedProcess, 0, len(lines))
	for _, line := range lines {
		row := strings.SplitN(line, "|", 2)
		if len(row) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(row[0]))
		if err != nil || pid == selfPID {
			continue
		}
		processes = append(processes, DetectedProcess{PID: pid, Command: strings.TrimSpace(row[1])})
	}
	return processes
}
