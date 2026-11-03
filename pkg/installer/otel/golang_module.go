package otel

import (
	"os"
	"path/filepath"
	"strings"
)

// goModuleRequirements returns the module paths required by a go.mod file with
// a flag telling whether the requirement is marked `// indirect`.
func goModuleRequirements(goModPath string) (map[string]bool, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}
	reqs := make(map[string]bool)
	inBlock := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		indirect := false
		if idx := strings.Index(line, "//"); idx >= 0 {
			indirect = strings.Contains(line[idx:], "indirect")
			line = strings.TrimSpace(line[:idx])
		}
		switch {
		case line == "":
			continue
		case inBlock && line == ")":
			inBlock = false
		case line == "require (":
			inBlock = true
		case inBlock:
			if f := strings.Fields(line); len(f) >= 2 {
				reqs[f[0]] = indirect
			}
		case strings.HasPrefix(line, "require "):
			if f := strings.Fields(strings.TrimPrefix(line, "require ")); len(f) >= 2 {
				reqs[f[0]] = indirect
			}
		}
	}
	return reqs, nil
}

// moduleUsesOtelSDK reports whether the module directly requires the
// OpenTelemetry SDK or an OTLP/other exporter (`// indirect` requirements are
// ignored: they come from other dependencies, not from the application).
func moduleUsesOtelSDK(goModPath string) bool {
	reqs, err := goModuleRequirements(goModPath)
	if err != nil {
		return false
	}
	for mod, indirect := range reqs {
		if indirect {
			continue
		}
		if strings.HasPrefix(mod, "go.opentelemetry.io/otel/sdk") ||
			strings.HasPrefix(mod, "go.opentelemetry.io/otel/exporters/") {
			return true
		}
	}
	return false
}

// findGoWork returns the path of the go.work file that applies to dir (in dir
// or any parent directory), or "" when the module is not part of a workspace.
func findGoWork(dir string) string {
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, "go.work")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// goWorkspaceModules returns the absolute module directories named by the
// `use` directives of <projectPath>/go.work that lie inside the project and
// contain a go.mod. The project directory itself and entries outside it are
// dropped; the result has no duplicates. It is empty without a go.work.
func goWorkspaceModules(projectPath string) []string {
	data, err := os.ReadFile(filepath.Join(projectPath, "go.work"))
	if err != nil {
		return nil
	}
	var dirs []string
	seen := map[string]bool{}
	inUse := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := raw
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		var entry string
		switch {
		case inUse && line == ")":
			inUse = false
		case inUse:
			entry = line
		case line == "use (":
			inUse = true
		case strings.HasPrefix(line, "use ") && !strings.HasSuffix(line, "("):
			entry = strings.TrimPrefix(line, "use ")
		}
		entry = strings.Trim(strings.TrimSpace(entry), "\"`")
		if entry == "" {
			continue
		}
		dir := filepath.Clean(filepath.Join(projectPath, filepath.FromSlash(entry)))
		rel, err := filepath.Rel(projectPath, dir)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || seen[dir] {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}
