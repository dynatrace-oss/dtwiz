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
	"@opentelemetry/sdk-node",
	"@opentelemetry/auto-instrumentations-node",
	"@opentelemetry/exporter-trace-otlp-http",
}

func detectNodeProjects() []ScannedProject {
	return scanProjectDirs([]string{"package.json"}, []string{"node_modules"})
}

func detectNodeProcesses() []DetectedProcess {
	return detectProcesses("node", []string{"npm "})
}

type packageJSON struct {
	Main    string            `json:"main"`
	Scripts map[string]string `json:"scripts"`
}

func detectNodeEntrypoints(projectPath string) []string {
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
	Project    ScannedProject
	Entrypoint string
	EnvVars    map[string]string
}

func (p *NodeInstrumentationPlan) Runtime() string { return "Node.js" }

func buildNodeInstrumentationPlan(proj ScannedProject, apiURL, token string) *NodeInstrumentationPlan {
	entrypoints := detectNodeEntrypoints(proj.Path)
	if len(entrypoints) == 0 {
		fmt.Printf("  Skipping %s — no Node.js entrypoint found.\n", proj.Path)
		fmt.Println("    Looked for: package.json 'main' or 'scripts.start', or common files (index.js, app.js, server.js and .ts variants).")
		fmt.Println("    Add one of these and re-run dtwiz.")
		return nil
	}

	svcName := projectServiceName(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	return &NodeInstrumentationPlan{
		Project:    proj,
		Entrypoint: entrypoints[0],
		EnvVars:    envVars,
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
	fmt.Printf("     Project:    %s\n", p.Project.Path)
	fmt.Printf("     npm install %s\n", strings.Join(otelNodePackages, " "))
	if p.Entrypoint != "" {
		fmt.Printf("     node --require @opentelemetry/auto-instrumentations-node/register %s\n", p.Entrypoint)
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
