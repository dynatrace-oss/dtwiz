package otel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// goCleaner plugs Go into the runtime uninstall flow. It stops services that
// dtwiz launched from a module's .otel/go/bin directory.
type goCleaner struct{}

func (goCleaner) Label() string { return "Go" }

// DetectProcesses implements RuntimeCleaner. With the GoInstrumentation flag off
// it reports nothing, and a failed process scan yields nil (runtime skipped).
func (goCleaner) DetectProcesses() []DetectedProcess {
	if !featureflags.IsEnabled(featureflags.GoInstrumentation) {
		return []DetectedProcess{}
	}
	all := goListProcessesFn()
	if all == nil {
		return nil
	}
	return selectDtwizGoProcesses(all)
}

// selectDtwizGoProcesses keeps processes whose executable lives under a
// .otel/go/bin/ directory that carries dtwiz's marker file, wherever that
// directory is on the machine. Plain `go run` processes and binaries that merely
// sit in a similarly named folder never match.
func selectDtwizGoProcesses(all []DetectedProcess) []DetectedProcess {
	matched := make([]DetectedProcess, 0)
	for _, p := range all {
		if isDtwizGoBinary(p.Command) && hasDtwizGoMarker(p.Command) {
			matched = append(matched, p)
		}
	}
	return matched
}

// hasDtwizGoMarker reports whether the .otel/go directory that holds the
// command's executable carries the marker file dtwiz writes when it generates it.
func hasDtwizGoMarker(command string) bool {
	path := strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(command), `"`), `\`, "/")
	i := strings.Index(path, dtwizGoBinDir)
	if i < 0 {
		return false
	}
	goDir := path[:i+len("/.otel/go")]
	_, err := os.Stat(filepath.Join(filepath.FromSlash(goDir), goMarkerFile))
	return err == nil
}

// findGoOtelDirs finds <module>/.otel/go directories carrying dtwiz's marker
// file: those recorded in the registry (any location) plus those below the
// current directory (installs from before the registry existed). It returns nil when the
// GoInstrumentation flag is off.
func findGoOtelDirs() []string {
	if !featureflags.IsEnabled(featureflags.GoInstrumentation) {
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	var mu sync.Mutex
	seen := map[string]bool{}
	var dirs []string
	walkCandidateDirs(cwd, 0, func(dir string, entries []os.DirEntry) bool {
		for _, e := range entries {
			if e.Name() != ".otel" || !e.IsDir() {
				continue
			}
			goDir := filepath.Join(dir, ".otel", goOtelDirName)
			if _, err := os.Stat(filepath.Join(goDir, goMarkerFile)); err != nil {
				continue
			}
			key := goDir
			if resolved, err := filepath.EvalSymlinks(goDir); err == nil {
				key = resolved
			}
			mu.Lock()
			if !seen[key] {
				seen[key] = true
				dirs = append(dirs, goDir)
				logger.Debug("found Go instrumentation directory", "dir", goDir)
			}
			mu.Unlock()
		}
		return false
	}, isIgnoredDir)

	// Directories dtwiz recorded when it created them, wherever they are.
	for _, goDir := range registeredOtelDirs() {
		if _, err := os.Stat(filepath.Join(goDir, goMarkerFile)); err != nil {
			continue
		}
		key := goDir
		if resolved, err := filepath.EvalSymlinks(goDir); err == nil {
			key = resolved
		}
		if !seen[key] {
			seen[key] = true
			dirs = append(dirs, goDir)
			logger.Debug("found registered Go instrumentation directory", "dir", goDir)
		}
	}
	return dirs
}

// goBackupDir returns the interrupted-build backup inside a Go instrumentation
// directory when it exists and holds files.
func goBackupDir(goDir string) (string, bool) {
	backup := filepath.Join(goDir, goModguardDirName)
	entries, err := os.ReadDir(backup)
	return backup, err == nil && len(entries) > 0
}

// isolatedOtelcPath returns the path of the otelc that dtwiz installed into its
// own tool directory, if present. Copies on PATH or in GOBIN are never returned.
// It returns "" when the GoInstrumentation flag is off.
func isolatedOtelcPath() string {
	if !featureflags.IsEnabled(featureflags.GoInstrumentation) {
		return ""
	}
	dir, err := isolatedOtelcDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, otelcBinaryName())
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	return ""
}

// removeGoOtelDir deletes one Go instrumentation directory and then the
// module's .otel folder if that is left empty. A leftover go.mod/go.sum backup
// is preserved; its path is returned so the caller can warn.
func removeGoOtelDir(goDir string) (preservedBackup string, err error) {
	if backup, ok := goBackupDir(goDir); ok {
		entries, readErr := os.ReadDir(goDir)
		if readErr != nil {
			return backup, readErr
		}
		var errs []error
		for _, e := range entries {
			if e.Name() == goModguardDirName {
				continue
			}
			if rmErr := removeWithRetry(filepath.Join(goDir, e.Name())); rmErr != nil {
				errs = append(errs, rmErr)
			}
		}
		return backup, errors.Join(errs...)
	}
	if err := removeWithRetry(goDir); err != nil {
		return "", err
	}
	removeIfEmpty(filepath.Dir(goDir)) // the module's .otel folder
	return "", nil
}

// removeIsolatedOtelc deletes dtwiz's own otelc and the then-empty tool
// directories. The shared Go module cache is not touched.
func removeIsolatedOtelc(path string) error {
	if err := removeWithRetry(path); err != nil {
		return err
	}
	dir := filepath.Dir(path) // ~/.opentelemetry/go/bin
	for range 3 {
		if !removeIfEmpty(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}
	return nil
}

// removeIfEmpty removes dir when it has no entries and reports whether it did.
func removeIfEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return false
	}
	return os.Remove(dir) == nil
}

// printGoUninstallSection lists the Go directories, backups and tool that will
// be removed.
func printGoUninstallSection(dirs []string, toolPath string) {
	if len(dirs) > 0 {
		fmt.Println("  Go instrumentation directories that will be removed:")
		for _, d := range dirs {
			fmt.Printf("    rm -rf %s\n", d)
			if backup, ok := goBackupDir(d); ok {
				fmt.Printf("    Warning: %s holds a go.mod/go.sum backup from an interrupted build; it will be kept.\n", backup)
			}
		}
		fmt.Println()
	}
	if toolPath != "" {
		fmt.Println("  Instrumentation tool installed by dtwiz that will be removed:")
		fmt.Printf("    rm %s\n", toolPath)
		fmt.Println()
	}
}

// removeGoArtifacts removes the listed directories and tool after the services
// were stopped. Failures are reported per item and do not stop the others.
func removeGoArtifacts(dirs []string, toolPath string) {
	for _, d := range dirs {
		backup, err := removeGoOtelDir(d)
		if err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", d, err)
		} else {
			fmt.Printf("  Removed %s\n", d)
		}
		if backup != "" {
			fmt.Printf("  Warning: kept %s — it holds go.mod/go.sum from an interrupted build; restore them from it and delete it.\n", backup)
		}
	}
	if toolPath != "" {
		if err := removeIsolatedOtelc(toolPath); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", toolPath, err)
		} else {
			fmt.Printf("  Removed %s\n", toolPath)
		}
	}
}
