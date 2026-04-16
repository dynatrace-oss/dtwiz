package installer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/fatih/color"
)

type DetectedProcess struct {
	PID              int
	Command          string
	WorkingDirectory string
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

	var scanChildDirs func(dir string) int
	scanChildDirs = func(dir string) int {
		initialCount := len(discoveredProjects)
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if !entry.IsDir() || shouldSkipDir(entry.Name()) {
				continue
			}
			childDir := filepath.Join(dir, entry.Name())
			if !dirMatches(childDir) {
				scanChildDirs(childDir)
			}
		}
		return len(discoveredProjects) - initialCount
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return discoveredProjects
	}

	if !dirMatches(workingDir) {
		scanChildDirs(workingDir)
	}

	currentDir := workingDir
	for range 2 {
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		logger.Debug("scanning ancestor dir", "path", parentDir)
		if scanChildDirs(parentDir) > 0 {
			break
		}
		currentDir = parentDir
	}

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
	header := color.New(color.FgMagenta)
	fmt.Println()
	header.Printf("  %s projects on this machine:\n", label)
	fmt.Println("  " + strings.Repeat("─", 50))
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
		return nil
	}

	selection, err := strconv.Atoi(answer)
	if err != nil || selection < 1 || selection > len(projects) {
		fmt.Println("  Invalid selection, skipping instrumentation.")
		return nil
	}
	return &projects[selection-1]
}

func stopProcesses(pids []int) {
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := process.Signal(os.Interrupt); err != nil {
			fmt.Printf("    Warning: could not stop PID %d: %v\n", pid, err)
			continue
		}
		_, _ = process.Wait()
		fmt.Printf("    Stopped PID %d\n", pid)
	}
}

// parseWinProcessOutput splits raw PowerShell output into non-blank lines,
// stripping the trailing CR that PowerShell adds on Windows (\r\n line endings).
// It is defined here (not in the windows-only file) so it can be unit-tested on
// all platforms without a real PowerShell invocation.
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
