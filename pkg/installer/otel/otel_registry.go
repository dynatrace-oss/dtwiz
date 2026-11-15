package otel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// The registry records the instrumentation directories dtwiz created (Node.js
// `.otel/`, Go `.otel/go`), so `dtwiz uninstall otel` finds them from any working
// directory instead of only below the current one. It lives next to the other
// dtwiz-owned files under the home directory.

// otelRegistryPath returns the registry file location. A variable so tests can
// point it at a temporary file.
var otelRegistryPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".opentelemetry", "dtwiz", "instrumented-dirs.json"), nil
}

type otelRegistryFile struct {
	Dirs []string `json:"dirs"`
}

func readOtelRegistry() []string {
	path, err := otelRegistryPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f otelRegistryFile
	if err := json.Unmarshal(data, &f); err != nil {
		logger.Debug("ignoring unreadable instrumentation registry", "path", path, "err", err)
		return nil
	}
	return f.Dirs
}

func writeOtelRegistry(dirs []string) error {
	path, err := otelRegistryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	sort.Strings(dirs)
	data, err := json.MarshalIndent(otelRegistryFile{Dirs: dirs}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// registerOtelDir records dir as created by dtwiz. Failures are logged only:
// the registry is a lookup aid and must never fail an installation.
func registerOtelDir(dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		logger.Debug("could not resolve instrumentation directory", "dir", dir, "err", err)
		return
	}
	dirs := readOtelRegistry()
	for _, d := range dirs {
		if d == abs {
			return
		}
	}
	if err := writeOtelRegistry(append(dirs, abs)); err != nil {
		logger.Debug("could not record instrumentation directory", "dir", abs, "err", err)
	}
}

// registeredOtelDirs returns the recorded directories that still exist.
func registeredOtelDirs() []string {
	var existing []string
	for _, d := range readOtelRegistry() {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			existing = append(existing, d)
		}
	}
	return existing
}

// pruneOtelRegistry drops recorded directories that no longer exist. When none
// are left it removes the registry file and its directory, if that is empty.
func pruneOtelRegistry() {
	all := readOtelRegistry()
	kept := registeredOtelDirs()
	if len(kept) == 0 {
		if path, err := otelRegistryPath(); err == nil {
			_ = os.Remove(path)
			removeIfEmpty(filepath.Dir(path))
		}
		return
	}
	if len(kept) == len(all) {
		return
	}
	if err := writeOtelRegistry(kept); err != nil {
		logger.Debug("could not update instrumentation registry", "err", err)
	}
}
