//go:build windows

package otel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// windowsDetachedProcess: DETACHED_PROCESS (0x8) | CREATE_NEW_PROCESS_GROUP (0x200).
const windowsDetachedProcess = 0x00000008 | 0x00000200

// detectServicesOnPorts returns processes that have established TCP connections
// TO any of the given ports.  Uses netstat -ano and PowerShell for process names.
func detectServicesOnPorts(ports []string) []connectedService {
	if len(ports) == 0 {
		return nil
	}

	portSet := make(map[string]bool, len(ports))
	for _, p := range ports {
		portSet[p] = true
	}

	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		logger.Debug("netstat failed", "err", err)
		return nil
	}

	// netstat -ano output (Windows):
	// Proto  Local Address      Foreign Address    State        PID
	// TCP    127.0.0.1:52341    127.0.0.1:4317     ESTABLISHED  12301

	seenPIDs := map[int]bool{}
	var orderedPIDs []int
	pidPort := map[int]string{} // first collector port seen per PID

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 {
			continue
		}
		// Accept any active state (ESTABLISHED, TIME_WAIT, CLOSE_WAIT, etc.) so
		// HTTP OTLP exporters that close the connection after each batch are still
		// detected.  Skip LISTENING — those are the collector's own sockets.
		if fields[0] != "TCP" || fields[3] == "LISTENING" {
			continue
		}
		remoteAddr := fields[2]
		remotePort := portAfterLastColon(remoteAddr)
		if !portSet[remotePort] {
			continue
		}
		localAddr := fields[1]
		localPort := portAfterLastColon(localAddr)
		if portSet[localPort] {
			continue // collector's own accepted connection entry
		}
		pid, err := strconv.Atoi(fields[4])
		if err != nil || pid <= 0 {
			continue
		}
		if !seenPIDs[pid] {
			seenPIDs[pid] = true
			orderedPIDs = append(orderedPIDs, pid)
		}
		if _, ok := pidPort[pid]; !ok {
			pidPort[pid] = remotePort
		}
	}

	var result []connectedService
	for _, pid := range orderedPIDs {
		cmd := processFullArgs(pid)
		if cmd == "" {
			continue
		}
		result = append(result, connectedService{
			pid:           pid,
			name:          serviceDisplayName(cmd),
			command:       cmd,
			workDir:       lookupProcessWorkingDirectory(pid),
			collectorPort: pidPort[pid],
			listenPorts:   detectListenPorts(pid),
			env:           readProcessEnv(pid),
		})
	}
	logger.Debug("detectServicesOnPorts", "ports", ports, "found", len(result))
	return result
}

// detectInstrumentedServices finds OTel-instrumented processes on Windows via
// command-line pattern matching. Process environment blocks require elevated
// ReadProcessMemory so env vars cannot be checked; instead we match the
// command-line tokens that dtwiz and common OTel launchers produce:
//   - Java agent:   -javaagent:...opentelemetry-javaagent
//   - Python:       opentelemetry-instrument
//   - Node.js:      @opentelemetry/auto-instrumentations-node/register
//
// The tenantIDs and ports filter parameters are ignored because we cannot
// read process environments; ALL OTel-instrumented processes are returned.
// Their env field is nil; restartConnectedServices uses os.Environ() as the
// base and overrides OTEL_EXPORTER_OTLP_ENDPOINT to the local collector.
func detectInstrumentedServices(_, _ []string) []connectedService {
	currentPID := os.Getpid()
	currentPIDStr := strconv.Itoa(currentPID)

	otelPatterns := []string{
		"opentelemetry-javaagent",
		"opentelemetry-instrument",
		"opentelemetry/auto-instrumentations-node/register",
	}

	seen := map[int]bool{}
	var result []connectedService

	for _, pattern := range otelPatterns {
		lines, err := winProcessQuery(
			// Exclude dtwiz's own powershell.exe detection processes: their CommandLine
			// contains both the search pattern AND 'Get-CimInstance', so they would
			// otherwise match and be returned as instrumented services.
			"$_.CommandLine -match '"+pattern+"' -and $_.ProcessId -ne "+currentPIDStr+
				" -and $_.CommandLine -notmatch 'Get-CimInstance'",
			// WorkingDirectory before CommandLine: CommandLine may contain "|" which
			// would break SplitN(3) if it appeared before the last field.
			"\"$($_.ProcessId)|$($_.WorkingDirectory)|$($_.CommandLine)\"",
		)
		if err != nil {
			logger.Debug("detectInstrumentedServices: query failed", "pattern", pattern, "err", err)
			continue
		}
		for _, line := range lines {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) < 3 {
				continue
			}
			pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil || pid == currentPID || seen[pid] {
				continue
			}
			seen[pid] = true
			workDir := strings.TrimSpace(parts[1])
			command := strings.TrimSpace(parts[2])
			result = append(result, connectedService{
				pid:     pid,
				name:    serviceDisplayName(command),
				command: command,
				workDir: workDir,
				// env is nil — process environment requires elevated access on Windows
			})
			logger.Debug("detectInstrumentedServices: found", "pid", pid, "pattern", pattern)
		}
	}

	logger.Debug("detectInstrumentedServices (Windows)", "found", len(result))
	return result
}

// detectListenPorts returns the TCP ports that the given process is listening on.
func detectListenPorts(pid int) []string {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return nil
	}
	pidStr := strconv.Itoa(pid)
	seen := map[string]bool{}
	var ports []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 {
			continue
		}
		if fields[0] != "TCP" || fields[3] != "LISTENING" || fields[4] != pidStr {
			continue
		}
		port := portAfterLastColon(fields[1])
		if port != "" && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	sort.Strings(ports)
	return ports
}

// portAfterLastColon extracts the port portion after the last colon.
func portAfterLastColon(addr string) string {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return ""
	}
	return addr[idx+1:]
}

// readProcessEnv is not implemented on Windows; relaunched services inherit dtwiz's environment.
func readProcessEnv(_ int) []string {
	return nil
}

// stopService terminates the process and its child tree using taskkill.
func stopService(pid int) error {
	out, err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(outStr), "not found") {
			return nil // already gone
		}
		return fmt.Errorf("%w: %s", err, outStr)
	}
	return nil
}

// relaunchService restarts the service detached (windowsDetachedProcess) from its
// captured command and workdir; env cannot be captured so the child inherits dtwiz's.
func relaunchService(svc connectedService) (int, error) {
	// Windows CommandLine is a single string; strings.Fields may misparse
	// space-containing args (quoted paths, -D flags). Full fix requires
	// CommandLineToArgvW, which is not yet implemented.
	argv := strings.Fields(svc.command)
	if len(argv) == 0 {
		return 0, fmt.Errorf("no command recorded")
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = svc.workDir
	if cmd.Dir == "" {
		// Windows CreateProcess with DETACHED_PROCESS requires an explicit
		// working directory; it cannot inherit the parent's CWD when Dir is "".
		if wd, err := os.Getwd(); err == nil {
			cmd.Dir = wd
		}
	}
	if len(svc.env) > 0 {
		cmd.Env = svc.env
	}
	// Detach from dtwiz's console so the child keeps running after exit.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windowsDetachedProcess}

	if logFile := serviceLogFile(svc); logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

// serviceLogFile opens a log file in the service's workDir (or temp dir), returning nil on failure.
func serviceLogFile(svc connectedService) *os.File {
	dir := svc.workDir
	if dir == "" {
		dir = os.TempDir()
	}
	safe := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(svc.name)
	if safe == "" {
		safe = strconv.Itoa(svc.pid)
	}
	path := filepath.Join(dir, "dtwiz-"+safe+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		logger.Debug("service log file open failed", "path", path, "err", err)
		return nil
	}
	return f
}
