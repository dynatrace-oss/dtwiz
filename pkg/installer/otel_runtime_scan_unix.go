//go:build !windows

package installer

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

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
		logger.Debug("lsof cwd lookup failed", "pid", pid, "err", err)
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}

// javaDescendantPort finds the listening port of an app JVM spawned by a
// build-tool wrapper (mvn spring-boot:run, gradlew bootRun). Uses jps -l to
// enumerate JVMs, filters out build-tool processes, and checks each candidate
// for a listening port.
func javaDescendantPort(pid int, projectDir string) string {
	out, err := exec.Command("jps", "-l").Output()
	if err != nil {
		logger.Debug("jps -l failed", "err", err)
		return ""
	}

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		jvmPID, err := strconv.Atoi(fields[0])
		if err != nil || jvmPID == pid {
			continue
		}
		if isBuildToolJVM(fields[1]) {
			continue
		}
		if projectDir != "" && !isUnderDir(fields[1], projectDir) {
			continue
		}
		if port := detectProcessListeningPort(jvmPID); port != "" {
			logger.Debug("javaDescendantPort: port found", "wrapper_pid", pid, "jvm_pid", jvmPID, "class", fields[1], "port", port)
			return port
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

// jvmHasAgentLoaded reports whether a JVM process has loaded the given agent JAR.
// It uses lsof to check open file descriptors, catching cases where the agent
// is injected via JAVA_TOOL_OPTIONS and doesn't appear in the command line.
func jvmHasAgentLoaded(pid int, agentJAR string) bool {
	output, err := exec.Command("lsof", "-p", strconv.Itoa(pid), "-Fn").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "n") && strings.Contains(line, agentJAR) {
			return true
		}
	}
	return false
}
