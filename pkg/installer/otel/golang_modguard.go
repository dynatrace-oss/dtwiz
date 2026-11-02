package otel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	goOtelDirName       = "go"
	goModguardDirName   = "modguard"
	goModguardNoSumFile = "go.sum.absent"
	modguardRestoreTry  = 5
)

// modguardRetryDelay is the pause between restore attempts (Windows can hold
// files open briefly). A variable so tests do not wait.
var modguardRetryDelay = 200 * time.Millisecond

// goOtelDir returns the directory holding all dtwiz Go instrumentation
// artifacts of a module: <module>/.otel/go.
func goOtelDir(modulePath string) string {
	return filepath.Join(modulePath, ".otel", goOtelDirName)
}

// modGuard backs up go.mod/go.sum before otelc (which may rewrite them) runs
// and restores them byte-identical afterwards.
type modGuard struct {
	moduleDir string
	backupDir string
}

// newModGuard backs up the module files. A leftover backup from an interrupted
// run that differs from the current files stops the run; an identical one is
// stale and is removed.
func newModGuard(moduleDir string) (*modGuard, error) {
	g := &modGuard{moduleDir: moduleDir, backupDir: filepath.Join(goOtelDir(moduleDir), goModguardDirName)}

	if _, err := os.Stat(g.backupDir); err == nil {
		same, cmpErr := g.backupMatchesCurrent()
		if cmpErr != nil || !same {
			return nil, fmt.Errorf("a backup of go.mod/go.sum from an interrupted dtwiz run exists at %s and differs from the current files; "+
				"restore go.mod/go.sum from it (or from version control) and delete the directory, then rerun", g.backupDir)
		}
		if err := os.RemoveAll(g.backupDir); err != nil {
			return nil, fmt.Errorf("remove stale backup %s: %w", g.backupDir, err)
		}
	}

	if err := os.MkdirAll(g.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup directory %s: %w", g.backupDir, err)
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(moduleDir, name))
		switch {
		case err == nil:
			if err := os.WriteFile(filepath.Join(g.backupDir, name), data, 0644); err != nil {
				return nil, fmt.Errorf("back up %s: %w", name, err)
			}
		case errors.Is(err, os.ErrNotExist) && name == "go.sum":
			if err := os.WriteFile(filepath.Join(g.backupDir, goModguardNoSumFile), nil, 0644); err != nil {
				return nil, fmt.Errorf("record absent go.sum: %w", err)
			}
		default:
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
	}
	return g, nil
}

func (g *modGuard) backupMatchesCurrent() (bool, error) {
	for _, name := range []string{"go.mod", "go.sum"} {
		backup, bErr := os.ReadFile(filepath.Join(g.backupDir, name))
		current, cErr := os.ReadFile(filepath.Join(g.moduleDir, name))
		if errors.Is(bErr, os.ErrNotExist) && errors.Is(cErr, os.ErrNotExist) {
			continue
		}
		if bErr != nil || cErr != nil || !bytes.Equal(backup, current) {
			return false, nil
		}
	}
	return true, nil
}

// Restore puts go.mod/go.sum back to their backed-up state and removes the
// backup. On failure the backup is kept and its path is part of the error.
func (g *modGuard) Restore() error {
	var errs []error
	for _, name := range []string{"go.mod", "go.sum"} {
		target := filepath.Join(g.moduleDir, name)
		data, err := os.ReadFile(filepath.Join(g.backupDir, name))
		if errors.Is(err, os.ErrNotExist) {
			if name == "go.sum" {
				// go.sum did not exist before: remove whatever the build created.
				if rmErr := retry(func() error { return removeIfExists(target) }); rmErr != nil {
					errs = append(errs, fmt.Errorf("remove %s: %w", name, rmErr))
				}
			}
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("read backup of %s: %w", name, err))
			continue
		}
		if err := retry(func() error { return atomicWriteFile(target, data) }); err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("could not restore go.mod/go.sum (backup kept at %s): %w", g.backupDir, errors.Join(errs...))
	}
	if err := os.RemoveAll(g.backupDir); err != nil {
		logger.Warn("could not remove go.mod backup", "dir", g.backupDir, "err", err)
	}
	return nil
}

// withModGuard runs fn with go.mod/go.sum protected. The files are restored
// even when fn fails or its context is cancelled (the deferred restore runs
// after the cancelled command has returned).
func withModGuard(moduleDir string, fn func() error) (err error) {
	g, err := newModGuard(moduleDir)
	if err != nil {
		return err
	}
	defer func() {
		if rerr := g.Restore(); rerr != nil {
			err = errors.Join(err, rerr)
		}
	}()
	return fn()
}

func retry(op func() error) error {
	var err error
	for i := range modguardRestoreTry {
		if err = op(); err == nil {
			return nil
		}
		if i < modguardRestoreTry-1 {
			time.Sleep(modguardRetryDelay)
		}
	}
	return err
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// atomicWriteFile writes data to a temporary file next to path and renames it
// over path so a crash never leaves a half-written file.
func atomicWriteFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dtwiz-restore-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
