//go:build windows

package installer

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"golang.org/x/sys/windows"
)

// pythonLeafPID finds the leaf python process matching the given entrypoint.
// Returns 0, nil if no matching processes are found.
func pythonLeafPID(entrypoint string) (int, error) {
	escaped := strings.ReplaceAll(entrypoint, `\`, `\\`)
	script := `$m = Get-CimInstance Win32_Process | Where-Object { $_.Name -match 'python' -and $_.CommandLine -match '` + escaped + `' }; ($m | Where-Object { $_.ProcessId -notin @($m.ParentProcessId) } | Select-Object -First 1).ProcessId`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		logger.Debug("pythonLeafPID: no matching process found", "entrypoint", entrypoint)
		return 0, nil
	}
	logger.Debug("pythonLeafPID: leaf found", "entrypoint", entrypoint, "pid", pid)
	return pid, nil
}

// adoptExeclChildren handles the Windows os.execl child-adoption pass.
// For processes that exited cleanly, attempts to find and adopt the surviving
// Python child process by matching its CommandLine entrypoint.
func adoptExeclChildren(procs []*ManagedProcess, started, notStarted *int) {
	for _, p := range procs {
		exited, waitErr := p.WaitResult()
		if !exited {
			logger.Debug("adoption: process still running, skipping", "name", p.Name, "pid", p.PID)
			continue
		}
		if waitErr != nil {
			logger.Debug("adoption: process crashed, skipping", "name", p.Name, "pid", p.PID, "err", waitErr)
			continue
		}
		if p.Entrypoint == "" || !p.IsExeclLauncher {
			logger.Debug("adoption: not an execl launcher, skipping", "name", p.Name, "pid", p.PID)
			continue
		}
		childPID, err := pythonLeafPID(p.Entrypoint)
		if err != nil {
			logger.Debug("adoption: CommandLine query failed", "name", p.Name, "entrypoint", p.Entrypoint, "err", err)
			continue
		}
		if childPID == 0 {
			logger.Debug("adoption: no running python process matched entrypoint", "name", p.Name, "entrypoint", p.Entrypoint)
			continue
		}
		oldPID := p.PID
		p.PID = childPID
		p.hasExited = false
		p.cachedWaitErr = nil
		p.resultConsumed = false
		p.exitResultCh = watchPID(childPID)
		*started++
		*notStarted--
		logger.Debug("adoption: adopted windows child process",
			"name", p.Name, "launcher_pid", oldPID, "child_pid", childPID, "entrypoint", p.Entrypoint)
	}
}

// watchPID opens the process with SYNCHRONIZE access and waits for it to exit
// in a goroutine, sending the result to a buffered channel.
// If OpenProcess fails, logs a debug message and sends nil to the channel.
func watchPID(pid int) chan error {
	ch := make(chan error, 1)
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		logger.Debug("windows watchPID: OpenProcess failed — if the Python process belongs to a different user, run dtwiz as the same user that owns the Python process",
			"pid", pid, "err", err)
		ch <- nil
		return ch
	}

	go func() {
		defer windows.CloseHandle(handle)
		_, err := windows.WaitForSingleObject(handle, windows.INFINITE)
		if err != nil {
			logger.Debug("windows watchPID: WaitForSingleObject error", "pid", pid, "err", err)
		} else {
			logger.Debug("windows watchPID: process exited", "pid", pid)
		}
		ch <- nil
	}()
	return ch
}
