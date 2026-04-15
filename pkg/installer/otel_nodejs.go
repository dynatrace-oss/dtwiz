package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var otelNodePackages = []string{
	"@opentelemetry/auto-instrumentations-node",
	"@opentelemetry/sdk-node",
	"@opentelemetry/exporter-trace-otlp-http",
	"@opentelemetry/exporter-metrics-otlp-http",
	"@opentelemetry/exporter-logs-otlp-http",
}

func detectNodeProjects() []ScannedProject {
	projects := scanProjectDirs([]string{"package.json"}, []string{"node_modules"})

	// Expand monorepo workspaces: for each project with a "workspaces" field,
	// resolve workspace directories and add them as individual projects.
	var expanded []ScannedProject
	seen := make(map[string]bool)
	for _, p := range projects {
		seen[p.Path] = true
	}
	for _, p := range projects {
		dirs := resolveWorkspaces(p.Path)
		for _, dir := range dirs {
			if !seen[dir] {
				seen[dir] = true
				expanded = append(expanded, ScannedProject{
					Path:    dir,
					Markers: []string{"package.json"},
				})
			}
		}
	}

	return append(projects, expanded...)
}

func detectNodeProcesses() []DetectedProcess {
	return detectProcesses("node", []string{"npm "})
}

type packageJSON struct {
	Main            string            `json:"main"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

// isNextJSProject checks for Next.js config files or next in package.json dependencies.
func isNextJSProject(projectPath string) bool {
	for _, name := range []string{"next.config.js", "next.config.ts", "next.config.mjs"} {
		if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
			return true
		}
	}
	return hasDependency(projectPath, "next")
}

// isNuxtProject checks for Nuxt config files or nuxt in package.json dependencies.
func isNuxtProject(projectPath string) bool {
	for _, name := range []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "nuxt.config.mts"} {
		if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
			return true
		}
	}
	return hasDependency(projectPath, "nuxt")
}

// hasDependency checks if a package name appears in dependencies or devDependencies.
func hasDependency(projectPath, pkgName string) bool {
	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return false
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	if _, ok := pkg.Dependencies[pkgName]; ok {
		return true
	}
	if _, ok := pkg.DevDependencies[pkgName]; ok {
		return true
	}
	return false
}

// detectNodeFramework returns "next", "nuxt", or "" for a project directory.
// Next.js takes precedence when both are detected.
func detectNodeFramework(projectPath string) string {
	if isNextJSProject(projectPath) {
		return "next"
	}
	if isNuxtProject(projectPath) {
		return "nuxt"
	}
	return ""
}

// detectNodePackageManager detects the package manager from lockfiles.
func detectNodePackageManager(projectPath string) string {
	if _, err := os.Stat(filepath.Join(projectPath, "package-lock.json")); err == nil {
		return "npm"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "yarn.lock")); err == nil {
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	return "npm"
}

// resolveWorkspaces parses the workspaces field from package.json and returns
// workspace directories that contain their own package.json.
func resolveWorkspaces(projectPath string) []string {
	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	if pkg.Workspaces == nil {
		return nil
	}

	var patterns []string

	// Try as array of strings: "workspaces": ["packages/*"]
	var arr []string
	if err := json.Unmarshal(pkg.Workspaces, &arr); err == nil {
		patterns = arr
	} else {
		// Try as object: "workspaces": {"packages": ["packages/*"]}
		var obj struct {
			Packages []string `json:"packages"`
		}
		if err := json.Unmarshal(pkg.Workspaces, &obj); err == nil {
			patterns = obj.Packages
		}
	}

	if len(patterns) == 0 {
		return nil
	}

	var workspaceDirs []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectPath, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(match, "package.json")); err == nil {
				workspaceDirs = append(workspaceDirs, match)
			}
		}
	}
	return workspaceDirs
}

// generateOtelNodeEnvVars extends base OTel env vars with Node.js-specific settings.
func generateOtelNodeEnvVars(apiURL, token, serviceName string) map[string]string {
	envVars := generateBaseOtelEnvVars(apiURL, token, serviceName)
	envVars["OTEL_NODE_RESOURCE_DETECTORS"] = "all"
	return envVars
}

func detectNodeEntrypoints(projectPath string) []string {
	// For framework projects, return marker entrypoints.
	framework := detectNodeFramework(projectPath)
	if framework == "next" {
		logger.Debug("node entrypoint: Next.js project detected", "path", projectPath)
		return []string{"next:start"}
	}
	if framework == "nuxt" {
		logger.Debug("node entrypoint: Nuxt project detected", "path", projectPath)
		return []string{"nuxt:start"}
	}

	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}

	var pkg packageJSON
	_ = json.Unmarshal(data, &pkg)

	if pkg.Main != "" {
		logger.Debug("node entrypoint: checking 'main' field", "main", pkg.Main)
		if _, err := os.Stat(filepath.Join(projectPath, pkg.Main)); err == nil {
			logger.Debug("node entrypoint found via 'main'", "file", pkg.Main)
			return []string{pkg.Main}
		}
	}

	if start, ok := pkg.Scripts["start"]; ok && start != "" {
		logger.Debug("node entrypoint: checking 'scripts.start'", "start", start)
		parts := strings.Fields(start)
		for _, part := range parts {
			if strings.HasSuffix(part, ".js") || strings.HasSuffix(part, ".ts") ||
				strings.HasSuffix(part, ".mjs") || strings.HasSuffix(part, ".cjs") ||
				strings.HasSuffix(part, ".mts") || strings.HasSuffix(part, ".cts") {
				if _, err := os.Stat(filepath.Join(projectPath, part)); err == nil {
					logger.Debug("node entrypoint found via 'scripts.start'", "file", part)
					return []string{part}
				}
			}
		}
	}

	for _, base := range []string{"index", "app", "server"} {
		for _, ext := range []string{".js", ".ts", ".mjs", ".cjs", ".mts", ".cts"} {
			name := base + ext
			if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
				logger.Debug("node entrypoint found via fallback", "file", name)
				return []string{name}
			}
		}
	}

	return nil
}

type NodeInstrumentationPlan struct {
	Project        ScannedProject
	Entrypoint     string
	EnvVars        map[string]string
	PackageManager string
	OtelDir        string
	Framework      string
	EnvURL         string
	PlatformToken  string
}

func (p *NodeInstrumentationPlan) Runtime() string { return "Node.js" }

func buildNodeInstrumentationPlan(proj ScannedProject, apiURL, token string) *NodeInstrumentationPlan {
	framework := detectNodeFramework(proj.Path)
	entrypoints := detectNodeEntrypoints(proj.Path)
	if len(entrypoints) == 0 && framework == "" {
		fmt.Printf("  Skipping %s — no Node.js entrypoint found.\n", proj.Path)
		fmt.Println("    Looked for: package.json 'main' or 'scripts.start', or common files (index.js, app.js, server.js and .ts variants).")
		fmt.Println("    Add one of these and re-run dtwiz.")
		return nil
	}

	svcName := projectServiceName(proj.Path)
	envVars := generateOtelNodeEnvVars(apiURL, token, svcName)
	pkgManager := detectNodePackageManager(proj.Path)
	otelDir := filepath.Join(proj.Path, ".otel")

	return &NodeInstrumentationPlan{
		Project:        proj,
		Entrypoint:     entrypoints[0],
		EnvVars:        envVars,
		PackageManager: pkgManager,
		OtelDir:        otelDir,
		Framework:      framework,
	}
}

func DetectNodePlan(apiURL, token string) *NodeInstrumentationPlan {
	if _, err := exec.LookPath("node"); err != nil {
		logger.Debug("node not found on PATH", "skipping Node.js instrumentation")
		return nil
	}

	projects, processes := runInParallel(detectNodeProjects, detectNodeProcesses)
	matchProcessesToProjects(projects, processes)

	if len(projects) == 0 {
		logger.Debug("no Node.js projects detected", "skipping Node.js instrumentation")
		return nil
	}

	sel := promptProjectSelection("Node.js", projects)
	if sel == nil {
		return nil
	}

	return buildNodeInstrumentationPlan(*sel, apiURL, token)
}

func (p *NodeInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project:         %s\n", p.Project.Path)
	fmt.Printf("     Package manager: %s\n", p.PackageManager)
	if p.Framework != "" {
		fmt.Printf("     Framework:       %s\n", p.Framework)
	}
	fmt.Printf("     Create %s/ with OTel deps\n", p.OtelDir)
	fmt.Printf("     npm install (in .otel/)\n")
	switch p.Framework {
	case "next":
		fmt.Printf("     node .otel/next-register.js start\n")
	case "nuxt":
		fmt.Printf("     node .otel/nuxt-register.js start\n")
	default:
		if p.Entrypoint != "" {
			fmt.Printf("     node --require @opentelemetry/auto-instrumentations-node/register %s\n", p.Entrypoint)
		}
	}
}

func (p *NodeInstrumentationPlan) Execute() {
	fmt.Println()
	fmt.Printf("  Install OTel packages:\n")
	fmt.Printf("    cd %s\n", p.Project.Path)
	fmt.Printf("    npm install %s\n", strings.Join(otelNodePackages, " "))
	fmt.Println()
	fmt.Println("  Set the following environment variables:")
	fmt.Println()
	for _, line := range formatEnvExportLines(p.EnvVars) {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
	if p.Entrypoint != "" {
		fmt.Println("  Run your application with auto-instrumentation:")
		fmt.Printf("    node --require @opentelemetry/auto-instrumentations-node/register %s\n", p.Entrypoint)
	}
}
