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

// javaDescendantPort finds the listening port of an app JVM spawned by a
// build-tool wrapper (mvn spring-boot:run, gradlew bootRun). Uses jps -l to
// enumerate JVMs, skips build-tool JVMs, and prefers one whose working
// directory is under projectDir.
func javaDescendantPort(pid int, projectDir string) string {
	out, err := exec.Command("jps", "-l").Output()
	if err != nil {
		logger.Debug("jps -l failed", "err", err)
		return ""
	}

	type jvmCandidate struct {
		pid   int
		class string
		cwd   string
	}
	var candidates []jvmCandidate
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		jvmPID, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		if jvmPID == pid {
			// Tracked PID is itself an app JVM — it hasn't bound its port yet.
			// Return "" so the poll loop retries direct detection.
			if !isBuildToolJVM(fields[1]) {
				logger.Debug("tracked pid is app JVM, waiting for it to bind", "pid", pid, "class", fields[1])
				return ""
			}
			continue
		}
		if isBuildToolJVM(fields[1]) {
			continue
		}
		cwd := lookupProcessWorkingDirectory(jvmPID)
		candidates = append(candidates, jvmCandidate{pid: jvmPID, class: fields[1], cwd: cwd})
	}

	// First pass: prefer JVMs whose working directory is under projectDir.
	for _, c := range candidates {
		if !isUnderDir(c.cwd, projectDir) {
			continue
		}
		if port := detectProcessListeningPort(c.pid); port != "" {
			logger.Debug("javaDescendantPort: port found (cwd match)", "wrapper_pid", pid, "jvm_pid", c.pid, "port", port)
			return port
		}
	}

	// Second pass: any eligible JVM with an open port.
	for _, c := range candidates {
		if port := detectProcessListeningPort(c.pid); port != "" {
			logger.Debug("javaDescendantPort: port found (no cwd match)", "wrapper_pid", pid, "jvm_pid", c.pid, "port", port)
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
