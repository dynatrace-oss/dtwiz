//go:build windows

package installer

import (
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"golang.org/x/sys/windows"
)

// pythonPIDsByCommandLine returns the PIDs of running python processes whose
// CommandLine contains the given entrypoint string, using Get-CimInstance.
func pythonPIDsByCommandLine(entrypoint string) ([]int, error) {
	// Escape backslashes for PowerShell -match (regex).
	escaped := strings.ReplaceAll(entrypoint, `\`, `\\`)
	lines, err := winProcessQuery(
		"$_.Name -match 'python' -and $_.CommandLine -match '"+escaped+"'",
		"$_.ProcessId",
	)
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, s := range lines {
		pid, err := strconv.Atoi(strings.TrimSpace(s))
		if err == nil {
			pids = append(pids, pid)
		}
	}
	logger.Debug("pythonPIDsByCommandLine result", "entrypoint", entrypoint, "pids", pids)
	return pids, nil
}

// adoptExeclChildren handles the Windows os.execl child-adoption pass.
// opentelemetry-instrument on Windows spawns the real app via subprocess.Popen
// then calls sys.exit(0). The launcher exits cleanly (~500ms after start) while
// the real app runs as an orphaned python.exe process. By settle time the
// launcher is gone, so parent-PID queries find nothing. Instead we match by
// CommandLine: we know the entrypoint path we launched, and it always appears
// in the CommandLine of the surviving python.exe process(es). We pick the
// lowest matching PID (first spawned) and adopt it.
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
		if p.Entrypoint == "" {
			logger.Debug("adoption: no entrypoint recorded, skipping", "name", p.Name, "pid", p.PID)
			continue
		}
		pids, err := pythonPIDsByCommandLine(p.Entrypoint)
		if err != nil {
			logger.Debug("adoption: CommandLine query failed", "name", p.Name, "entrypoint", p.Entrypoint, "err", err)
			continue
		}
		if len(pids) == 0 {
			logger.Debug("adoption: no running python process matched entrypoint", "name", p.Name, "entrypoint", p.Entrypoint)
			continue
		}
		childPID := pids[0]
		for _, pid := range pids[1:] {
			if pid < childPID {
				childPID = pid
			}
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
// in a goroutine, sending the result (always nil — we can't get an exit code
// this way) to a buffered channel of capacity 1. This mirrors the pattern used
// by StartManagedProcess for cmd.Wait().
//
// If OpenProcess fails (e.g. the process belongs to a different user or has
// already exited), an actionable debug message is logged and nil is sent so
// the caller sees the process as gone — no regression from current behaviour.
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
