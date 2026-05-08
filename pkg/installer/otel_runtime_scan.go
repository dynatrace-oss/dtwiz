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
	"Library":      true,
	"Applications": true,
	"System":       true,
	"Movies":       true,
	"Music":        true,
	"Pictures":     true,
	"Public":       true,
	".git":         true,
	".svn":         true,
	"node_modules": true,
	"vendor":       true,
	"venv":         true,
	".venv":        true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"out":          true,
}

func isIgnoredDir(name string) bool {
	return strings.HasPrefix(name, ".") || ignoredProjectDirNames[name]
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

// walkCandidateDirs scans root and its descendants, then walks up to
// parentLevels ancestor directories doing the same scan. visit is called
// for every candidate directory; returning true means "matched — skip
// children". shouldSkip decides whether a child entry name is skipped
// entirely. The parent walk stops as soon as a level produces a match.
func walkCandidateDirs(root string, parentLevels int, visit func(dir string) bool, shouldSkip func(name string) bool) {
	var scan func(dir string) bool
	scan = func(dir string) bool {
		found := visit(dir)
		if found {
			return true
		}
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if !entry.IsDir() || shouldSkip(entry.Name()) {
				continue
			}
			if scan(filepath.Join(dir, entry.Name())) {
				found = true
			}
		}
		return found
	}

	scan(root)

	currentDir := root
	for range parentLevels {
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		logger.Debug("scanning ancestor dir", "path", parentDir)
		if scan(parentDir) {
			break
		}
		currentDir = parentDir
	}
}

func scanProjectDirs(markers []string, excludeNames []string) []ScannedProject {
	excludedDirNames := make(map[string]bool, len(excludeNames))
	for _, name := range excludeNames {
		excludedDirNames[name] = true
	}

	shouldSkipDir := func(name string) bool {
		return strings.HasPrefix(name, ".") || excludedDirNames[name] || ignoredProjectDirNames[name]
	}

	discoveredProjects := make([]ScannedProject, 0)
	visitedDirs := make(map[string]bool) // present=visited, value=matched

	dirMatches := func(dir string) bool {
		if shouldSkipDir(filepath.Base(dir)) {
			logger.Debug("skipping ignored dir", "path", dir)
			return false
		}

		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			resolvedDir = dir
		}

		normalizedDir := strings.ToLower(resolvedDir)
		if matched, seen := visitedDirs[normalizedDir]; seen {
			return matched
		}

		matchedMarkers := make([]string, 0, len(markers))
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				matchedMarkers = append(matchedMarkers, marker)
			}
		}

		if len(matchedMarkers) == 0 {
			logger.Debug("project dir scanned, no markers", "path", dir, "looking_for", strings.Join(markers, ","))
			visitedDirs[normalizedDir] = false
			return false
		}

		logger.Debug("project dir matched", "path", dir, "markers", strings.Join(matchedMarkers, ","))
		discoveredProjects = append(discoveredProjects, ScannedProject{Path: dir, Markers: matchedMarkers})
		visitedDirs[normalizedDir] = true
		return true
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return discoveredProjects
	}

	walkCandidateDirs(workingDir, 2, dirMatches, shouldSkipDir)

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

// stripping trailing CR characters (\r\n line endings).
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
