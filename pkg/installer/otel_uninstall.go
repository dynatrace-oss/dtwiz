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

type otelProcessInfo struct {
	pid        int
	binaryPath string
	installDir string
}

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
	checkDir := func(dir string) bool {
		otelDir := filepath.Join(dir, ".otel")
		// Only bother with dedup and validation if .otel/ actually exists.
		if _, err := os.Stat(otelDir); err != nil {
			return false
		}
		key := otelDir
		if resolved, err := filepath.EvalSymlinks(otelDir); err == nil {
			key = resolved
		}
		if seen[key] {
			return false
		}
		seen[key] = true
		if isNodeOtelDir(otelDir) {
			logger.Debug("found Node.js .otel/ directory", "dir", otelDir)
			dirs = append(dirs, otelDir)
			return true
		}
		return false
	}

	// scanChildren recursively checks dir and its children (skipping the
	// same ignored directories as scanProjectDirs).
	var scanChildren func(dir string)
	scanChildren = func(dir string) {
		checkDir(dir) // check this directory itself

		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Skip the usual ignored directories, but note: .otel/ is
			// handled by checkDir above — we never recurse INTO .otel/.
			if isIgnoredDir(name) {
				continue
			}
			scanChildren(filepath.Join(dir, entry.Name()))
		}
	}

	// 1. Scan CWD and its children.
	scanChildren(cwd)

	// 2. Walk up to 2 parent levels (same as scanProjectDirs).
	currentDir := cwd
	for range 2 {
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		before := len(dirs)
		scanChildren(parentDir)
		if len(dirs) > before {
			break // found something at this level, stop climbing
		}
		currentDir = parentDir
	}

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

// UninstallOtelCollector kills all running Dynatrace OTel Collector processes
// and removes the installation directories created by dtwiz. It also detects
// and removes Node.js OTel instrumentation artifacts (.otel/ directories and
// instrumented Node.js processes).
func UninstallOtelCollector(dryRun bool) error {
	header := color.New(color.FgMagenta, color.Bold)
	muted := color.New()
	red := color.New(color.FgRed)

	// Collector artifacts.
	processes := findRunningOtelProcesses()
	dirs := candidateOtelDirs(processes)

	// Node.js .otel/ directory artifacts.
	nodeOtelDirs := findNodeOtelDirs()

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

	// ── Preview ──────────────────────────────────────────────────────────────
	header.Println("  Dynatrace OTel Uninstall")
	muted.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	if len(processes) == 0 && len(dirs) == 0 && !anyRuntimeProcs && len(nodeOtelDirs) == 0 {
		muted.Println("  Nothing to remove — no running collector, no install directories, and no Node.js instrumentation found.")
		return nil
	}

	// ── Collector section ────────────────────────────────────────────────────
	if len(processes) > 0 || len(dirs) > 0 {
		header.Println("  OTel Collector")
		fmt.Println()
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
	}

	for _, r := range runtimeResults {
		if len(r.procs) > 0 {
			fmt.Printf("  Instrumented %s processes that will be stopped:\n", r.label)
			for _, p := range r.procs {
				fmt.Printf("    ")
				red.Printf("stop PID %d", p.PID)
				muted.Printf("  (%s)\n", p.Command)
			}
			fmt.Println()
		}
	}

	if len(nodeOtelDirs) > 0 {
		fmt.Println("  .otel/ directories that will be removed:")
		for _, d := range nodeOtelDirs {
			fmt.Printf("    ")
			red.Printf("rm -rf %s\n", d)
		}
		fmt.Println()
	}

	muted.Println("  " + strings.Repeat("─", 50))

	if dryRun {
		muted.Println("  [dry-run] No changes made.")
		return nil
	}

	ok, err := confirmProceed("  Proceed with uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		muted.Println("  Uninstall cancelled.")
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

	for _, d := range dirs {
		if err := removeWithRetry(d); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", d, err)
			continue
		}
		fmt.Printf("  Removed %s\n", d)
	}

	// ── Remove .otel/ directories ───────────────────────────────────────────
	for _, d := range nodeOtelDirs {
		if err := removeWithRetry(d); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", d, err)
			continue
		}
		fmt.Printf("  Removed %s\n", d)
	}

	fmt.Println()
	color.New(color.FgGreen, color.Bold).Println("  ✓ OTel uninstalled.")
	return nil
}
