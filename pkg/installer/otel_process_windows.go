//go:build windows

package installer

import (
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"golang.org/x/sys/windows"
)

// pythonChildPIDs returns the PIDs of direct child processes of parentPID
// whose exe name contains "python", using Get-CimInstance.
func pythonChildPIDs(parentPID int) ([]int, error) {
	lines, err := winProcessQuery(
		"$_.ParentProcessId -eq "+strconv.Itoa(parentPID)+" -and $_.Name -match 'python'",
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
	logger.Debug("pythonChildPIDs result", "parent_pid", parentPID, "child_pids", pids)
	return pids, nil
}

// adoptExeclChildren handles the Windows os.execl child-adoption pass.
// opentelemetry-instrument calls os.execl which on Windows is implemented as
// subprocess.Popen + sys.exit(0) — the launcher exits cleanly while the real
// app process runs as an orphaned child. This function finds and adopts those
// children so dtwiz can continue tracking them.
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
		logger.Debug("adoption: launcher exited cleanly, querying children", "name", p.Name, "pid", p.PID)
		pids, err := pythonChildPIDs(p.PID)
		if err != nil {
			logger.Debug("adoption: child query failed", "name", p.Name, "pid", p.PID, "err", err)
			continue
		}
		if len(pids) == 0 {
			logger.Debug("adoption: no python children found", "name", p.Name, "pid", p.PID)
			continue
		}
		// Pick the lowest PID (earliest spawned).
		childPID := pids[0]
		for _, pid := range pids[1:] {
			if pid < childPID {
				childPID = pid
			}
		}
		if len(pids) > 1 {
			logger.Debug("adoption: multiple children found, picking lowest PID", "name", p.Name, "candidates", pids, "selected", childPID)
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
			"name", p.Name, "launcher_pid", oldPID, "child_pid", childPID)
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
