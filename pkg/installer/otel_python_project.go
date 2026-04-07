package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// PythonProcess describes a detected running Python process.
type PythonProcess struct {
	PID     int
	Command string
	CWD     string
}

// PythonProject describes a detected Python project directory.
type PythonProject struct {
	Path        string
	Markers     []string
	RunningPIDs []int
}

var pythonProjectMarkers = []string{
	"pyproject.toml",
	"setup.py",
	"setup.cfg",
	"requirements.txt",
	"Pipfile",
	"poetry.lock",
	"manage.py",
}

// detectPythonProjects scans common locations for Python project directories.
// Looks in CWD (+ one level of subdirectories) and common project locations under $HOME.
//
// TODO(post-rebase): Replace this implementation with the version from the upstream branch
// that properly handles Python project detection. This is a placeholder until the rebase
// on top of the proper handling lands.
func detectPythonProjects() []PythonProject {
	var projects []PythonProject
	seen := make(map[string]bool)

	checkDir := func(dir string) {
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			resolved = dir
		}
		key := strings.ToLower(resolved)
		if seen[key] {
			return
		}
		seen[key] = true
		var markers []string
		for _, marker := range pythonProjectMarkers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				markers = append(markers, marker)
			}
		}
		if len(markers) > 0 {
			projects = append(projects, PythonProject{Path: dir, Markers: markers})
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		checkDir(cwd)
		entries, _ := os.ReadDir(cwd)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				checkDir(filepath.Join(cwd, e.Name()))
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		for _, base := range []string{"Code", "code", "projects", "src", "dev"} {
			dir := filepath.Join(home, base)
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				sub := filepath.Join(dir, e.Name())
				checkDir(sub)
				subEntries, err := os.ReadDir(sub)
				if err != nil {
					continue
				}
				for _, se := range subEntries {
					if se.IsDir() && !strings.HasPrefix(se.Name(), ".") {
						checkDir(filepath.Join(sub, se.Name()))
					}
				}
			}
		}
	}

	return projects
}

// matchProcessesToProjects associates detected Python processes with their
// project directories by checking CWD and command line.
func matchProcessesToProjects(projects []PythonProject, procs []PythonProcess) {
	for i := range projects {
		projLower := strings.ToLower(projects[i].Path)
		for _, p := range procs {
			cwdLower := strings.ToLower(p.CWD)
			cmdLower := strings.ToLower(p.Command)
			if strings.HasPrefix(cwdLower, projLower) || strings.Contains(cmdLower, projLower) {
				projects[i].RunningPIDs = append(projects[i].RunningPIDs, p.PID)
			}
		}
	}
}

// stopProcesses sends SIGINT to the given PIDs and waits for them to exit.
func stopProcesses(pids []int) {
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := proc.Signal(os.Interrupt); err != nil {
			fmt.Printf("    Warning: could not stop PID %d: %v\n", pid, err)
			continue
		}
		_, _ = proc.Wait()
		fmt.Printf("    Stopped PID %d\n", pid)
	}
}

var commonEntrypoints = []string{
	"main.py",
	"app.py",
	"run.py",
	"server.py",
	"manage.py",
	"wsgi.py",
	"asgi.py",
}

// serviceNameFromEntrypoint derives OTEL_SERVICE_NAME from a project path and entrypoint.
//
// Examples:
//
//	"app.py"                in "orderschnitzel" → "orderschnitzel"
//	"s-frontend/app.py"     in "orderschnitzel" → "orderschnitzel-s-frontend"
//	"services/api/main.py"  in "myapp"          → "myapp-api"
func serviceNameFromEntrypoint(projectPath, entrypoint string) string {
	projectName := filepath.Base(projectPath)
	dir := filepath.Dir(entrypoint)
	if dir == "." || dir == "" {
		return projectName
	}
	servicePart := filepath.Base(dir)
	return projectName + "-" + servicePart
}

// detectPythonEntrypoints finds Python entrypoint files in a project.
// Checks pyproject.toml scripts, common filenames in the project root, and
// common filenames in immediate subdirectories (for multi-service projects).
func detectPythonEntrypoints(projectPath string) []string {
	var entrypoints []string

	pyproject := filepath.Join(projectPath, "pyproject.toml")
	if data, err := os.ReadFile(pyproject); err == nil {
		if ep := parseEntrypointFromPyproject(string(data)); ep != "" {
			entrypoints = append(entrypoints, ep)
		}
	}
	if len(entrypoints) > 0 {
		return entrypoints
	}

	for _, name := range commonEntrypoints {
		if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
			entrypoints = append(entrypoints, name)
		}
	}
	if len(entrypoints) > 0 {
		return entrypoints
	}

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "__pycache__" ||
			e.Name() == "node_modules" {
			continue
		}
		subDir := filepath.Join(projectPath, e.Name())
		for _, name := range commonEntrypoints {
			if _, err := os.Stat(filepath.Join(subDir, name)); err == nil {
				entrypoints = append(entrypoints, filepath.Join(e.Name(), name))
			}
		}
	}
	return entrypoints
}

// parseEntrypointFromPyproject extracts a script entrypoint from pyproject.toml content.
// Converts `module:func` under [project.scripts] to a file path.
func parseEntrypointFromPyproject(content string) string {
	inScripts := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inScripts = trimmed == "[project.scripts]" || trimmed == "[tool.poetry.scripts]"
			continue
		}
		if !inScripts {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		if colonIdx := strings.Index(val, ":"); colonIdx > 0 {
			modPath := val[:colonIdx]
			return strings.ReplaceAll(modPath, ".", "/") + ".py"
		}
	}
	return ""
}

// detectPythonProcesses finds running Python processes (excluding current process and system processes).
func detectPythonProcesses() []PythonProcess {
	out, err := exec.Command("ps", "ax", "-o", "pid=,command=").Output()
	if err != nil {
		return nil
	}

	var procs []PythonProcess
	myPID := os.Getpid()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid == myPID {
			continue
		}
		cmd := strings.TrimSpace(parts[1])
		if !strings.Contains(cmd, "python") {
			continue
		}
		if strings.Contains(cmd, "pip ") || strings.Contains(cmd, "setup.py") ||
			strings.Contains(cmd, "/bin/dtwiz") {
			continue
		}
		procs = append(procs, PythonProcess{PID: pid, Command: cmd, CWD: getProcessCWD(pid)})
	}
	return procs
}

// getProcessCWD returns the current working directory of a process using lsof.
func getProcessCWD(pid int) string {
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strconv.Itoa(pid), "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}
