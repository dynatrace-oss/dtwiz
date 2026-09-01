package analyzer

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectTech represents a technology detected in the current project directory.
type ProjectTech struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// detectProject scans the current working directory for well-known technology
// indicator files. Returns the display path of the directory (~ for home) and
// the detected tech stack.
func detectProject() (dir string, techs []ProjectTech) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil
	}

	dir = shortenPath(cwd)

	type entry struct {
		tech    string
		pattern string
	}

	candidates := []entry{
		{"Node.js", "package.json"},
		{"Go", "go.mod"},
		{"Python", "requirements.txt"},
		{"Python", "pyproject.toml"},
		{"Python", "setup.py"},
		{"Java", "pom.xml"},
		{"Java", "build.gradle"},
		{"Java", "build.gradle.kts"},
		{"Rust", "Cargo.toml"},
		{"Ruby", "Gemfile"},
		{"PHP", "composer.json"},
		{".NET", "*.csproj"},
		{".NET", "*.fsproj"},
		{".NET", "*.sln"},
	}

	seen := make(map[string]bool)

	for _, c := range candidates {
		if seen[c.tech] {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(cwd, c.pattern))
		if err != nil || len(matches) == 0 {
			continue
		}
		techs = append(techs, ProjectTech{Name: c.tech, Path: shortenPath(matches[0])})
		seen[c.tech] = true
	}

	return dir, techs
}

// shortenPath replaces the home directory prefix with ~.
// Uses filepath.Rel to avoid false matches on sibling directories
// (e.g. /home/alice vs /home/alice2) and to handle separator differences.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	if rel == "." {
		return "~"
	}
	return "~" + string(filepath.Separator) + rel
}
