//go:build windows

package otel

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

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
		})
	}
	logger.Debug("detectServicesOnPorts", "ports", ports, "found", len(result))
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

// terminateService terminates the process using taskkill.
func terminateService(svc connectedService) error {
	out, err := exec.Command("taskkill", "/PID", strconv.Itoa(svc.pid), "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
