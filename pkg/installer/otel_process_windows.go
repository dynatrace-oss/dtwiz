//go:build windows

package installer

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"golang.org/x/sys/windows"
)

// pythonLeafPID finds the leaf python process matching the given entrypoint —
// i.e. the real app process, not the opentelemetry-instrument launcher.
// On Windows, opentelemetry-instrument spawns the real app as a child via
// subprocess.Popen then calls sys.exit(0), leaving a two-process chain
// (launcher → real app). The leaf is the process whose ProcessId does not
// appear as the ParentProcessId of any other matched process.
//
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
// opentelemetry-instrument on Windows spawns the real app via subprocess.Popen
// then calls sys.exit(0). The launcher exits cleanly (~500ms after start) while
// the real app runs as an orphaned python.exe process. By settle time the
// launcher is gone, so parent-PID queries find nothing. Instead we match by
// CommandLine via pythonLeafPID: we know the entrypoint path we launched, and
// it always appears in the CommandLine of the surviving python.exe process.
// The leaf is the python.exe whose PID does not appear as the ParentProcessId
// of any other matched process — i.e. the real app, not an intermediate launcher.
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
