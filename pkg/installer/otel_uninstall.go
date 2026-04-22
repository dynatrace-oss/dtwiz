package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// otelProcessInfo holds PID + resolved binary path for a running collector.
type otelProcessInfo struct {
	pid        int
	binaryPath string
	installDir string
}

// findRunningOtelProcesses returns detailed info for every running
// dynatrace-otel-collector process, including its install directory.
func findRunningOtelProcesses() []otelProcessInfo {
	procs := findRunningOtelCollectors()
	var infos []otelProcessInfo
	for _, rc := range procs {
		binPath := rc.path
		if binPath == "" {
			binPath = binaryPathFromPID(rc.pid)
		}
		installDir := ""
		if binPath != "" {
			installDir = filepath.Dir(binPath)
		}
		logger.Debug("running OTel Collector process", "pid", rc.pid, "binary", binPath, "installDir", installDir)
		infos = append(infos, otelProcessInfo{
			pid:        rc.pid,
			binaryPath: binPath,
			installDir: installDir,
		})
	}
	return infos
}

// binaryPathFromPID returns the executable path of a process (first word of
// its command line), or empty string when it cannot be determined.
func binaryPathFromPID(pid int) string {
	pidStr := strconv.Itoa(pid)
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"ProcessId=%s\").ExecutablePath", pidStr)).Output()
	} else {
		out, err = exec.Command("ps", "-p", pidStr, "-o", "args=").Output()
	}
	if err != nil {
		return ""
	}
	result := strings.TrimSpace(string(out))
	if runtime.GOOS == "windows" {
		// PowerShell returns the full path as a single line.
		if result == "" {
			return ""
		}
		return result
	}
	fields := strings.Fields(result)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// candidateOtelDirs returns a deduplicated list of directories that look like
// they were created by dtwiz's OTel Collector installer:
//   - install dirs derived from running process binary paths
//   - ~/opentelemetry  (default when dtwiz was run from $HOME)
//   - ./opentelemetry  (default when dtwiz was run from CWD)
func candidateOtelDirs(infos []otelProcessInfo) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(d string) {
		if d == "" || seen[d] {
			return
		}
		// Only include directories that actually exist on disk.
		if _, err := os.Stat(d); err == nil {
			logger.Debug("candidate OTel install dir found", "dir", d)
			seen[d] = true
			dirs = append(dirs, d)
		} else {
			logger.Debug("candidate OTel install dir not present", "dir", d)
		}
	}

	for _, info := range infos {
		add(info.installDir)
	}

	// Well-known default locations dtwiz uses.
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, "opentelemetry"))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, "opentelemetry"))
	}

	return dirs
}

// killCollectorProcesses kills every process in procs, prints status lines, and
// returns the binary path of the first successfully-killed process (useful for
// restarting). Non-fatal errors are printed as warnings.
func killCollectorProcesses(procs []otelProcessInfo) string {
	var restartBinary string
	for _, p := range procs {
		proc, err := os.FindProcess(p.pid)
		if err != nil {
			fmt.Printf("  Warning: could not find process %d: %v\n", p.pid, err)
			continue
		}
		if err := killAndWaitProcess(proc); err != nil {
			fmt.Printf("  Warning: could not kill process %d: %v\n", p.pid, err)
			continue
		}
		fmt.Printf("  Stopped collector (PID %d).\n", p.pid)
		if restartBinary == "" && p.binaryPath != "" {
			restartBinary = p.binaryPath
		}
	}
	return restartBinary
}

// removeWithRetry attempts os.RemoveAll, retrying a few times with a short
// delay. On Windows the OS may briefly hold file locks after a process is
// killed, causing the first attempt to fail with "Access is denied".
func removeWithRetry(path string) error {
	const maxAttempts = 5
	const delay = 500 * time.Millisecond

	var err error
	for i := range maxAttempts {
		if err = os.RemoveAll(path); err == nil {
			return nil
		}
		if i < maxAttempts-1 {
			logger.Debug("RemoveAll failed, retrying", "path", path, "attempt", i+1, "err", err)
			time.Sleep(delay)
		}
	}
	return err
}

// findInstrumentedPythonProcesses returns running Python processes that are
// likely OTel-instrumented, using the same filter as install-time detection.
func findInstrumentedPythonProcesses() []DetectedProcess {
	procs := detectProcesses("python", []string{"pip ", "setup.py", "/bin/dtwiz"})
	for _, p := range procs {
		logger.Debug("instrumented python process found", "pid", p.PID, "command", p.Command)
	}
	logger.Debug("python process scan complete", "matched", len(procs))
	return procs
}

// UninstallOtelCollector kills all running Dynatrace OTel Collector processes
// and removes the installation directories created by dtwiz.
func UninstallOtelCollector(dryRun bool) error {
	header := color.New(color.FgMagenta, color.Bold)
	muted := color.New()
	red := color.New(color.FgRed)

	processes := findRunningOtelProcesses()
	dirs := candidateOtelDirs(processes)
	pythonProcs := findInstrumentedPythonProcesses()

	// ── Preview ──────────────────────────────────────────────────────────────
	header.Println("  Dynatrace OTel Collector Uninstall")
	muted.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	if len(processes) == 0 && len(dirs) == 0 && len(pythonProcs) == 0 {
		muted.Println("  Nothing to remove — no running collector and no install directories found.")
		return nil
	}

	if len(processes) > 0 {
		fmt.Println("  Processes that will be killed:")
		for _, p := range processes {
			hint := ""
			if p.binaryPath != "" {
				hint = "  (" + p.binaryPath + ")"
			}
			fmt.Printf("    ")
			red.Printf("kill PID %d", p.pid)
			muted.Printf("%s\n", hint)
		}
		fmt.Println()
	} else {
		muted.Println("  No running collector processes found.")
		fmt.Println()
	}

	if len(dirs) > 0 {
		fmt.Println("  Directories that will be removed:")
		for _, d := range dirs {
			fmt.Printf("    ")
			red.Printf("rm -rf %s\n", d)
		}
		fmt.Println()
	} else {
		muted.Println("  No installation directories found.")
		fmt.Println()
	}

	if len(pythonProcs) > 0 {
		fmt.Println("  Instrumented Python processes that will be stopped:")
		for _, p := range pythonProcs {
			fmt.Printf("    ")
			red.Printf("kill PID %d", p.PID)
			muted.Printf("  (%s)\n", p.Command)
		}
		fmt.Println()
	} else {
		muted.Println("  No instrumented Python processes found.")
		fmt.Println()
	}

	muted.Println("  " + strings.Repeat("─", 50))

	if dryRun {
		muted.Println("  [dry-run] No changes made.")
		return nil
	}

	// ── Confirmation ─────────────────────────────────────────────────────────
	ok, err := confirmProceed("  Proceed with uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		muted.Println("  Uninstall cancelled.")
		return nil
	}
	fmt.Println()

	// ── Kill processes ───────────────────────────────────────────────────────
	killCollectorProcesses(processes)

	// ── Stop instrumented Python processes ───────────────────────────────────
	if len(pythonProcs) > 0 {
		pids := make([]int, len(pythonProcs))
		for i, p := range pythonProcs {
			pids[i] = p.PID
		}
		stopProcesses(pids)
	}

	// ── Remove directories ───────────────────────────────────────────────────
	for _, d := range dirs {
		if err := removeWithRetry(d); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", d, err)
			continue
		}
		fmt.Printf("  Removed %s\n", d)
	}

	fmt.Println()
	color.New(color.FgGreen, color.Bold).Println("  ✓ OTel Collector uninstalled.")
	return nil
}
