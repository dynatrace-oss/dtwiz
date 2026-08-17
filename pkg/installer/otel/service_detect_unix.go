//go:build !windows

package otel

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// readProcCmdline reads the null-delimited argv from /proc/<pid>/cmdline on Linux,
// returning nil on other platforms or when the file is unreadable.
func readProcCmdline(pid int) []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return nil
	}
	// /proc/<pid>/cmdline is null-terminated; trailing null produces an empty element.
	parts := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(parts) == 0 {
		return nil
	}
	return parts
}

// detectServicesOnPorts returns processes that have active TCP connections
// TO any of the given ports (i.e. clients of the collector's receivers).
// Uses lsof(1), which is available on macOS and most Linux distributions.
func detectServicesOnPorts(ports []string) []connectedService {
	if len(ports) == 0 {
		return nil
	}

	portSet := make(map[string]bool, len(ports))
	for _, p := range ports {
		portSet[p] = true
	}

	// lsof -i TCP:<p1> [-i TCP:<p2>] -nP -Fn
	// No TCP state filter: HTTP OTLP exporters close the connection after each
	// batch, so -sTCP:ESTABLISHED would miss them between exports.  LISTEN
	// sockets are excluded below because they lack the "->" separator.
	// -n: no hostname resolution, -P: no port-name translation, -Fn: output fields
	args := make([]string, 0, len(ports)*2+3)
	for _, p := range ports {
		args = append(args, "-i", "TCP:"+p)
	}
	args = append(args, "-nP", "-Fn")

	out, err := exec.Command("lsof", args...).Output()
	if err != nil {
		// lsof exits 1 when no matching file descriptors are found — not a real error.
		logger.Debug("lsof found no connected services", "ports", ports, "err", err)
		return nil
	}
	logger.Debug("lsof raw output for connected-service scan", "out", string(out))

	// Parse lsof -Fn output.  Each process section starts with a "p<PID>" line
	// followed by file-descriptor lines starting with "f", "n", etc.
	// Network address lines start with "n": "n<local>-><remote>".
	seenPIDs := map[int]bool{}
	var orderedPIDs []int
	pidPort := map[int]string{} // first collector port seen per PID
	curPID := 0

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "p"):
			pid, err := strconv.Atoi(line[1:])
			if err == nil {
				curPID = pid
			}
		case strings.HasPrefix(line, "n") && curPID > 0:
			conn := line[1:] // e.g. "127.0.0.1:52341->127.0.0.1:4317"
			if !strings.Contains(conn, "->") {
				continue
			}
			parts := strings.SplitN(conn, "->", 2)
			localPort := portAfterLastColon(parts[0])
			remotePort := portAfterLastColon(parts[1])

			// Client connection: local port is ephemeral, remote port is a receiver port.
			if portSet[localPort] {
				continue // this is the collector's own connection entry
			}
			if !portSet[remotePort] {
				continue
			}
			if !seenPIDs[curPID] {
				seenPIDs[curPID] = true
				orderedPIDs = append(orderedPIDs, curPID)
			}
			if _, ok := pidPort[curPID]; !ok {
				pidPort[curPID] = remotePort
			}
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
			cmdline:       readProcCmdline(pid),
			workDir:       lookupProcessWorkingDirectory(pid),
			collectorPort: pidPort[pid],
			listenPorts:   detectListenPorts(pid),
			env:           readProcessEnv(pid),
		})
	}
	logger.Debug("detectServicesOnPorts", "ports", ports, "found", len(result))
	return result
}

// readProcessEnv returns the process environment via ps eww as "KEY=VAL" strings, or nil if unavailable.
func readProcessEnv(pid int) []string {
	out, err := exec.Command("ps", "eww", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return nil
	}
	return envSuffix(strings.TrimSpace(string(out)))
}

// detectInstrumentedServices finds OTel-instrumented processes exporting to the
// given tenant IDs or local collector ports, via a single ps axeww env scan.
func detectInstrumentedServices(tenantIDs, ports []string) []connectedService {
	if len(tenantIDs) == 0 && len(ports) == 0 {
		return nil
	}
	tenantSet := make(map[string]bool, len(tenantIDs))
	for _, t := range tenantIDs {
		tenantSet[t] = true
	}
	portSet := make(map[string]bool, len(ports))
	for _, p := range ports {
		portSet[p] = true
	}

	// BSD-style axe flags required: -A/-e silently drops the environment from output.
	out, err := exec.Command("ps", "axeww", "-o", "pid=,command=").Output()
	if err != nil {
		logger.Debug("ps env scan failed", "err", err)
		return nil
	}

	var result []connectedService
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, " ", 2)
		pid, err := strconv.Atoi(fields[0])
		if err != nil || len(fields) < 2 {
			continue
		}
		rest := fields[1]

		endpoint := otlpEndpointFromEnv(rest)
		if endpoint == "" {
			continue
		}
		if !endpointMatchesCollector(endpoint, tenantSet, portSet) {
			continue
		}

		cmd := stripEnvSuffix(rest)
		result = append(result, connectedService{
			pid:         pid,
			name:        serviceDisplayName(cmd),
			command:     cmd,
			cmdline:     readProcCmdline(pid),
			workDir:     lookupProcessWorkingDirectory(pid),
			listenPorts: detectListenPorts(pid),
			exportsTo:   endpoint,
			env:         envSuffix(rest),
		})
	}
	logger.Debug("detectInstrumentedServices", "tenants", tenantIDs, "found", len(result))
	return result
}

// detectListenPorts returns the TCP ports the process is listening on.
// -a ANDs the -p and -i filters; without it lsof ORs them and floods results.
func detectListenPorts(pid int) []string {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-i", "TCP", "-sTCP:LISTEN", "-nP", "-Fn").Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var ports []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		port := portAfterLastColon(line[1:])
		if port != "" && port != "*" && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	sort.Strings(ports)
	return ports
}

// portAfterLastColon extracts the port/service portion after the last colon in
// a network address string (e.g. "127.0.0.1:4317" → "4317").
func portAfterLastColon(addr string) string {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return ""
	}
	return addr[idx+1:]
}

// stopService sends SIGTERM, polls ~5s for exit, then escalates to SIGKILL.
func stopService(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil // already gone
		}
		return err
	}
	for range 50 { // ~5s
		if syscall.Kill(pid, 0) != nil {
			return nil // exited
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Still alive — force kill and give it a moment to release ports.
	_ = syscall.Kill(pid, syscall.SIGKILL)
	for range 20 { // ~2s
		if syscall.Kill(pid, 0) != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("process %d did not exit after SIGKILL", pid)
}

// relaunchService restarts the service detached (new session) from its captured
// command, workdir, and env; output goes to a log file in the workdir.
func relaunchService(svc connectedService) (int, error) {
	argv := svc.cmdline
	if len(argv) == 0 {
		// /proc/<pid>/cmdline unavailable (macOS or unreadable); ps-derived command
		// may misparse args that contain spaces (e.g. Java -D flags, paths with spaces).
		argv = strings.Fields(svc.command)
	}
	if len(argv) == 0 {
		return 0, fmt.Errorf("no command recorded")
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = svc.workDir
	if len(svc.env) > 0 {
		cmd.Env = svc.env
	}
	// New session: detach from dtwiz's process group and terminal.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if logFile := serviceLogFile(svc); logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Release so the child is reparented and not left as a zombie.
	_ = cmd.Process.Release()
	return pid, nil
}

// serviceLogFile opens a log file in the service's workDir (or temp dir), returning nil on failure.
func serviceLogFile(svc connectedService) *os.File {
	dir := svc.workDir
	if dir == "" {
		dir = os.TempDir()
	}
	safe := strings.NewReplacer("/", "-", " ", "-", string(os.PathSeparator), "-").Replace(svc.name)
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
