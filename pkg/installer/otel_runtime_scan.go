package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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
	visitedDirs := make(map[string]bool)

	inspectDir := func(dir string) {
		if shouldSkipDir(filepath.Base(dir)) {
			return
		}

		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			resolvedDir = dir
		}

		normalizedDir := strings.ToLower(resolvedDir)
		if visitedDirs[normalizedDir] {
			return
		}
		visitedDirs[normalizedDir] = true

		matchedMarkers := make([]string, 0, len(markers))
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				matchedMarkers = append(matchedMarkers, marker)
			}
		}

		if len(matchedMarkers) == 0 {
			logger.Debug("project dir scanned, no markers", "path", dir, "looking_for", strings.Join(markers, ","))
			return
		}

		logger.Debug("project dir matched", "path", dir, "markers", strings.Join(matchedMarkers, ","))
		discoveredProjects = append(discoveredProjects, ScannedProject{Path: dir, Markers: matchedMarkers})
	}

	scanChildDirs := func(dir string) int {
		initialCount := len(discoveredProjects)
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if !entry.IsDir() || shouldSkipDir(entry.Name()) {
				continue
			}

			childDir := filepath.Join(dir, entry.Name())
			beforeChild := len(discoveredProjects)
			inspectDir(childDir)
			if len(discoveredProjects) != beforeChild {
				continue
			}

			grandchildren, _ := os.ReadDir(childDir)
			for _, grandchild := range grandchildren {
				if grandchild.IsDir() && !shouldSkipDir(grandchild.Name()) {
					inspectDir(filepath.Join(childDir, grandchild.Name()))
				}
			}
		}
		return len(discoveredProjects) - initialCount
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return discoveredProjects
	}

	inspectDir(workingDir)
	scanChildDirs(workingDir)

	currentDir := workingDir
	for range 2 {
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		if scanChildDirs(parentDir) > 0 {
			break
		}
		currentDir = parentDir
	}

	return discoveredProjects
}

func detectProcesses(filterTerm string, excludeTerms []string) []DetectedProcess {
	output, err := exec.Command("ps", "ax", "-o", "pid=,command=").Output()
	if err != nil {
		logger.Warn("ps command failed", "filter", filterTerm, "err", err)
		return nil
	}
	logger.Debug("scanning processes", "filter", filterTerm)

	processes := make([]DetectedProcess, 0)
	currentPID := os.Getpid()
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid == currentPID {
			continue
		}

		command := strings.TrimSpace(parts[1])
		if !strings.Contains(command, filterTerm) {
			continue
		}

		excluded := false
		for _, excludeTerm := range excludeTerms {
			if strings.Contains(command, excludeTerm) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		processes = append(processes, DetectedProcess{
			PID:              pid,
			Command:          command,
			WorkingDirectory: lookupProcessWorkingDirectory(pid),
		})
	}
	logger.Debug("process scan complete", "filter", filterTerm, "matched", len(processes))
	return processes
}

func lookupProcessWorkingDirectory(pid int) string {
	output, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strconv.Itoa(pid), "-Fn").Output()
	if err != nil {
		logger.Warn("lsof cwd lookup failed", "pid", pid, "err", err)
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
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

func detectProcessListeningPort(pid int) string {
	output, err := exec.Command("lsof", "-a", "-i", "TCP", "-sTCP:LISTEN", "-p", strconv.Itoa(pid), "-Fn", "-P").Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		separator := strings.LastIndex(line, ":")
		if separator < 0 {
			continue
		}
		port := line[separator+1:]
		if port != "4317" && port != "4318" {
			return port
		}
	}
	return ""
}
