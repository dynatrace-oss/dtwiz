package otel

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/markers"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type packageJSON struct {
	Main            string            `json:"main"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

var nodeProjectMarkers = []string{
	"package.json",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"bun.lockb",
	".nvmrc",
	".node-version",
}

func detectNodeProjects(roots []string) []ScannedProject {
	projects := scanProjectDirs(nodeProjectMarkers, []string{"node_modules"}, roots)

	// Expand monorepo workspaces: for each project with a "workspaces" field,
	// resolve workspace directories and add them as individual projects.
	var expanded []ScannedProject
	seen := make(map[string]bool)
	for _, p := range projects {
		seen[p.Path] = true
	}
	for _, p := range projects {
		dirs := resolveWorkspaces(p.Path)
		for _, dir := range dirs {
			if !seen[dir] {
				seen[dir] = true
				expanded = append(expanded, ScannedProject{
					Path:    dir,
					Markers: []string{"package.json"},
				})
			}
		}
	}

	return append(projects, expanded...)
}

func detectNodeProcesses() []DetectedProcess {
	return detectProcesses("node", []string{"npm "})
}

// detectNodeFramework returns "next", "nuxt", or "" for a project directory.
// Next.js takes precedence when both are detected.
func detectNodeFramework(projectPath string) string {
	if markers.IsNextJSProject(projectPath) {
		return "next"
	}
	if markers.IsNuxtProject(projectPath) {
		return "nuxt"
	}
	return ""
}

// detectNodePackageManager detects the package manager from lockfiles.
func detectNodePackageManager(projectPath string) string {
	if _, err := os.Stat(filepath.Join(projectPath, "package-lock.json")); err == nil {
		logger.Debug("detected package manager", "path", projectPath, "manager", "npm")
		return "npm"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "yarn.lock")); err == nil {
		logger.Debug("detected package manager", "path", projectPath, "manager", "yarn")
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "pnpm-lock.yaml")); err == nil {
		logger.Debug("detected package manager", "path", projectPath, "manager", "pnpm")
		return "pnpm"
	}
	logger.Debug("no lockfile found, defaulting to npm", "path", projectPath)
	return "npm"
}

// resolveWorkspaces parses the workspaces field from package.json and returns
// workspace directories that contain their own package.json.
func resolveWorkspaces(projectPath string) []string {
	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	if pkg.Workspaces == nil {
		return nil
	}

	var patterns []string

	// Try as array of strings: "workspaces": ["packages/*"]
	var arr []string
	if err := json.Unmarshal(pkg.Workspaces, &arr); err == nil {
		patterns = arr
	} else {
		// Try as object: "workspaces": {"packages": ["packages/*"]}
		var obj struct {
			Packages []string `json:"packages"`
		}
		if err := json.Unmarshal(pkg.Workspaces, &obj); err == nil {
			patterns = obj.Packages
		}
	}

	if len(patterns) == 0 {
		return nil
	}

	var workspaceDirs []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectPath, pattern))
		if err != nil {
			continue
		}
		logger.Debug("resolved workspace pattern", "pattern", pattern, "matchCount", len(matches))
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(match, "package.json")); err == nil {
				logger.Debug("workspace dir added", "dir", match)
				workspaceDirs = append(workspaceDirs, match)
			}
		}
	}
	return workspaceDirs
}
