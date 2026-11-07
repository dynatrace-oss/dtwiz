package otel

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// goModuleRequirements returns the module paths required by a go.mod file with
// a flag telling whether the requirement is marked `// indirect`.
func goModuleRequirements(goModPath string) (map[string]bool, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}
	f, err := modfile.ParseLax(goModPath, data, nil)
	if err != nil {
		return nil, err
	}
	reqs := make(map[string]bool, len(f.Require))
	for _, r := range f.Require {
		reqs[r.Mod.Path] = r.Indirect
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
	work, err := modfile.ParseWork("go.work", data, nil)
	if err != nil {
		logger.Debug("could not parse go.work", "path", projectPath, "err", err)
		return nil
	}
	var dirs []string
	seen := map[string]bool{}
	for _, use := range work.Use {
		entry := use.Path
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
