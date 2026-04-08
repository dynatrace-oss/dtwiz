package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var otelGoPackages = []string{
	"go.opentelemetry.io/otel",
	"go.opentelemetry.io/otel/sdk",
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
}

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

func detectGoProjects() []GoProject {
	scanned := scanProjectDirs([]string{"go.mod"}, nil)
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

type GoInstrumentationPlan struct {
	Project GoProject
	EnvVars map[string]string
}

func (p *GoInstrumentationPlan) Runtime() string { return "Go" }

func DetectGoPlan(apiURL, token string) *GoInstrumentationPlan {
	if _, err := exec.LookPath("go"); err != nil {
		logger.Debug("go not found on PATH", "skipping Go instrumentation")
		return nil
	}

	projects, processes := runInParallel(detectGoProjects, func() []DetectedProcess {
		return detectProcesses("go", nil)
	})
	if len(projects) == 0 {
		logger.Debug("no Go projects detected", "skipping Go instrumentation")
		return nil
	}

	scanned := make([]ScannedProject, len(projects))
	for i := range projects {
		scanned[i] = projects[i].ScannedProject
	}

	matchProcessesToProjects(scanned, processes)

	sel := promptProjectSelection("Go", scanned)
	if sel == nil {
		return nil
	}

	var goProj GoProject
	for _, p := range projects {
		if p.Path == sel.Path {
			goProj = p
			goProj.ScannedProject = *sel
			break
		}
	}

	svcName := projectServiceName(sel.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	return &GoInstrumentationPlan{
		Project: goProj,
		EnvVars: envVars,
	}
}

func (p *GoInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project:    %s\n", p.Project.Path)
	if p.Project.ModuleName != "" {
		fmt.Printf("     Module:     %s\n", p.Project.ModuleName)
	}
	fmt.Println("     SDK integration (manual):")
	for _, pkg := range otelGoPackages {
		fmt.Printf("       go get %s\n", pkg)
	}
}

func (p *GoInstrumentationPlan) Execute() {
	fmt.Println()
	fmt.Printf("  cd %s\n", p.Project.Path)
	fmt.Println()
	fmt.Println("  Add OTel Go SDK dependencies:")
	for _, pkg := range otelGoPackages {
		fmt.Printf("    go get %s\n", pkg)
	}
	fmt.Println()
	fmt.Println("  Set the following environment variables:")
	fmt.Println()
	for _, line := range formatEnvExportLines(p.EnvVars) {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
	fmt.Println("  Initialize the OTel SDK in your main() function.")
	fmt.Println("  See: https://opentelemetry.io/docs/languages/go/getting-started/")
}
