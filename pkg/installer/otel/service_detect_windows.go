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
			collectorPort: pidPort[pid],
			listenPorts:   detectListenPorts(pid),
			env:           readProcessEnv(pid),
		})
	}
	logger.Debug("detectServicesOnPorts", "ports", ports, "found", len(result))
	return result
}

// detectInstrumentedServices is a no-op on Windows; process env requires elevated ReadProcessMemory.
func detectInstrumentedServices(_, _ []string) []connectedService {
	logger.Debug("detectInstrumentedServices not supported on windows")
	return nil
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
