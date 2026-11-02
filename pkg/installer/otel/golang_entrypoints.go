package otel

import (
	"go/ast"
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
	Name   string // service name used for OTEL_SERVICE_NAME and the binary
	Dir    string // absolute directory of the main package
	RelDir string // slash-separated path relative to the module root, "." for the root
}

// discoverGoServices finds every `package main` directory in the module rooted
// at modulePath, skipping testdata, ignored directories and nested modules.
func discoverGoServices(modulePath string) []GoService {
	var services []GoService

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			logger.Debug("go service discovery: cannot read directory", "dir", dir, "err", err)
			return
		}
		if hasMainPackage(dir, entries) {
			rel, _ := filepath.Rel(modulePath, dir)
			services = append(services, GoService{Dir: dir, RelDir: filepath.ToSlash(rel)})
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
				continue // nested module: its own project
			}
			walk(child, depth+1)
		}
	}
	walk(modulePath, 0)

	sort.Slice(services, func(i, j int) bool { return services[i].RelDir < services[j].RelDir })
	nameGoServices(services, filepath.Base(modulePath))
	logger.Debug("go services discovered", "module", modulePath, "count", len(services))
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
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.PackageClauseOnly|parser.ParseComments)
		if err != nil || f.Name.Name != "main" || hasIgnoreConstraint(f) {
			continue
		}
		return true
	}
	return false
}

// hasIgnoreConstraint reports files marked `//go:build ignore` (generators, scripts).
func hasIgnoreConstraint(f *ast.File) bool {
	for _, group := range f.Comments {
		if group.Pos() > f.Package {
			break
		}
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:build") && strings.Contains(c.Text, "ignore") {
				return true
			}
		}
	}
	return false
}

// nameGoServices assigns each service its directory name, the project folder
// name for the root, and the normalized relative path when names collide.
func nameGoServices(services []GoService, projectFolder string) {
	baseName := func(s GoService) string {
		if s.RelDir == "." {
			return environment.ProjectServiceName(projectFolder)
		}
		return filepath.Base(filepath.FromSlash(s.RelDir))
	}
	counts := make(map[string]int, len(services))
	for _, s := range services {
		counts[baseName(s)]++
	}
	for i, s := range services {
		name := baseName(s)
		if counts[name] > 1 && s.RelDir != "." {
			name = environment.NormalizeServiceName(s.RelDir)
		}
		services[i].Name = name
	}
}
