package installer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var pythonProjectMarkers = []string{
	"pyproject.toml",
	"setup.py",
	"setup.cfg",
	"requirements.txt",
	"Pipfile",
	"poetry.lock",
	"manage.py",
}

func detectPythonProjects() []ScannedProject {
	return scanProjectDirs(pythonProjectMarkers, nil)
}

func detectPythonProcesses() []DetectedProcess {
	return detectProcesses("python", []string{"pip ", "setup.py", "/bin/dtwiz"})
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

// serviceNameFromEntrypoint derives OTEL_SERVICE_NAME from project path and entrypoint.
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
		logger.Debug("could not read project directory while scanning for entrypoints", "path", projectPath, "error", err)
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || isIgnoredDir(e.Name()) {
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


