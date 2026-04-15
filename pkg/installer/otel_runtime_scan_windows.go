//go:build windows

package installer

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// winProcessQuery runs a Get-CimInstance Win32_Process query on Windows.
// whereClause is the PowerShell Where-Object expression (without braces), e.g.
// "$_.CommandLine -match 'foo'". fieldsExpr is the ForEach-Object body that
// produces one line of output per matching process, e.g.
// "\"$($_.ProcessId)|$($_.CommandLine)\"".
// Returns the raw trimmed lines of output; blank lines are omitted.
func winProcessQuery(whereClause, fieldsExpr string) []string {
	out, err := exec.Command(
		"powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_Process | Where-Object { "+whereClause+" } | ForEach-Object { "+fieldsExpr+" }",
	).Output()
	if err != nil {
		return nil
	}
	return parseWinProcessOutput(string(out))
}

// detectProcesses lists running processes on Windows matching filterTerm in the
// command line, excluding those matching excludeTerms.
// Uses Get-CimInstance Win32_Process to query command line and working directory.
func detectProcesses(filterTerm string, excludeTerms []string) []DetectedProcess {
	logger.Debug("scanning processes via PowerShell", "filter", filterTerm)

	currentPID := os.Getpid()
	lowerFilter := strings.ToLower(filterTerm)

	lines := winProcessQuery(
		"$_.CommandLine -match '"+filterTerm+"'",
		"\"$($_.ProcessId)|$($_.CommandLine)|$($_.WorkingDirectory)\"",
	)
	if lines == nil {
		logger.Debug("detectProcesses: PowerShell query failed", "filter", filterTerm)
		return nil
	}

	var processes []DetectedProcess
	for _, line := range lines {
		row := strings.SplitN(line, "|", 3)
		if len(row) < 3 {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(row[0]))
		if err != nil || pid == currentPID {
			continue
		}

		command := strings.TrimSpace(row[1])
		if command == "" || !strings.Contains(strings.ToLower(command), lowerFilter) {
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
			logger.Debug("process excluded by term", "pid", pid, "terms", excludeTerms)
			continue
		}

		workingDir := strings.TrimSpace(row[2])
		logger.Debug("process matched", "pid", pid, "working_dir", workingDir)
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
