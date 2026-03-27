package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// otelGoPackages are the go get commands needed for OTel SDK integration.
var otelGoPackages = []string{
	"go.opentelemetry.io/otel",
	"go.opentelemetry.io/otel/sdk",
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
}

// GoProject describes a detected Go project directory.
// It embeds ScannedProject and adds the module name from go.mod.
type GoProject struct {
	ScannedProject
	ModuleName string // module name extracted from go.mod
}

// extractGoModuleName reads the module name from a go.mod file.
func extractGoModuleName(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// detectGoProjects scans common locations for Go project directories (go.mod).
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
	return projects
}

// GoInstrumentationPlan captures a user's Go instrumentation choices.
type GoInstrumentationPlan struct {
	Project       GoProject
	EnvVars       map[string]string
	EnvURL        string
	PlatformToken string
}

func (p *GoInstrumentationPlan) Runtime() string         { return "Go" }
func (p *GoInstrumentationPlan) SetTokens(envURL, platformToken string) {
	p.EnvURL = envURL
	p.PlatformToken = platformToken
}


// DetectGoPlan scans for Go projects, prompts the user, and returns a plan or nil.
func DetectGoPlan(apiURL, token string) *GoInstrumentationPlan {
	if _, err := exec.LookPath("go"); err != nil {
		return nil
	}

	projects := detectGoProjects()
	if len(projects) == 0 {
		return nil
	}

	// Extract ScannedProject slice for shared selection UI.
	scanned := make([]ScannedProject, len(projects))
	for i := range projects {
		scanned[i] = projects[i].ScannedProject
	}

	procs := detectProcesses("go", []string{"/bin/dtwiz"})
	matchProcessesToProjects(scanned, procs)

	sel := promptProjectSelection("Go", scanned)
	if sel == nil {
		return nil
	}

	// Find the matching GoProject to preserve the module name.
	var goProj GoProject
	for _, p := range projects {
		if p.Path == sel.Path {
			goProj = p
			goProj.ScannedProject = *sel // pick up RunningPIDs
			break
		}
	}

	svcName := serviceNameFromPath(sel.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	return &GoInstrumentationPlan{
		Project: goProj,
		EnvVars: envVars,
	}
}

// PrintPlanSteps prints the Go SDK integration steps for a combined plan preview.
// Labeled as "SDK integration (manual)" since Go requires compile-time changes.
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

// Execute prints Go OTel SDK integration guidance.
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
	for k, v := range p.EnvVars {
		fmt.Printf("    export %s=%q\n", k, v)
	}
	fmt.Println()
	fmt.Println("  Initialize the OTel SDK in your main() function.")
	fmt.Println("  See: https://opentelemetry.io/docs/languages/go/getting-started/")
}
