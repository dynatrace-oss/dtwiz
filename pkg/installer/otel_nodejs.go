package installer

import (
	"bufio"
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

// detectNodeProjects scans common locations for Node.js project directories,
// excluding node_modules directories.
func detectNodeProjects() []ScannedProject {
	return scanProjectDirs([]string{"package.json"}, []string{"node_modules"})
}

// detectNodeProcesses finds running node processes.
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

// NodeInstrumentationPlan captures a user's Node.js instrumentation choices.
type NodeInstrumentationPlan struct {
	Project       ScannedProject
	Entrypoint    string
	EnvVars       map[string]string
	EnvURL        string
	PlatformToken string
}

func (p *NodeInstrumentationPlan) Runtime() string         { return "Node.js" }
func (p *NodeInstrumentationPlan) SetTokens(envURL, platformToken string) {
	p.EnvURL = envURL
	p.PlatformToken = platformToken
}


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
	var entrypoint string
	if len(entrypoints) > 0 {
		entrypoint = entrypoints[0]
	} else {
		fmt.Print("  No entrypoint detected. Enter the JS/TS file to run (e.g. app.js): ")
		reader := bufio.NewReader(os.Stdin)
		ep, _ := reader.ReadString('\n')
		ep = strings.TrimSpace(ep)
		if ep == "" {
			return nil
		}
		entrypoint = ep
	}

	svcName := serviceNameFromPath(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	return &NodeInstrumentationPlan{
		Project:    proj,
		Entrypoint: entrypoint,
		EnvVars:    envVars,
	}
}

// PrintPlanSteps prints the Node.js instrumentation steps for a combined plan preview.
func (p *NodeInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project:    %s\n", p.Project.Path)
	fmt.Printf("     npm install %s\n", strings.Join(otelNodePackages, " "))
	if p.Entrypoint != "" {
		fmt.Printf("     node --require @opentelemetry/auto-instrumentations-node/register %s\n", p.Entrypoint)
	}
}

// Execute prints Node.js instrumentation instructions (manual steps).
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
