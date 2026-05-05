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

// javaDescendantPort finds the listening port of a java.exe process that is a
// descendant of wrapperPID. This handles Maven/Gradle wrappers (cmd /c mvn.cmd)
// which spawn a java.exe child that owns the actual listening port.
// Walks up to 4 levels of the process tree from wrapperPID, collects all
// java.exe descendants, then returns the first listening port found among them.
func javaDescendantPort(wrapperPID int, _ string) string {
	pidStr := strconv.Itoa(wrapperPID)
	script := `
$all = Get-CimInstance Win32_Process -Property ProcessId,ParentProcessId,Name -ErrorAction SilentlyContinue
$pids = @(` + pidStr + `)
1..4 | ForEach-Object {
    $children = @($all | Where-Object { $pids -contains $_.ParentProcessId -and $_.ProcessId -notin $pids })
    if ($children.Count -eq 0) { return }
    $pids = $pids + @($children | ForEach-Object { $_.ProcessId })
}
$javaPids = @($all | Where-Object { $pids -contains $_.ProcessId -and $_.Name -match '^java' -and $_.ProcessId -ne ` + pidStr + ` } | ForEach-Object { $_.ProcessId })
if ($javaPids) {
    Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
        Where-Object { $javaPids -contains $_.OwningProcess -and $_.LocalPort -notin @(4317,4318) } |
        Select-Object -First 1 -ExpandProperty LocalPort
}`
	logger.Debug("javaDescendantPort: scanning", "wrapper_pid", wrapperPID)
	output, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	port := strings.TrimSpace(string(output))
	if port == "" {
		if err != nil {
			logger.Debug("javaDescendantPort: query failed", "wrapper_pid", wrapperPID, "err", err)
		} else {
			logger.Debug("javaDescendantPort: no descendant with port found", "wrapper_pid", wrapperPID)
		}
		return ""
	}
	logger.Debug("javaDescendantPort: found", "wrapper_pid", wrapperPID, "port", port)
	return port
}

// jvmHasAgentLoaded reports whether a JVM process has loaded the given agent JAR.
func jvmHasAgentLoaded(pid int, agentJAR string) bool {
	script := `Get-Process -Id ` + strconv.Itoa(pid) + ` -ErrorAction SilentlyContinue | ForEach-Object { $_.Modules } | Where-Object { $_.FileName -eq '` + strings.ReplaceAll(agentJAR, "'", "''") + `' } | Select-Object -First 1`
	output, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}
