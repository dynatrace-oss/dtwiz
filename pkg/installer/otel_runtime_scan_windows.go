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
// whereClause is the PowerShell Where-Object expression and fieldsExpr
// is the ForEach-Object body that produces one line per matching process.
func winProcessQuery(whereClause, fieldsExpr string) ([]string, error) {
	script := "Get-CimInstance Win32_Process | Where-Object { " + whereClause + " } | ForEach-Object { " + fieldsExpr + " }"
	logger.Debug("winProcessQuery: executing", "where", whereClause, "fields", fieldsExpr)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debug("winProcessQuery: PowerShell invocation failed",
			"where", whereClause,
			"err", err,
			"output", strings.TrimSpace(string(out)),
		)
		return nil, err
	}
	lines := parseWinProcessOutput(string(out))
	logger.Debug("winProcessQuery: success", "where", whereClause, "result_count", len(lines))
	return lines, nil
}

// detectProcesses lists running processes on Windows matching filterTerm in the
// command line, excluding those matching excludeTerms.
// Uses Get-CimInstance Win32_Process to query command line and working directory.
func detectProcesses(filterTerm string, excludeTerms []string) []DetectedProcess {
	logger.Debug("scanning processes via PowerShell", "filter", filterTerm)

	currentPID := os.Getpid()
	lowerFilter := strings.ToLower(filterTerm)

	lines, err := winProcessQuery(
		"$_.CommandLine -match '"+filterTerm+"'",
		"\"$($_.ProcessId)|$($_.CommandLine)|$($_.WorkingDirectory)\"",
	)
	if err != nil {
		logger.Debug("detectProcesses: PowerShell query failed", "filter", filterTerm, "err", err)
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

// detectProcessListeningPort returns the first non-OTel TCP port a process is
// listening on, using Get-NetTCPConnection (Windows Server 2012 R2+ / Win 8.1+).
//
// Get-NetTCPConnection -OwningProcess exits with code 1 when the process has no
// connections even with -ErrorAction SilentlyContinue, so we read output
// regardless of the exit code and treat non-empty output as success.
func detectProcessListeningPort(pid int) string {
	cmd := exec.Command(
		"powershell", "-NoProfile", "-Command",
		"Get-NetTCPConnection -State Listen -OwningProcess "+strconv.Itoa(pid)+
			" -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -notin @(4317,4318) } | Select-Object -First 1 -ExpandProperty LocalPort",
	)
	output, err := cmd.Output()
	port := strings.TrimSpace(string(output))
	if port == "" {
		if err != nil {
			logger.Debug("detectProcessListeningPort: query failed", "pid", pid, "err", err)
		}
		return ""
	}
	logger.Debug("detectProcessListeningPort: found port", "pid", pid, "port", port)
	return port
}

// detectJavaPortByCommandLine finds the listening port of a Java process whose
// Win32_Process.CommandLine contains projectDir. The caller injects
// -Ddtwiz.project=<projectDir> into the launched command (via
// spring-boot.run.jvmArguments for Maven, MAVEN_OPTS for in-process goals),
// so the marker is guaranteed to be in the command line of the right JVM.
// Matching is done in Go to avoid PowerShell escaping issues with Windows paths.
func detectJavaPortByCommandLine(projectDir string) string {
	script := `
$procs = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue
$javaPids = @($procs | Where-Object Name -like 'java*' | ForEach-Object ProcessId)
if ($javaPids) {
    Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
        Where-Object { $javaPids -contains $_.OwningProcess -and $_.LocalPort -notin @(4317,4318) } |
        ForEach-Object {
            $cmd = ($procs | Where-Object ProcessId -eq $_.OwningProcess | Select-Object -First 1).CommandLine
            "$($_.OwningProcess)|$($_.LocalPort)|$cmd"
        }
}`
	logger.Debug("detectJavaPortByCommandLine: scanning", "project_dir", projectDir)
	output, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil && len(output) == 0 {
		logger.Debug("detectJavaPortByCommandLine: query failed", "project_dir", projectDir, "err", err)
		return ""
	}

	projLower := strings.ToLower(projectDir)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimRight(line, "\r")
		// format: <pid>|<port>|<cmdline>  — SplitN(3) keeps | inside cmdline intact
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		pid, port, cmd := parts[0], parts[1], parts[2]
		logCmd := cmd
		if len(logCmd) > 200 {
			logCmd = logCmd[:200] + "…"
		}
		logger.Debug("detectJavaPortByCommandLine: listening java process", "pid", pid, "port", port, "cmd", logCmd)
		if strings.Contains(strings.ToLower(cmd), projLower) {
			logger.Debug("detectJavaPortByCommandLine: matched", "pid", pid, "port", port)
			return port
		}
	}

	logger.Debug("detectJavaPortByCommandLine: no match", "project_dir", projectDir)
	return ""
}
