//go:build !windows

package installer

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// DetectProcesses is the exported version of detectProcesses for use by other packages.
func DetectProcesses(filterTerm string, excludeTerms []string) []DetectedProcess {
	return detectProcesses(filterTerm, excludeTerms)
}

// DetectProcessListeningPort is the exported version of detectProcessListeningPort for use by other packages.
func DetectProcessListeningPort(pid int) string {
	return detectProcessListeningPort(pid)
}

func detectProcesses(filterTerm string, excludeTerms []string) []DetectedProcess {
	output, err := exec.Command("ps", "ax", "-o", "pid=,command=").Output()
	if err != nil {
		logger.Warn("ps command failed", "filter", filterTerm, "err", err)
		return nil
	}
	logger.Debug("scanning processes", "filter", filterTerm)

	processes := make([]DetectedProcess, 0)
	currentPID := os.Getpid()
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid == currentPID {
			continue
		}

		command := strings.TrimSpace(parts[1])
		lowerCommand := strings.ToLower(command)
		if !strings.Contains(lowerCommand, strings.ToLower(filterTerm)) {
			continue
		}

		excluded := false
		for _, excludeTerm := range excludeTerms {
			if strings.Contains(lowerCommand, strings.ToLower(excludeTerm)) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		processes = append(processes, DetectedProcess{
			PID:              pid,
			Command:          command,
			WorkingDirectory: lookupProcessWorkingDirectory(pid),
		})
	}
	logger.Debug("process scan complete", "filter", filterTerm, "matched", len(processes))
	return processes
}

func lookupProcessWorkingDirectory(pid int) string {
	output, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strconv.Itoa(pid), "-Fn").Output()
	if err != nil {
		logger.Warn("lsof cwd lookup failed", "pid", pid, "err", err)
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}

func detectProcessListeningPort(pid int) string {
	output, err := exec.Command("lsof", "-a", "-i", "TCP", "-sTCP:LISTEN", "-p", strconv.Itoa(pid), "-Fn", "-P").Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		separator := strings.LastIndex(line, ":")
		if separator < 0 {
			continue
		}
		port := line[separator+1:]
		if port != "4317" && port != "4318" {
			return port
		}
	}
	return ""
}
