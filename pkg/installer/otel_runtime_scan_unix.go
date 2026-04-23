//go:build !windows

package installer

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var otelEnvVarMarkers = []string{
	"OTEL_SERVICE_NAME",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
}

// isOtelProcess reports whether the process with the given PID is an
// OTel-instrumented process, determined by checking for OTel env vars.
//
// On Linux it reads /proc/<pid>/environ (null-delimited).
// On macOS it uses "ps eww -p <pid>" which emits env vars inline with the command.
func isOtelProcess(pid int) bool {
	if runtime.GOOS == "linux" {
		return linuxProcessHasOtelEnvVars(pid)
	}
	return macosProcessHasOtelEnvVars(pid)
}

func linuxProcessHasOtelEnvVars(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		logger.Debug("could not read /proc/environ", "pid", pid, "err", err)
		return false
	}
	// /proc/<pid>/environ is null-delimited key=value pairs.
	for _, entry := range strings.Split(string(data), "\x00") {
		for _, marker := range otelEnvVarMarkers {
			if strings.HasPrefix(entry, marker+"=") {
				return true
			}
		}
	}
	return false
}

func macosProcessHasOtelEnvVars(pid int) bool {
	out, err := exec.Command("ps", "eww", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		logger.Debug("ps eww failed", "pid", pid, "err", err)
		return false
	}
	output := string(out)
	for _, marker := range otelEnvVarMarkers {
		if strings.Contains(output, marker+"=") {
			return true
		}
	}
	return false
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
	lowerFilter := strings.ToLower(filterTerm)
	lowerExcludeTerms := make([]string, 0, len(excludeTerms))
	for _, excludeTerm := range excludeTerms {
		lowerExcludeTerms = append(lowerExcludeTerms, strings.ToLower(excludeTerm))
	}
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
		if !strings.Contains(lowerCommand, lowerFilter) {
			continue
		}

		excluded := false
		for _, excludeTerm := range lowerExcludeTerms {
			if strings.Contains(lowerCommand, excludeTerm) {
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
