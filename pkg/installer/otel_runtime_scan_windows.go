//go:build windows

package installer

import (
	"encoding/csv"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// detectProcesses lists running processes on Windows using Get-CimInstance Win32_Process
// and filters by filterTerm in the command line, excluding those matching excludeTerms.
func detectProcesses(filterTerm string, excludeTerms []string) []DetectedProcess {
	output, err := exec.Command(
		"powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_Process | Select-Object ProcessId,CommandLine,WorkingDirectory | ConvertTo-Csv -NoTypeInformation",
	).Output()
	if err != nil {
		logger.Warn("Get-CimInstance failed", "filter", filterTerm, "err", err)
		return nil
	}
	logger.Debug("scanning processes", "filter", filterTerm)

	csvReader := csv.NewReader(strings.NewReader(string(output)))
	records, err := csvReader.ReadAll()
	if err != nil || len(records) < 2 {
		logger.Debug("Get-CimInstance CSV parse failed or empty", "filter", filterTerm, "err", err, "rows", len(records))
		return nil
	}

	processes := make([]DetectedProcess, 0)
	currentPID := os.Getpid()
	// records[0] is the header row; data starts at records[1]
	for _, record := range records[1:] {
		if len(record) < 3 {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(record[0]))
		if err != nil || pid == currentPID {
			continue
		}

		command := strings.TrimSpace(record[1])
		if command == "" {
			continue
		}

		if !strings.Contains(strings.ToLower(command), strings.ToLower(filterTerm)) {
			continue
		}

		excluded := false
		for _, excludeTerm := range excludeTerms {
			if strings.Contains(strings.ToLower(command), strings.ToLower(excludeTerm)) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		workingDir := strings.TrimSpace(record[2])
		processes = append(processes, DetectedProcess{
			PID:              pid,
			Command:          command,
			WorkingDirectory: workingDir,
		})
	}
	logger.Debug("process scan complete", "filter", filterTerm, "matched", len(processes))
	return processes
}

// lookupProcessWorkingDirectory returns the working directory of a process on Windows
// by querying Win32_Process via Get-CimInstance.
func lookupProcessWorkingDirectory(pid int) string {
	output, err := exec.Command(
		"powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_Process -Filter \"ProcessId="+strconv.Itoa(pid)+"\" | Select-Object -ExpandProperty WorkingDirectory",
	).Output()
	if err != nil {
		logger.Warn("Get-CimInstance WorkingDirectory lookup failed", "pid", pid, "err", err)
		return ""
	}
	return strings.TrimSpace(string(output))
}

// detectProcessListeningPort returns the first non-OTel TCP port a process is listening on,
// using Get-NetTCPConnection (available on Windows Server 2012 R2+ / Windows 8.1+).
func detectProcessListeningPort(pid int) string {
	output, err := exec.Command(
		"powershell", "-NoProfile", "-Command",
		"Get-NetTCPConnection -State Listen -OwningProcess "+strconv.Itoa(pid)+
			" -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -notin @(4317,4318) } | Select-Object -First 1 -ExpandProperty LocalPort",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
