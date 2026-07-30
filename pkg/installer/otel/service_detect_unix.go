//go:build !windows

package otel

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// detectServicesOnPorts returns processes that have established TCP connections
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

	// lsof -i TCP:<p1> [-i TCP:<p2>] -sTCP:ESTABLISHED -nP -Fn
	// -n: no hostname resolution, -P: no port-name translation, -Fn: output fields
	args := make([]string, 0, len(ports)*2+4)
	for _, p := range ports {
		args = append(args, "-i", "TCP:"+p)
	}
	args = append(args, "-sTCP:ESTABLISHED", "-nP", "-Fn")

	out, err := exec.Command("lsof", args...).Output()
	if err != nil {
		// lsof exits 1 when no matching file descriptors are found — not a real error.
		logger.Debug("lsof found no connected services", "ports", ports, "err", err)
		return nil
	}

	// Parse lsof -Fn output.  Each process section starts with a "p<PID>" line
	// followed by file-descriptor lines starting with "f", "n", etc.
	// Network address lines start with "n": "n<local>-><remote>".
	seenPIDs := map[int]bool{}
	var orderedPIDs []int
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
		}
	}

	var result []connectedService
	for _, pid := range orderedPIDs {
		cmd := processFullArgs(pid)
		if cmd == "" {
			continue
		}
		result = append(result, connectedService{
			pid:     pid,
			name:    serviceDisplayName(cmd),
			command: cmd,
			workDir: lookupProcessWorkingDirectory(pid),
		})
	}
	logger.Debug("detectServicesOnPorts", "ports", ports, "found", len(result))
	return result
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

// terminateService sends SIGTERM to the process.  If the process is managed by
// a supervisor (systemd, launchd, etc.), the supervisor will restart it
// automatically; otherwise the process simply exits.
func terminateService(svc connectedService) error {
	return syscall.Kill(svc.pid, syscall.SIGTERM)
}
