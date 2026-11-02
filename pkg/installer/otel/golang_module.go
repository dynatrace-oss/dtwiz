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
