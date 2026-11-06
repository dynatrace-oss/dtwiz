package otel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// otelc rejects -modfile, so it builds against the project's real go.mod/go.sum and may rewrite
// them (go directive bump, added requirements). modGuard keeps the originals both in memory and
// on disk under <module>/.otel/modguard/, so they can be put back after the build and, if dtwiz
// is killed mid-build, be recovered by hand.

const (
	modGuardDirName = "modguard"
	absentSuffix    = ".absent" // marker file: the original did not exist
)

var guardedModuleFiles = []string{"go.mod", "go.sum"}

type guardedFile struct {
	name    string
	data    []byte
	mode    os.FileMode
	existed bool
}

type modGuard struct {
	moduleDir string
	files     []guardedFile
}

func modGuardDir(moduleDir string) string {
	return filepath.Join(moduleDir, goOtelDirName, modGuardDirName)
}

// writeFileAtomic writes next to path and renames, so a crash never leaves a half-written file.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".dtwiz-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// checkLeftoverBackup fails when an earlier build was interrupted and left a backup that no
// longer matches the module files. Restoring automatically could overwrite changes the user made
// since, so the decision is theirs; identical files mean the previous run did restore and the
// backup is simply stale.
func checkLeftoverBackup(moduleDir string) error {
	dir := modGuardDir(moduleDir)
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	for _, name := range guardedModuleFiles {
		current, curErr := os.ReadFile(filepath.Join(moduleDir, name))
		backup, bakErr := os.ReadFile(filepath.Join(dir, name))
		_, absentErr := os.Stat(filepath.Join(dir, name+absentSuffix))
		switch {
		case bakErr == nil && curErr == nil && bytes.Equal(current, backup):
		case absentErr == nil && os.IsNotExist(curErr):
		case bakErr != nil && absentErr != nil:
			// no backup was recorded for this file, so the build never touched it
		default:
			return fmt.Errorf("a previous dtwiz build was interrupted and %s may still contain otelc changes.\n"+
				"    The original go.mod/go.sum were saved in %s.\n"+
				"    Compare them with the module files (git diff also helps), copy them back if needed,\n"+
				"    delete that folder and run dtwiz again", name, dir)
		}
	}
	return os.RemoveAll(dir)
}

// newModGuard backs up go.mod and go.sum of the module. Call Restore once the build is done.
func newModGuard(moduleDir string) (*modGuard, error) {
	if err := checkLeftoverBackup(moduleDir); err != nil {
		return nil, err
	}
	dir := modGuardDir(moduleDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating backup dir %s: %w", dir, err)
	}
	g := &modGuard{moduleDir: moduleDir}
	for _, name := range guardedModuleFiles {
		path := filepath.Join(moduleDir, name)
		f := guardedFile{name: name}
		info, err := os.Stat(path)
		switch {
		case err == nil:
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil, fmt.Errorf("reading %s: %w", path, rerr)
			}
			f.data, f.mode, f.existed = data, info.Mode().Perm(), true
			if werr := writeFileAtomic(filepath.Join(dir, name), data, f.mode); werr != nil {
				return nil, fmt.Errorf("backing up %s: %w", path, werr)
			}
		case os.IsNotExist(err):
			if werr := os.WriteFile(filepath.Join(dir, name+absentSuffix), nil, 0o644); werr != nil {
				return nil, fmt.Errorf("backing up %s: %w", path, werr)
			}
		default:
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		g.files = append(g.files, f)
	}
	return g, nil
}

// Restore puts the original go.mod/go.sum back. On success the on-disk backup is deleted; on
// failure it is kept and the returned error says where it is.
func (g *modGuard) Restore() error {
	var errs []error
	for _, f := range g.files {
		path := filepath.Join(g.moduleDir, f.name)
		var err error
		if f.existed {
			err = writeFileAtomic(path, f.data, f.mode)
		} else if rerr := os.Remove(path); rerr != nil && !os.IsNotExist(rerr) {
			err = rerr
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("restoring %s: %w", path, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w\n    Your original files are saved in %s — copy them back manually",
			errors.Join(errs...), modGuardDir(g.moduleDir))
	}
	return os.RemoveAll(modGuardDir(g.moduleDir))
}
