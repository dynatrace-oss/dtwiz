package otel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// isJSFileExtension checks if a string ends with a JavaScript/TypeScript file extension.
func isJSFileExtension(s string) bool {
	return strings.HasSuffix(s, ".js") || strings.HasSuffix(s, ".ts") ||
		strings.HasSuffix(s, ".mjs") || strings.HasSuffix(s, ".cjs") ||
		strings.HasSuffix(s, ".mts") || strings.HasSuffix(s, ".cts")
}

// extractScriptFile extracts a file reference from a script command string.
// It looks for tokens ending in .js/.ts/.mjs/.cjs/.mts/.cts that exist on disk.
func extractScriptFile(projectPath, script string) string {
	parts := strings.Fields(script)
	for _, part := range parts {
		if isJSFileExtension(part) {
			if _, err := os.Stat(filepath.Join(projectPath, part)); err == nil {
				return part
			}
		}
	}
	return ""
}

// runtimeScriptPrefixes lists package.json script name prefixes that indicate
// a server/runtime entrypoint (as opposed to build/lint/test tooling). Only
// scripts matching one of these prefixes are considered when scanning "other
// scripts" for entrypoints.
var runtimeScriptPrefixes = []string{"start", "dev", "serve", "server"}

// isRuntimeScript checks if a package.json script name looks like a runtime
// entrypoint (e.g. "start:api", "dev", "serve:frontend") rather than a
// build/lint/test script.
func isRuntimeScript(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range runtimeScriptPrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+":") {
			return true
		}
	}
	return false
}

func detectNodeEntrypoints(projectPath string) []string {
	// For framework projects, return marker entrypoints.
	framework := detectNodeFramework(projectPath)
	if framework == "next" {
		logger.Debug("node entrypoint: Next.js project detected", "path", projectPath)
		return []string{"next:start"}
	}
	if framework == "nuxt" {
		logger.Debug("node entrypoint: Nuxt project detected", "path", projectPath)
		return []string{"nuxt:start"}
	}

	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}

	var pkg packageJSON
	_ = json.Unmarshal(data, &pkg)

	if pkg.Main != "" {
		logger.Debug("node entrypoint: checking 'main' field", "main", pkg.Main)
		if _, err := os.Stat(filepath.Join(projectPath, pkg.Main)); err == nil {
			logger.Debug("node entrypoint found via 'main'", "file", pkg.Main)
			return []string{pkg.Main}
		}
	}

	if start, ok := pkg.Scripts["start"]; ok && start != "" {
		logger.Debug("node entrypoint: checking 'scripts.start'", "start", start)
		if found := extractScriptFile(projectPath, start); found != "" {
			logger.Debug("node entrypoint found via 'scripts.start'", "file", found)
			return []string{found}
		}
	}

	// Scan runtime-like scripts (start:*, dev:*, serve:*, server:*) for file
	// references. Non-runtime scripts (build, lint, test, etc.) are skipped to
	// avoid picking up config files as entrypoints.
	if len(pkg.Scripts) > 0 {
		seen := make(map[string]bool)
		var otherEntrypoints []string
		for name, script := range pkg.Scripts {
			if name == "start" || script == "" {
				continue
			}
			if !isRuntimeScript(name) {
				continue
			}
			if found := extractScriptFile(projectPath, script); found != "" && !seen[found] {
				seen[found] = true
				otherEntrypoints = append(otherEntrypoints, found)
				logger.Debug("node entrypoint found via script", "script", name, "file", found)
			}
		}
		if len(otherEntrypoints) > 0 {
			sort.Strings(otherEntrypoints)
			return otherEntrypoints
		}
	}

	for _, base := range []string{"index", "app", "server"} {
		for _, ext := range []string{".js", ".ts", ".mjs", ".cjs", ".mts", ".cts"} {
			name := base + ext
			if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
				logger.Debug("node entrypoint found via fallback", "file", name)
				return []string{name}
			}
		}
	}

	return nil
}
