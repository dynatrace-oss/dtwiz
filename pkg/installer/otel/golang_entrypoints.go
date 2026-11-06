package otel

import (
	"bufio"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/environment"
)

// goMainPackage is a directory that holds a `package main` — a buildable entrypoint.
type goMainPackage struct {
	RelDir      string // slash-separated path relative to the project root; "." for the root
	ModuleDir   string // absolute path of the Go module containing the package (the build and .otel/ location)
	PkgDir      string // slash-separated path relative to ModuleDir; "." for the module root
	ServiceName string
}

// hasGoMainPackage reports whether dir contains a non-test .go file declaring `package main`.
func hasGoMainPackage(dir string) bool {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return false
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if goFileIsPackageMain(f) {
			return true
		}
	}
	return false
}

func goFileIsPackageMain(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package ") {
			return strings.Fields(line)[1] == "main"
		}
	}
	return false
}

const goMainSearchDepth = 5

// goWorkspaceModules returns the absolute module directories listed in the project's
// go.work `use` directives that live inside the project. Empty when there is no go.work.
func goWorkspaceModules(projectPath string) []string {
	data, err := os.ReadFile(filepath.Join(projectPath, "go.work"))
	if err != nil {
		return nil
	}
	var dirs []string
	inUseBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		var entry string
		switch {
		case inUseBlock && line == ")":
			inUseBlock = false
		case inUseBlock:
			entry = line
		case line == "use (":
			inUseBlock = true
		case strings.HasPrefix(line, "use ") && !strings.HasSuffix(line, "("):
			entry = strings.TrimPrefix(line, "use ")
		}
		entry = strings.Trim(strings.TrimSpace(entry), "\"`")
		if entry == "" {
			continue
		}
		dir := filepath.Clean(filepath.Join(projectPath, filepath.FromSlash(entry)))
		if rel, err := filepath.Rel(projectPath, dir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// detectGoMainPackages returns every directory declaring `package main` in the project
// module and, when the project has a go.work, in each workspace module (root,
// cmd/<name>, services/<name>, ...). Other nested modules are skipped — they are
// detected as separate projects. The root gets rootServiceName; others get their
// directory name, or their relative path when directory names collide
// (e.g. services/a/cmd/server and services/b/cmd/server).
func detectGoMainPackages(projectPath string, rootServiceName string) []goMainPackage {
	var pkgs []goMainPackage
	for _, moduleDir := range append([]string{projectPath}, goWorkspaceModules(projectPath)...) {
		pkgs = append(pkgs, walkGoModuleMains(projectPath, moduleDir)...)
	}

	counts := map[string]int{}
	for _, p := range pkgs {
		counts[path.Base(p.RelDir)]++
	}
	for i := range pkgs {
		switch base := path.Base(pkgs[i].RelDir); {
		case pkgs[i].RelDir == ".":
			pkgs[i].ServiceName = rootServiceName
		case counts[base] > 1:
			pkgs[i].ServiceName = environment.NormalizeServiceName(pkgs[i].RelDir)
		default:
			pkgs[i].ServiceName = base
		}
	}
	return pkgs
}

// walkGoModuleMains finds main packages inside one module, not descending into nested modules.
func walkGoModuleMains(projectPath, moduleDir string) []goMainPackage {
	var pkgs []goMainPackage
	_ = filepath.WalkDir(moduleDir, func(dir string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		pkgRel, relErr := filepath.Rel(moduleDir, dir)
		if relErr != nil {
			return nil
		}
		if pkgRel != "." {
			if isIgnoredDir(d.Name()) || d.Name() == "testdata" || strings.Count(filepath.ToSlash(pkgRel), "/") >= goMainSearchDepth {
				return filepath.SkipDir
			}
			if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
				return filepath.SkipDir
			}
		}
		if hasGoMainPackage(dir) {
			projRel, _ := filepath.Rel(projectPath, dir)
			pkgs = append(pkgs, goMainPackage{
				RelDir:    filepath.ToSlash(projRel),
				ModuleDir: moduleDir,
				PkgDir:    filepath.ToSlash(pkgRel),
			})
		}
		return nil
	})
	return pkgs
}
