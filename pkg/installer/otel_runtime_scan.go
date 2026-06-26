package installer

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
func walkCandidateDirs(root string, parentLevels int, visit func(dir string, entries []os.DirEntry) bool, shouldSkip func(name string) bool) {
	concurrency := max(runtime.NumCPU()*scanConcurrencyPerCPU, minScanConcurrency)
	// When full, child scans run synchronously instead of spawning more goroutines.
	sem := make(chan struct{}, concurrency)

	// Dedup across scanTree calls (symlinks and ancestor revisits).
	var queued sync.Map
	var wg sync.WaitGroup

	var scanDir func(dir string, anyFound *atomic.Bool)
	scanDir = func(dir string, anyFound *atomic.Bool) {
		defer wg.Done()

		entries, _ := os.ReadDir(dir)

		if visit(dir, entries) {
			if anyFound != nil {
				anyFound.Store(true)
			}
			return // matched: skip children
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
					scanDir(p, anyFound)
				}(childPath)
			default:
				// Pool saturated — run synchronously.
				wg.Add(1)
				scanDir(childPath, anyFound)
			}
		}
	}

	scanTree := func(dir string) bool {
		if _, loaded := queued.LoadOrStore(dir, struct{}{}); loaded {
			return false
		}
		var found atomic.Bool
		wg.Add(1)
		scanDir(dir, &found)
		wg.Wait()
		return found.Load()
	}

	scanTree(root)

	currentDir := root
	for range parentLevels {
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		logger.Debug("scanning ancestor dir", "path", parentDir)
		if scanTree(parentDir) {
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

func scanProjectDirs(markers []string, excludeNames []string) []ScannedProject {
	excludedDirNames := make(map[string]bool, len(excludeNames))
	for _, name := range excludeNames {
		excludedDirNames[name] = true
	}

	shouldSkipDir := func(name string) bool {
		return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") || excludedDirNames[name] || ignoredProjectDirNames[name]
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return nil
	}

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

		logger.Debug("project dir matched", "path", dir, "markers", strings.Join(matchedMarkers, ","))
		mu.Lock()
		discoveredProjects = append(discoveredProjects, ScannedProject{Path: dir, Markers: matchedMarkers})
		mu.Unlock()
		return true
	}

	walkCandidateDirs(workingDir, 2, dirMatches, shouldSkipDir)

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
	fmt.Printf("  Select a project to instrument [1-%d] or press Enter to skip: ", len(projects))
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
			err = killAndWaitProcess(process)
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
