package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type otelProcessInfo struct {
	pid        int
	binaryPath string
	installDir string
	command    string
	workingDir string
}

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

func javaAgentDir() string {
	agentPath, err := javaAgentPath()
	if err != nil {
		return ""
	}
	return filepath.Dir(agentPath)
}

// detectJavaProcessesFunc and enrichProcessesWithJPSFunc are package-level
// variables to allow overriding in tests.
var detectJavaProcessesFunc = detectJavaProcesses
var enrichProcessesWithJPSFunc = enrichProcessesWithJPS

// findInstrumentedJavaProcesses returns running Java processes that were
// started with dtwiz Java auto-instrumentation. It checks both the command
// line (for direct -javaagent usage) and open file descriptors (for
// JAVA_TOOL_OPTIONS injection, e.g. Gradle bootRun).
func findInstrumentedJavaProcesses() []DetectedProcess {
	agentPath, err := javaAgentPath()
	if err != nil {
		logger.Debug("could not resolve java agent path for process filter", "err", err)
		return []DetectedProcess{}
	}
	processes := detectJavaProcessesFunc()
	processes = enrichProcessesWithJPSFunc(processes)
	var instrumented []DetectedProcess
	for _, p := range processes {
		if strings.Contains(p.Command, agentPath) || jvmHasAgentLoaded(p.PID, agentPath) {
			instrumented = append(instrumented, p)
		}
	}
	logger.Debug("findInstrumentedJavaProcesses", "total", len(processes), "instrumented", len(instrumented))
	return instrumented
}

func candidateOtelDirs(infos []otelProcessInfo) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(d string) {
		if d == "" || seen[d] {
			return
		}
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

	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, "opentelemetry"))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, "opentelemetry"))
	}

	return dirs
}

func killCollectorProcesses(procs []otelProcessInfo) string {
	var restartBinary string
	for _, p := range procs {
		if p.pid <= 0 {
			continue // installed but not running — nothing to kill
		}
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

// findNodeOtelDirs scans CWD (recursively) and parent directories for .otel/
// directories that contain a package.json with @opentelemetry in its content —
// these are directories created by dtwiz's Node.js auto-instrumentation
// installer. The scan mirrors scanProjectDirs: CWD + children, then up to 2
// ancestor levels.
func findNodeOtelDirs() []string {
	var dirs []string
	seen := map[string]bool{}

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	// checkDir tests whether dir contains a .otel/ child that is a valid
	// Node.js OTel directory. Returns true if found (and appends to dirs).
	// Deduplication uses the symlink-resolved path so that /tmp/.otel and
	// /private/tmp/.otel (same directory on macOS) are not listed twice.
	// mu guards dirs and seen since walkCandidateDirs calls this concurrently.
	var mu sync.Mutex
	checkDir := func(dir string, entries []os.DirEntry) bool {
		// Use entries (already read by walkCandidateDirs) to check for .otel/
		// without an extra Stat syscall.
		hasOtel := false
		for _, e := range entries {
			if e.Name() == ".otel" && e.IsDir() {
				hasOtel = true
				break
			}
		}
		if !hasOtel {
			return false
		}
		otelDir := filepath.Join(dir, ".otel")
		key := otelDir
		if resolved, err := filepath.EvalSymlinks(otelDir); err == nil {
			key = resolved
		}
		mu.Lock()
		if seen[key] {
			mu.Unlock()
			return false
		}
		seen[key] = true
		mu.Unlock()
		if isNodeOtelDir(otelDir) {
			logger.Debug("found Node.js .otel/ directory", "dir", otelDir)
			mu.Lock()
			dirs = append(dirs, otelDir)
			mu.Unlock()
			return true
		}
		return false
	}

	// walkCandidateDirs recursively checks dir and its children (skipping the
	// same ignored directories as scanProjectDirs).
	walkCandidateDirs(cwd, 2, func(dir string, entries []os.DirEntry) bool {
		checkDir(dir, entries)
		return false
	}, isIgnoredDir)

	return dirs
}

// isNodeOtelDir checks if a directory is a dtwiz-created Node.js OTel
// instrumentation directory by verifying it contains a package.json
// with @opentelemetry in its content.
func isNodeOtelDir(dir string) bool {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "@opentelemetry")
}

func printCollectorUninstallPreview(processes []otelProcessInfo, dirs []string) {
	if len(processes) == 0 && len(dirs) == 0 {
		return
	}
	display.Header("  OTel Collector")
	fmt.Println()
	if len(processes) > 0 {
		fmt.Println("  Processes that will be killed:")
		for _, p := range processes {
			hint := ""
			if p.binaryPath != "" {
				hint = "  (" + p.binaryPath + ")"
			}
			fmt.Printf("    ")
			display.ColorError.Printf("kill PID %d", p.pid)
			display.ColorMuted.Printf("%s\n", hint)
		}
		fmt.Println()
	} else {
		display.ColorMuted.Println("  No running collector processes found.")
		fmt.Println()
	}

	if len(dirs) > 0 {
		fmt.Println("  Directories that will be removed:")
		for _, d := range dirs {
			fmt.Printf("    ")
			display.ColorError.Printf("rm -rf %s\n", d)
		}
		fmt.Println()
	} else {
		display.ColorMuted.Println("  No installation directories found.")
		fmt.Println()
	}
}

// collectorToProcessInfo converts a collectorInstance to the otelProcessInfo type
// used by the kill/remove helpers.
func collectorToProcessInfo(c collectorInstance) otelProcessInfo {
	installDir := ""
	if c.binaryPath != "" {
		installDir = filepath.Dir(c.binaryPath)
	}
	return otelProcessInfo{
		pid:        c.pid,
		binaryPath: c.binaryPath,
		installDir: installDir,
	}
}

// UninstallOtelCollector shows a list of Dynatrace OTel Collector instances,
// lets the user select which to remove, then kills the selected process(es) and
// deletes their installation directories.  It also removes dtwiz-installed
// runtime instrumentation artifacts (Node.js, Python, Java).
func UninstallOtelCollector(dryRun bool) error {
	display.Header("Dynatrace OTel Collector Uninstall")

	// Find all Dynatrace OTel Collectors and let the user choose which to uninstall.
	dtCollectors := findDynatraceOtelCollectors()
	var processes []otelProcessInfo

	if len(dtCollectors) > 0 {
		display.ColorMessage.Println("  Collectors available for uninstall:")
		selected, err := selectCollectorForUninstall(dtCollectors)
		if err != nil {
			display.ColorDefault.Println("  Uninstall cancelled.")
			return ErrInstallCancelled
		}
		for _, c := range selected {
			processes = append(processes, collectorToProcessInfo(c))
		}
	}

	// Collector install directories derived from the selected processes.
	dirs := candidateOtelDirs(processes)

	// Node.js .otel/ directory artifacts.
	nodeOtelDirs := findNodeOtelDirs()

	logger.Debug("uninstall scan complete", "collectorProcesses", len(processes), "collectorDirs", len(dirs), "nodeOtelDirs", len(nodeOtelDirs))

	type runtimeResult struct {
		label string
		procs []DetectedProcess
	}
	var runtimeResults []runtimeResult
	anyRuntimeProcs := false
	for _, c := range runtimeCleaners {
		procs := c.DetectProcesses()
		// Treat nil as an error condition and skip this runtime.
		if procs == nil {
			logger.Debug("runtime process scan failed (skipped)", "runtime", c.Label())
			continue
		}
		for _, p := range procs {
			logger.Debug("instrumented process found", "runtime", c.Label(), "pid", p.PID, "command", p.Command)
		}
		logger.Debug("runtime process scan complete", "runtime", c.Label(), "matched", len(procs))
		runtimeResults = append(runtimeResults, runtimeResult{c.Label(), procs})
		if len(procs) > 0 {
			anyRuntimeProcs = true
		}
	}

	javaProcs := findInstrumentedJavaProcesses()
	agentDir := javaAgentDir()
	agentDirExists := false
	if agentDir != "" {
		if _, err := os.Stat(agentDir); err == nil {
			agentDirExists = true
		}
	}
	hasJavaCleanup := len(javaProcs) > 0 || agentDirExists

	if len(processes) == 0 && len(dirs) == 0 && !anyRuntimeProcs && !hasJavaCleanup && len(nodeOtelDirs) == 0 {
		display.ColorDefault.Println("  Nothing to remove — no Dynatrace OTel Collectors or instrumentation artifacts found.")
		return nil
	}

	printCollectorUninstallPreview(processes, dirs)

	for _, r := range runtimeResults {
		if len(r.procs) > 0 {
			fmt.Printf("  Instrumented %s processes that will be stopped:\n", r.label)
			for _, p := range r.procs {
				fmt.Printf("    ")
				display.ColorError.Printf("stop PID %d", p.PID)
				display.ColorDefault.Printf("  (%s)\n", p.Command)
			}
			fmt.Println()
		}
	}

	if hasJavaCleanup {
		if len(javaProcs) > 0 {
			fmt.Println("  Instrumented Java processes that will be stopped:")
			for _, p := range javaProcs {
				desc := p.Command
				if p.Description != "" {
					desc = p.Description
				}
				fmt.Printf("    ")
				display.ColorError.Printf("stop PID %d", p.PID)
				display.ColorDefault.Printf("  (%s)\n", desc)
			}
			fmt.Println()
		}
		if agentDirExists {
			fmt.Println("  Java agent directory that will be removed:")
			fmt.Printf("    ")
			display.ColorError.Printf("rm -rf %s\n", agentDir)
			fmt.Println()
		}
	}

	display.PrintSectionDivider()

	if len(nodeOtelDirs) > 0 {
		fmt.Println("  .otel/ directories that will be removed:")
		for _, d := range nodeOtelDirs {
			fmt.Printf("    ")
			display.ColorError.Printf("rm -rf %s\n", d)
		}
		fmt.Println()
	}

	display.ColorMuted.Println("  " + strings.Repeat("─", 50))

	if dryRun {
		display.ColorDefault.Println("  [dry-run] No changes made.")
		return nil
	}

	ok, err := confirmProceed("  Proceed with uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		display.ColorDefault.Println("  Uninstall cancelled.")
		return nil
	}
	fmt.Println()

	killCollectorProcesses(processes)

	for _, r := range runtimeResults {
		if len(r.procs) == 0 {
			continue
		}
		pids := make([]int, len(r.procs))
		for i, p := range r.procs {
			pids[i] = p.PID
		}
		logger.Debug("stopping runtime processes", "runtime", r.label, "count", len(pids))
		stopProcesses(pids)
	}

	if len(javaProcs) > 0 {
		javaPIDs := make([]int, len(javaProcs))
		for i, p := range javaProcs {
			javaPIDs[i] = p.PID
		}
		logger.Debug("stopping instrumented java processes", "count", len(javaPIDs))
		stopProcesses(javaPIDs)
	}

	for _, d := range dirs {
		if err := removeWithRetry(d); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", d, err)
			continue
		}
		fmt.Printf("  Removed %s\n", d)
	}

	for _, d := range nodeOtelDirs {
		if err := removeWithRetry(d); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", d, err)
			continue
		}
		fmt.Printf("  Removed %s\n", d)
	}
	if agentDirExists {
		if err := removeWithRetry(agentDir); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", agentDir, err)
		} else {
			fmt.Printf("  Removed %s\n", agentDir)
		}
	}

	fmt.Println()
	display.ColorOK.Println("  ✓ OTel Collector uninstalled.")
	return nil
}
