package otel

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type GoProject struct {
	ScannedProject
	ModuleName string
}

func extractGoModuleName(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		logger.Warn("failed to read go.mod", "path", goModPath, "err", err)
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			logger.Debug("go module name extracted", "path", goModPath, "module", mod)
			return mod
		}
	}
	return ""
}

func detectGoProjects(roots []string) []GoProject {
	scanned := scanProjectDirs([]string{"go.mod"}, nil, roots)
	projects := make([]GoProject, 0, len(scanned))
	for _, s := range scanned {
		moduleName := extractGoModuleName(filepath.Join(s.Path, "go.mod"))
		projects = append(projects, GoProject{
			ScannedProject: s,
			ModuleName:     moduleName,
		})
	}
	logger.Debug("Go projects detected", "count", len(projects))
	return projects
}

func (p *GoInstrumentationPlan) Runtime() string { return "Go" }
