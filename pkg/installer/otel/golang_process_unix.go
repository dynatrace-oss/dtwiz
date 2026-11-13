//go:build !windows

package otel

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// listProcesses returns every process with its command line and working
// directory. The working directories come from a single lsof call.
func listProcesses() []DetectedProcess {
	output, err := exec.Command("ps", "ax", "-o", "pid=,command=").Output()
	if err != nil {
		logger.Warn("ps command failed", "err", err)
		return nil
	}
	cwds := listWorkingDirectories()
	return parsePsOutput(string(output), os.Getpid(), cwds)
}

func parsePsOutput(output string, selfPID int, cwds map[int]string) []DetectedProcess {
	processes := make([]DetectedProcess, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid == selfPID {
			continue
		}
		processes = append(processes, DetectedProcess{
			PID:              pid,
			Command:          strings.TrimSpace(parts[1]),
			WorkingDirectory: cwds[pid],
		})
	}
	return processes
}

func listWorkingDirectories() map[int]string {
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-Fpn").Output()
	if err != nil && len(out) == 0 {
		logger.Debug("lsof cwd listing failed", "err", err)
		return nil
	}
	return parseLsofCwd(string(out))
}

// parseLsofCwd parses `lsof -Fpn` output (p<pid> lines followed by n<path>).
func parseLsofCwd(output string) map[int]string {
	cwds := make(map[int]string)
	pid := 0
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "p"):
			pid, _ = strconv.Atoi(line[1:])
		case strings.HasPrefix(line, "n") && pid != 0:
			cwds[pid] = line[1:]
		}
	}
	return cwds
}
