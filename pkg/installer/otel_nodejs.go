package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// otelNodePackages are the npm packages needed for OTel auto-instrumentation.
var otelNodePackages = []string{
	"@opentelemetry/sdk-node",
	"@opentelemetry/auto-instrumentations-node",
	"@opentelemetry/exporter-trace-otlp-http",
}

func detectNodeProjects() []ScannedProject {
	return scanProjectDirs([]string{"package.json"}, []string{"node_modules"})
}

func detectNodeProcesses() []DetectedProcess {
	return detectProcesses("node", []string{"/bin/dtwiz", "npm "})
}

// packageJSON is the minimal structure we read from package.json.
type packageJSON struct {
	Main    string            `json:"main"`
	Scripts map[string]string `json:"scripts"`
}

// detectNodeEntrypoints infers entry point files for a Node.js project from
// package.json fields (main, scripts.start) or convention fallbacks.
func detectNodeEntrypoints(projectPath string) []string {
	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}

	var pkg packageJSON
	_ = json.Unmarshal(data, &pkg)

	if pkg.Main != "" {
		if _, err := os.Stat(filepath.Join(projectPath, pkg.Main)); err == nil {
			return []string{pkg.Main}
		}
	}

	if start, ok := pkg.Scripts["start"]; ok && start != "" {
		// Extract the filename from "node app.js" or "ts-node src/index.ts" etc.
		parts := strings.Fields(start)
		for _, part := range parts {
			if strings.HasSuffix(part, ".js") || strings.HasSuffix(part, ".ts") ||
				strings.HasSuffix(part, ".mjs") || strings.HasSuffix(part, ".cjs") ||
				strings.HasSuffix(part, ".mts") || strings.HasSuffix(part, ".cts") {
				if _, err := os.Stat(filepath.Join(projectPath, part)); err == nil {
					return []string{part}
				}
			}
		}
	}

	// Convention fallbacks: check both .js and .ts variants.
	for _, base := range []string{"index", "app", "server"} {
		for _, ext := range []string{".js", ".ts", ".mjs", ".cjs", ".mts", ".cts"} {
			name := base + ext
			if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
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

// DetectNodePlan scans for Node.js projects, prompts the user, performs
// entrypoint detection, and returns a plan or nil.
func DetectNodePlan(apiURL, token string) *NodeInstrumentationPlan {
	if _, err := exec.LookPath("node"); err != nil {
		return nil
	}

	projects := detectNodeProjects()
	procs := detectNodeProcesses()
	matchProcessesToProjects(projects, procs)

	if len(projects) == 0 {
		return nil
	}

	sel := promptProjectSelection("Node.js", projects)
	if sel == nil {
		return nil
	}
	proj := *sel

	entrypoints := detectNodeEntrypoints(proj.Path)
	if len(entrypoints) == 0 {
		fmt.Printf("  Skipping %s — no Node.js entrypoint found.\n", proj.Path)
		fmt.Println("    Looked for: package.json 'main' or 'scripts.start', or common files (index.js, app.js, server.js and .ts variants).")
		fmt.Println("    Add one of these and re-run dtwiz.")
		return nil
	}

	svcName := serviceNameFromPath(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	return &NodeInstrumentationPlan{
		Project:    proj,
		Entrypoint: entrypoints[0],
		EnvVars:    envVars,
	}
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
	for k, v := range p.EnvVars {
		fmt.Printf("    export %s=%q\n", k, v)
	}
	fmt.Println()
	if p.Entrypoint != "" {
		fmt.Println("  Run your application with auto-instrumentation:")
		fmt.Printf("    node --require @opentelemetry/auto-instrumentations-node/register %s\n", p.Entrypoint)
	}
}
