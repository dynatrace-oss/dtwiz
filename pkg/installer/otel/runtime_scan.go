package otel

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type DetectedProcess struct {
	PID              int
	Command          string
	WorkingDirectory string
	Description      string
}

type ScannedProject struct {
	Path              string
	Markers           []string
	RunningProcessIDs []int
}

var ignoredProjectDirNames = map[string]bool{
	// Project build/dependency artifacts
	"node_modules": true,
	"vendor":       true,
	"venv":         true,
	".venv":        true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"out":          true,
	// macOS system directories
	"Library":      true,
	"Applications": true,
	"System":       true,
	"Movies":       true,
	"Music":        true,
	"Pictures":     true,
	"Public":       true,
	// Windows system directories — scanning these is slow and never productive
	"Windows":      true,
	"System32":     true,
	"SysWOW64":     true,
	"WinSxS":       true,
	"ProgramData":  true,
	"AppData":      true,
	"$Recycle.Bin": true,
}

func isIgnoredDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") || ignoredProjectDirNames[name]
}

func runInParallel[A any, B any](left func() A, right func() B) (A, B) {
	var leftResult A
	var rightResult B

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	go func() {
		defer waitGroup.Done()
		leftResult = left()
	}()

	go func() {
		defer waitGroup.Done()
		rightResult = right()
	}()

	waitGroup.Wait()
	return leftResult, rightResult
}

// Directory scanning is syscall-bound, so oversubscribe CPUs; the floor keeps
// 1–2 core VMs from going effectively serial.
const (
	scanConcurrencyPerCPU = 2
	minScanConcurrency    = 4
)

// walkCandidateDirs scans root + descendants in parallel and walks up to
// parentLevels ancestors. visit receives each dir's entries (read once, reused
// for matching and recursion); returning true means matched — skip children.
// The scan root itself is exempted from the skip-children rule: it's the
// user's cwd, not necessarily a project boundary, so a marker match there
// (e.g. a stray lockfile) must not hide every nested project underneath it.
func walkCandidateDirs(root string, parentLevels int, visit func(dir string, entries []os.DirEntry) bool, shouldSkip func(name string) bool) {
	concurrency := max(runtime.NumCPU()*scanConcurrencyPerCPU, minScanConcurrency)
	// When full, child scans run synchronously instead of spawning more goroutines.
	sem := make(chan struct{}, concurrency)

	// Dedup across scanTree calls (symlinks and ancestor revisits).
	var queued sync.Map
	var wg sync.WaitGroup

	var scanDir func(dir string, anyFound *atomic.Bool, isRoot bool)
	scanDir = func(dir string, anyFound *atomic.Bool, isRoot bool) {
		defer wg.Done()

		entries, _ := os.ReadDir(dir)

		matched := visit(dir, entries)
		if matched {
			if anyFound != nil {
				anyFound.Store(true)
			}
			if !isRoot {
				return // matched: skip children
			}
		}

		for _, entry := range entries {
			if !entry.IsDir() || shouldSkip(entry.Name()) {
				continue
			}
			childPath := filepath.Join(dir, entry.Name())
			if _, loaded := queued.LoadOrStore(childPath, struct{}{}); loaded {
				continue
			}
			select {
			case sem <- struct{}{}:
				wg.Add(1)
				go func(p string) {
					defer func() { <-sem }()
					scanDir(p, anyFound, false)
				}(childPath)
			default:
				// Pool saturated — run synchronously.
				wg.Add(1)
				scanDir(childPath, anyFound, false)
			}
		}
	}

	scanTree := func(dir string, isRoot bool) bool {
		if _, loaded := queued.LoadOrStore(dir, struct{}{}); loaded {
			return false
		}
		var found atomic.Bool
		wg.Add(1)
		scanDir(dir, &found, isRoot)
		wg.Wait()
		return found.Load()
	}

	scanTree(root, true)

	currentDir := root
	for range parentLevels {
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		logger.Debug("scanning ancestor dir", "path", parentDir)
		if scanTree(parentDir, false) {
			break
		}
		currentDir = parentDir
	}
}

// Trees this large are normal for system paths (C:\Windows, /usr) but unusual
// for project dirs — print a progress notice when crossed.
const largeScanThreshold = 10000

// Shared across runtimes so the progress notice prints at most once per session.
var globalScanCount atomic.Int64

// defaultScanRoots returns the working directory as the sole scan root. Used by
// callers that do not go through the interactive root selection (the per-runtime
// install commands and process-correlation paths), preserving cwd-only scanning.
func defaultScanRoots() []string {
	wd, err := os.Getwd()
	if err != nil {
		logger.Debug("could not determine working directory for scan", "error", err)
		return nil
	}
	return []string{wd}
}

type scanChoice int

const (
	scanChoiceBoth    scanChoice = iota // scan working directory + home
	scanChoiceCwdOnly                   // scan working directory only
	scanChoiceAbort                     // cancel the command
)

// selectScanRoots decides which directories the `install otel` scan should walk.
// When the working directory lies outside the home tree it prompts the user with
// a three-way choice; otherwise (same lineage as home) it scans the working
// directory only. Returns installer.ErrInstallCancelled when the user aborts.
func selectScanRoots() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determining working directory: %w", err)
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		logger.Debug("could not resolve home directory; scanning working directory only", "error", homeErr)
		return []string{cwd}, nil
	}

	// Same lineage as home (cwd == home, home under cwd, or cwd under home):
	// the working-directory walk is sufficient, so no prompt and no extra root.
	if pathsInSameLineage(cwd, home) {
		return []string{cwd}, nil
	}

	// Disjoint trees: offer to also scan home. Non-interactive runs default to
	// scanning both without blocking.
	if installer.AutoConfirm || !stdinIsTTY() {
		return []string{cwd, home}, nil
	}

	switch promptHomeScanChoice(cwd, home) {
	case scanChoiceCwdOnly:
		return []string{cwd}, nil
	case scanChoiceAbort:
		return nil, installer.ErrInstallCancelled
	default: // scanChoiceBoth
		return []string{cwd, home}, nil
	}
}

func promptHomeScanChoice(cwd, home string) scanChoice {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println()
		fmt.Printf("  You're running from %s, outside your home directory.\n", cwd)
		fmt.Printf("  Also scan your home directory (%s) for projects?\n", home)
		fmt.Println("  [Y] this directory and home  (default)")
		fmt.Println("  [c] this directory only")
		fmt.Println("  [n] cancel")
		fmt.Print("  Choose [Y/c/n]: ")

		answer, readErr := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "y", "yes":
			return scanChoiceBoth
		case "c":
			return scanChoiceCwdOnly
		case "n", "no":
			return scanChoiceAbort
		}
		if readErr != nil {
			return scanChoiceAbort
		}
		fmt.Println("  Please enter Y, c, or n.")
	}
}

// stdinIsTTY reports whether stdin is an interactive terminal. When it is not
// (piped input, CI), the scan prompt is skipped in favour of the default.
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// pathsInSameLineage reports whether a and b are equal or one is an ancestor of
// the other, after resolving symlinks. Comparison is segment-wise (via
// filepath.Rel), so "/home/foo" and "/home/foobar" are correctly unrelated.
func pathsInSameLineage(a, b string) bool {
	ra := resolvePath(a)
	rb := resolvePath(b)
	if ra == rb {
		return true
	}
	return isDescendant(ra, rb) || isDescendant(rb, ra)
}

func resolvePath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return filepath.Clean(p)
}

// isDescendant reports whether child is strictly nested under parent.
func isDescendant(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

func scanProjectDirs(markers []string, excludeNames []string, roots []string) []ScannedProject {
	if len(roots) == 0 {
		return nil
	}

	excludedDirNames := make(map[string]bool, len(excludeNames))
	for _, name := range excludeNames {
		excludedDirNames[name] = true
	}

	shouldSkipDir := func(name string) bool {
		return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") || excludedDirNames[name] || ignoredProjectDirNames[name]
	}

	// The first root is the working directory; progress and relative-path
	// bookkeeping key off it.
	workingDir := roots[0]

	markerSet := make(map[string]struct{}, len(markers))
	for _, m := range markers {
		markerSet[m] = struct{}{}
	}

	var mu sync.Mutex
	discoveredProjects := make([]ScannedProject, 0)
	// Dedup matches when the same physical dir is reached via different symlink
	// paths. Lazy: only resolved on actual match.
	var matchedProjects sync.Map // lowercased resolved path → struct{}
	var subtreeCounts sync.Map   // relative top-level child → *atomic.Int64

	dirMatches := func(dir string, entries []os.DirEntry) bool {
		if shouldSkipDir(filepath.Base(dir)) {
			return false
		}

		n := globalScanCount.Add(1)
		if n == largeScanThreshold {
			fmt.Printf("  Scanning %s, this may take a moment…\n", workingDir)
		} else if n == 2*largeScanThreshold {
			fmt.Printf("  Tip: run dtwiz from the directory where your code lives for a faster scan.\n")
		}

		if rel, relErr := filepath.Rel(workingDir, dir); relErr == nil && rel != "." {
			topChild := strings.SplitN(rel, string(filepath.Separator), 2)[0]
			val, _ := subtreeCounts.LoadOrStore(topChild, &atomic.Int64{})
			val.(*atomic.Int64).Add(1)
		}

		matchedMarkers := make([]string, 0, len(markers))
		for _, e := range entries {
			if _, ok := markerSet[e.Name()]; ok {
				matchedMarkers = append(matchedMarkers, e.Name())
			}
		}

		if len(matchedMarkers) == 0 {
			return false
		}

		// EvalSymlinks runs only on actual matches (rare), not every visited dir.
		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			resolvedDir = dir
		}
		if _, loaded := matchedProjects.LoadOrStore(strings.ToLower(resolvedDir), struct{}{}); loaded {
			return true // dup via symlink: skip children but don't re-record
		}

		logger.Debug("project dir matched", "path", resolvedDir, "markers", strings.Join(matchedMarkers, ","))
		mu.Lock()
		discoveredProjects = append(discoveredProjects, ScannedProject{Path: resolvedDir, Markers: matchedMarkers})
		mu.Unlock()
		return true
	}

	for _, root := range roots {
		walkCandidateDirs(root, 0, dirMatches, shouldSkipDir)
	}

	// Also scan the bundled examples root (~/.dtwiz/examples) if it exists.
	if home, err := os.UserHomeDir(); err == nil {
		bundledRoot := filepath.Join(home, ".dtwiz", "examples")
		if _, err := os.Stat(bundledRoot); err == nil {
			walkCandidateDirs(bundledRoot, 0, dirMatches, shouldSkipDir)
		}
	}

	subtreeCounts.Range(func(key, value any) bool {
		logger.Debug("scan summary", "subdir", key.(string), "dirs_checked", value.(*atomic.Int64).Load())
		return true
	})
	logger.Debug("scan complete", "total_dirs_checked", globalScanCount.Load())

	return discoveredProjects
}

func matchingProcessIDs(dirPath string, processes []DetectedProcess) []int {
	normalizedPath := strings.ToLower(dirPath)
	matchedPIDs := make([]int, 0)
	for _, process := range processes {
		workingDir := strings.ToLower(process.WorkingDirectory)
		command := strings.ToLower(process.Command)
		if strings.HasPrefix(workingDir, normalizedPath) || strings.Contains(command, normalizedPath) {
			matchedPIDs = append(matchedPIDs, process.PID)
		}
	}
	return matchedPIDs
}

func matchProcessesToProjects(projects []ScannedProject, processes []DetectedProcess) {
	for i := range projects {
		projects[i].RunningProcessIDs = matchingProcessIDs(projects[i].Path, processes)
	}
}

func promptProjectSelection(label string, projects []ScannedProject) *ScannedProject {
	fmt.Println()
	display.ColorHeader.Printf("  %s projects on this machine:\n", label)
	display.PrintSectionDivider()
	for i, project := range projects {
		line := fmt.Sprintf("  [%d]  %s  (%s)", i+1, project.Path, strings.Join(project.Markers, ", "))
		if len(project.RunningProcessIDs) > 0 {
			pidStrings := make([]string, len(project.RunningProcessIDs))
			for j, pid := range project.RunningProcessIDs {
				pidStrings[j] = strconv.Itoa(pid)
			}
			line += fmt.Sprintf("  ← PIDs: %s", strings.Join(pidStrings, ", "))
		}
		fmt.Println(line)
	}
	fmt.Println()
	rangeHint := fmt.Sprintf("1-%d", len(projects))
	if len(projects) == 1 {
		rangeHint = "1"
	}
	fmt.Printf("  Select a project to instrument [%s] or press Enter to skip: ", rangeHint)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" {
		logger.Debug("user skipped project selection")
		return nil
	}

	selection, err := strconv.Atoi(answer)
	if err != nil || selection < 1 || selection > len(projects) {
		logger.Debug("invalid project selection, skipping", "input", answer)
		fmt.Println("  Invalid selection, skipping instrumentation.")
		return nil
	}
	logger.Debug("user selected project", "path", projects[selection-1].Path)
	return &projects[selection-1]
}

func stopProcesses(pids []int) {
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		// On Unix send SIGINT for graceful shutdown; on Windows use
		// killAndWaitProcess which calls Kill() and polls until the PID is gone.
		if runtime.GOOS == "windows" {
			err = installer.KillAndWaitProcess(process)
		} else {
			err = process.Signal(os.Interrupt)
			if err == nil {
				// proc.Wait() only works for child processes; for orphaned
				// processes (started by a previous dtwiz run) we poll via
				// kill(pid,0) until the OS reports the PID is gone.
				if !waitForProcessDeath(pid, 30*time.Second) {
					// Graceful shutdown timed out — escalate to SIGKILL.
					_ = process.Kill()
					waitForProcessDeath(pid, 5*time.Second)
				}
			}
		}
		if err != nil {
			fmt.Printf("    Warning: could not stop PID %d: %v\n", pid, err)
			continue
		}
		fmt.Printf("    Stopped PID %d\n", pid)
	}
}

// detectProcessOrChildListeningPort checks pid first, then its direct children.
// Needed for frameworks (e.g. Nuxt/Nitro) that may spawn a cluster worker to hold
// the TCP socket while the parent process acts as a manager.
func detectProcessOrChildListeningPort(pid int) string {
	if port := detectProcessListeningPort(pid); port != "" {
		return port
	}
	return detectChildListeningPort(pid)
}

// parseWinProcessOutput splits raw Windows process output into non-empty lines, stripping trailing CR characters from CRLF line endings.
func parseWinProcessOutput(raw string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
