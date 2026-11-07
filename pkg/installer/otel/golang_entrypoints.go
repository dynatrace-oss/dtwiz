package otel

import (
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/environment"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// goMaxServiceDepth is the deepest directory level below the module root that
// is searched for a `package main`.
const goMaxServiceDepth = 5

// GoService is one buildable `package main` inside a Go module.
type GoService struct {
	Name       string // service name used for OTEL_SERVICE_NAME and the binary
	Dir        string // absolute directory of the main package
	RelDir     string // slash-separated path relative to the module root, "." for the root
	ModuleDir  string // absolute directory of the module the package belongs to; empty means the project directory
	ProjectRel string // slash-separated path of Dir relative to the project, "." for the project root
}

// moduleDir returns the module the service is built in; fallback is the
// project directory, used when the service carries no module of its own.
func (s GoService) moduleDir(fallback string) string {
	if s.ModuleDir != "" {
		return s.ModuleDir
	}
	return fallback
}

// discoverGoServices finds every `package main` directory of the project: in
// its root module and, when the project has a go.work, in each workspace module
// listed there that lies inside the project. Testdata, ignored directories and
// other nested modules are skipped (those are separate projects).
func discoverGoServices(projectPath string) []GoService {
	var services []GoService
	for _, moduleDir := range append([]string{projectPath}, goWorkspaceModules(projectPath)...) {
		services = append(services, discoverModuleServices(projectPath, moduleDir)...)
	}

	sort.Slice(services, func(i, j int) bool { return services[i].ProjectRel < services[j].ProjectRel })
	nameGoServices(services, filepath.Base(projectPath))
	logger.Debug("go services discovered", "project", projectPath, "count", len(services))
	return services
}

// discoverModuleServices walks one module without descending into nested modules.
func discoverModuleServices(projectPath, moduleDir string) []GoService {
	var services []GoService

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			logger.Debug("go service discovery: cannot read directory", "dir", dir, "err", err)
			return
		}
		if hasMainPackage(dir, entries) {
			rel, _ := filepath.Rel(moduleDir, dir)
			projRel, _ := filepath.Rel(projectPath, dir)
			services = append(services, GoService{
				Dir:        dir,
				RelDir:     filepath.ToSlash(rel),
				ModuleDir:  moduleDir,
				ProjectRel: filepath.ToSlash(projRel),
			})
		}
		if depth >= goMaxServiceDepth {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || skipGoServiceDir(e.Name()) {
				continue
			}
			child := filepath.Join(dir, e.Name())
			if _, err := os.Stat(filepath.Join(child, "go.mod")); err == nil {
				continue // nested module: its own project (or listed in go.work)
			}
			walk(child, depth+1)
		}
	}
	walk(moduleDir, 0)
	return services
}

func skipGoServiceDir(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		ignoredProjectDirNames[name]
}

// hasMainPackage reports whether the non-test Go files in dir declare package main.
func hasMainPackage(dir string, entries []os.DirEntry) bool {
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// MatchFile evaluates //go:build and // +build lines and the _GOOS/_GOARCH
		// file-name suffixes for the current platform; the "ignore" tag is never set.
		if ok, err := build.Default.MatchFile(dir, name); err != nil || !ok {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err != nil || f.Name.Name != "main" {
			continue
		}
		return true
	}
	return false
}

// nameGoServices assigns each service its directory name, the project folder
// name for the root, and the normalized project-relative path when names collide.
func nameGoServices(services []GoService, projectFolder string) {
	baseName := func(s GoService) string {
		if s.ProjectRel == "." {
			return environment.ProjectServiceName(projectFolder)
		}
		return filepath.Base(filepath.FromSlash(s.ProjectRel))
	}
	counts := make(map[string]int, len(services))
	for _, s := range services {
		counts[baseName(s)]++
	}
	for i, s := range services {
		name := baseName(s)
		if counts[name] > 1 && s.ProjectRel != "." {
			name = environment.NormalizeServiceName(s.ProjectRel)
		}
		services[i].Name = name
	}
}
