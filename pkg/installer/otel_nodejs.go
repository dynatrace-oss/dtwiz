package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"

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

// isJSFileExtension checks if a string ends with a JavaScript/TypeScript file extension.
func isJSFileExtension(s string) bool {
	return strings.HasSuffix(s, ".js") || strings.HasSuffix(s, ".ts") ||
		strings.HasSuffix(s, ".mjs") || strings.HasSuffix(s, ".cjs") ||
		strings.HasSuffix(s, ".mts") || strings.HasSuffix(s, ".cts")
}

// extractScriptFile extracts a file reference from a script command string.
// It looks for tokens ending in .js/.ts/.mjs/.cjs/.mts/.cts that exist on disk.
func extractScriptFile(projectPath, script string) string {
	parts := strings.Fields(script)
	for _, part := range parts {
		if isJSFileExtension(part) {
			if _, err := os.Stat(filepath.Join(projectPath, part)); err == nil {
				return part
			}
		}
	}
	return ""
}

// runtimeScriptPrefixes lists package.json script name prefixes that indicate
// a server/runtime entrypoint (as opposed to build/lint/test tooling). Only
// scripts matching one of these prefixes are considered when scanning "other
// scripts" for entrypoints.
var runtimeScriptPrefixes = []string{"start", "dev", "serve", "server"}

// isRuntimeScript checks if a package.json script name looks like a runtime
// entrypoint (e.g. "start:api", "dev", "serve:frontend") rather than a
// build/lint/test script.
func isRuntimeScript(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range runtimeScriptPrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+":") {
			return true
		}
	}
	return false
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
		if found := extractScriptFile(projectPath, start); found != "" {
			logger.Debug("node entrypoint found via 'scripts.start'", "file", found)
			return []string{found}
		}
	}

	// Scan runtime-like scripts (start:*, dev:*, serve:*, server:*) for file
	// references. Non-runtime scripts (build, lint, test, etc.) are skipped to
	// avoid picking up config files as entrypoints.
	if len(pkg.Scripts) > 0 {
		seen := make(map[string]bool)
		var otherEntrypoints []string
		for name, script := range pkg.Scripts {
			if name == "start" || script == "" {
				continue
			}
			if !isRuntimeScript(name) {
				continue
			}
			if found := extractScriptFile(projectPath, script); found != "" && !seen[found] {
				seen[found] = true
				otherEntrypoints = append(otherEntrypoints, found)
				logger.Debug("node entrypoint found via script", "script", name, "file", found)
			}
		}
		if len(otherEntrypoints) > 0 {
			sort.Strings(otherEntrypoints)
			return otherEntrypoints
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
	Entrypoints    []string
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
		fmt.Println("    Looked for: package.json 'main', 'scripts.start', other scripts with file references, or common files (index.js, app.js, server.js and .ts variants).")
		fmt.Println("    Add one of these and re-run dtwiz.")
		return nil
	}

	svcName := projectServiceName(proj.Path)
	envVars := generateOtelNodeEnvVars(apiURL, token, svcName)
	pkgManager := detectNodePackageManager(proj.Path)
	otelDir := filepath.Join(proj.Path, ".otel")

	return &NodeInstrumentationPlan{
		Project:        proj,
		Entrypoints:    entrypoints,
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
	if len(p.Project.RunningProcessIDs) > 0 {
		pidStrs := make([]string, len(p.Project.RunningProcessIDs))
		for i, pid := range p.Project.RunningProcessIDs {
			pidStrs[i] = strconv.Itoa(pid)
		}
		fmt.Printf("     Stop running processes (PIDs: %s)\n", strings.Join(pidStrs, ", "))
	}
	fmt.Printf("     Package manager: %s\n", p.PackageManager)
	if p.Framework != "" {
		fmt.Printf("     Framework:       %s\n", p.Framework)
	}
	fmt.Printf("     Create %s/ with OTel deps\n", p.OtelDir)
	fmt.Printf("     npm install (in .otel/)\n")
	switch p.Framework {
	case "next":
		fmt.Printf("     node .otel/next-otel-bootstrap.js start\n")
	case "nuxt":
		fmt.Printf("     node --import .otel/nuxt-otel-bootstrap.mjs .output/server/index.mjs\n")
	default:
		for _, ep := range p.Entrypoints {
			svcName := serviceNameFromEntrypoint(p.Project.Path, ep)
			fmt.Printf("     node --require @opentelemetry/auto-instrumentations-node/register %s  (service: %s)\n", ep, svcName)
		}
	}
}

// createOtelDir creates the .otel/ directory and writes a package.json with OTel dependencies.
func createOtelDir(plan *NodeInstrumentationPlan) error {
	if err := os.MkdirAll(plan.OtelDir, 0755); err != nil {
		return fmt.Errorf("create .otel/ directory: %w", err)
	}

	deps := make(map[string]string, len(otelNodePackages))
	for _, pkg := range otelNodePackages {
		deps[pkg] = "latest"
	}

	pkgJSON := map[string]interface{}{
		"name":         "otel-instrumentation",
		"private":      true,
		"dependencies": deps,
	}

	data, err := json.MarshalIndent(pkgJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .otel/package.json: %w", err)
	}

	pkgPath := filepath.Join(plan.OtelDir, "package.json")
	if err := os.WriteFile(pkgPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("write .otel/package.json: %w", err)
	}

	return nil
}

// generateWrapperJS generates a CJS wrapper script that requires the
// auto-instrumentation register module and delegates to the framework CLI.
// OTEL_* env vars are NOT embedded in the script — they are passed via cmd.Env
// at launch time, which sets process.env before any JS code executes. This
// avoids writing secrets (e.g. API tokens in OTEL_EXPORTER_OTLP_HEADERS) to disk.
// Only used for Next.js — Nuxt bypasses the CLI and runs the Nitro server directly.
func generateWrapperJS(framework string) string {
	var sb strings.Builder
	sb.WriteString("// Auto-generated by dtwiz — do not edit\n")
	sb.WriteString("'use strict';\n\n")

	// Require auto-instrumentation register
	sb.WriteString("require('@opentelemetry/auto-instrumentations-node/register');\n\n")

	// Delegate to framework CLI
	if framework == "next" {
		// Next.js bin is CJS — require() works directly.
		sb.WriteString("require('next/dist/bin/next');\n")
	}

	return sb.String()
}

// generateNuxtBootstrapMJS generates an ESM bootstrap script (.mjs) for Nuxt projects.
// The Nitro server is ESM, so CJS-only require() hooks cannot intercept ESM imports
// like 'node:http'. This script uses module.register() to install ESM loader hooks
// (import-in-the-middle) before loading the CJS OTel register, ensuring both CJS and
// ESM modules are instrumented.
func generateNuxtBootstrapMJS(otelDir string) string {
	hookRel := filepath.ToSlash(filepath.Join(otelDir, "node_modules", "@opentelemetry", "instrumentation", "hook.mjs"))
	registerRel := filepath.ToSlash(filepath.Join(otelDir, "node_modules", "@opentelemetry",
		"auto-instrumentations-node", "build", "src", "register.js"))

	var sb strings.Builder
	sb.WriteString("// Auto-generated by dtwiz — do not edit\n")
	sb.WriteString("import { register } from 'node:module';\n")
	sb.WriteString("import { pathToFileURL } from 'node:url';\n")
	sb.WriteString("import { createRequire } from 'node:module';\n\n")

	// Register ESM loader hooks (import-in-the-middle) before any app code loads.
	sb.WriteString(fmt.Sprintf("register(pathToFileURL('%s'), pathToFileURL('./'));\n\n", hookRel))

	// Load the CJS OTel auto-instrumentation register.
	// Use CWD (project root) as the base so .otel/node_modules/... paths resolve correctly.
	sb.WriteString("const require = createRequire(pathToFileURL('./'));\n")
	sb.WriteString(fmt.Sprintf("require('%s');\n", registerRel))

	return sb.String()
}

// installOtelNodeDeps runs npm install inside the .otel/ directory.
func installOtelNodeDeps(otelDir string) error {
	npmBin := "npm"
	if runtime.GOOS == "windows" {
		npmBin = "npm.cmd"
	}
	cmd := exec.Command(npmBin, "install")
	cmd.Dir = otelDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm install in %s failed: %w\n%s", otelDir, err, string(out))
	}
	return nil
}

func (p *NodeInstrumentationPlan) Execute() {
	proj := p.Project

	// Validate prerequisites before doing any work. Nuxt requires a pre-built
	// Nitro server; fail fast instead of creating .otel/ and running npm install
	// only to discover the build output is missing.
	if p.Framework == "nuxt" {
		nitroEntry := filepath.Join(proj.Path, ".output", "server", "index.mjs")
		if _, err := os.Stat(nitroEntry); err != nil {
			fmt.Printf("    Nuxt build output not found at %s\n", nitroEntry)
			fmt.Println("    Run 'npx nuxt build' first, then re-run dtwiz.")
			return
		}
	}

	if len(proj.RunningProcessIDs) > 0 {
		fmt.Print("  Stopping running processes... ")
		stopProcesses(proj.RunningProcessIDs)
		fmt.Println("done.")
	}

	fmt.Print("  Creating .otel/ directory... ")
	if err := createOtelDir(p); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return
	}
	fmt.Println("done.")

	// For Next.js, write a CJS wrapper script. Nuxt bypasses the CLI entirely
	// (nuxt preview spawns a child process that loses OTel registration),
	// so we generate an ESM bootstrap script that uses module.register() for ESM hooks.
	if p.Framework == "next" {
		scriptName := "next-otel-bootstrap.js"
		scriptPath := filepath.Join(p.OtelDir, scriptName)
		fmt.Printf("  Writing %s... ", scriptName)
		content := generateWrapperJS(p.Framework)
		if err := os.WriteFile(scriptPath, []byte(content), 0600); err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			return
		}
		fmt.Println("done.")
	}
	if p.Framework == "nuxt" {
		scriptName := "nuxt-otel-bootstrap.mjs"
		scriptPath := filepath.Join(p.OtelDir, scriptName)
		fmt.Printf("  Writing %s... ", scriptName)
		content := generateNuxtBootstrapMJS(p.OtelDir)
		if err := os.WriteFile(scriptPath, []byte(content), 0600); err != nil {
			fmt.Println("failed.")
			fmt.Printf("    %v\n", err)
			return
		}
		fmt.Println("done.")
	}

	fmt.Print("  Installing OTel packages (npm install)... ")
	if err := installOtelNodeDeps(p.OtelDir); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return
	}
	fmt.Println("done.")

	fmt.Println()
	var procs []*ManagedProcess

	switch p.Framework {
	case "next":
		svcName := projectServiceName(proj.Path)
		epEnvVars := copyEnvVars(p.EnvVars)
		epEnvVars["OTEL_SERVICE_NAME"] = svcName

		cmd := exec.Command("node", filepath.Join(".otel", "next-otel-bootstrap.js"), "start")
		cmd.Dir = proj.Path
		cmd.Env = append(os.Environ(), formatEnvVars(epEnvVars)...)

		mp := launchEntrypoint(svcName, proj.Path, "next:start", cmd)
		if mp != nil {
			procs = append(procs, mp)
		}
	case "nuxt":
		svcName := projectServiceName(proj.Path)
		epEnvVars := copyEnvVars(p.EnvVars)
		epEnvVars["OTEL_SERVICE_NAME"] = svcName

		// Nuxt CLI "preview/start" spawns a child process (via tinyexec) to run
		// the built Nitro server, so OTel registration in the parent is lost.
		// The Nitro server is ESM, so CJS-only --require hooks can't intercept
		// ESM imports like 'node:http'. We run the Nitro server directly with
		// --import of an ESM bootstrap that uses module.register() for ESM hooks.
		nitroEntry := filepath.Join(proj.Path, ".output", "server", "index.mjs")

		bootstrap := filepath.Join(p.OtelDir, "nuxt-otel-bootstrap.mjs")
		cmd := exec.Command("node", "--import", bootstrap, nitroEntry)
		cmd.Dir = proj.Path
		cmd.Env = append(os.Environ(), formatEnvVars(epEnvVars)...)

		mp := launchEntrypoint(svcName, proj.Path, nitroEntry, cmd)
		if mp != nil {
			procs = append(procs, mp)
		}
	default:
		for _, ep := range p.Entrypoints {
			svcName := serviceNameFromEntrypoint(proj.Path, ep)
			epEnvVars := copyEnvVars(p.EnvVars)
			epEnvVars["OTEL_SERVICE_NAME"] = svcName

			relEntrypoint := filepath.Join("..", ep)
			cmd := exec.Command("node", "--require", "@opentelemetry/auto-instrumentations-node/register", relEntrypoint)
			cmd.Dir = p.OtelDir
			cmd.Env = append(os.Environ(), formatEnvVars(epEnvVars)...)

			mp := launchEntrypoint(svcName, proj.Path, ep, cmd)
			if mp != nil {
				procs = append(procs, mp)
			}
		}
	}

	startedServices, _ := PrintProcessSummary(procs, processSettleDelay)

	if len(startedServices) == 0 {
		fmt.Println()
		fmt.Println("  No services are running — check the logs above for errors.")
		return
	}

	fmt.Println()
	fmt.Println("  Waiting for traffic — send requests to your services to generate traces and metrics.")
}

// copyEnvVars returns a shallow copy of the env vars map.
func copyEnvVars(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// launchEntrypoint starts a managed process for a single entrypoint.
func launchEntrypoint(svcName, projectPath, entrypoint string, cmd *exec.Cmd) *ManagedProcess {
	logName := svcName + ".log"
	logPath := filepath.Join(projectPath, logName)
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Printf("    Failed to create log file %s: %v\n", logPath, err)
		return nil
	}

	mp, err := StartManagedProcess(svcName, logName, entrypoint, cmd, logFile)
	if err != nil {
		fmt.Printf("    Failed to start %s: %v\n", svcName, err)
		return nil
	}
	return mp
}

func InstallOtelNode(envURL, token, platformToken, serviceName string, dryRun bool) error {
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("node not found — install Node.js and ensure it is in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found — install npm and ensure it is in PATH")
	}

	apiURL := APIURL(envURL)
	if serviceName == "" {
		serviceName = "my-service"
	}

	envVars := generateOtelNodeEnvVars(apiURL, token, serviceName)

	if dryRun {
		fmt.Println("[dry-run] Would set up OpenTelemetry Node.js auto-instrumentation")
		fmt.Printf("  API URL:      %s\n", apiURL)
		fmt.Printf("  Service name: %s\n", serviceName)
		fmt.Println("  Packages to install (in .otel/ directory):")
		for _, pkg := range otelNodePackages {
			fmt.Printf("    %s\n", pkg)
		}
		fmt.Println()
		fmt.Println("  Environment variables:")
		for _, line := range formatPrintableEnvVars(envVars) {
			fmt.Printf("    %s\n", line)
		}
		return nil
	}

	cyan := color.New(color.FgMagenta)
	sep := strings.Repeat("─", 60)

	fmt.Println()
	cyan.Println("  Dynatrace Node.js Auto-Instrumentation")
	fmt.Println("  " + sep)

	plan := DetectNodePlan(apiURL, token)
	if plan == nil {
		fmt.Println()
		fmt.Println("  No Node.js projects detected. Make sure you are in or near a project directory")
		fmt.Println("  containing a package.json with a recognizable entrypoint.")
		return nil
	}

	fmt.Println()
	cyan.Println("  Steps:")
	plan.PrintPlanSteps()

	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	plan.EnvURL = envURL
	plan.PlatformToken = platformToken
	plan.EnvVars = envVars

	fmt.Printf("\n  ── Node.js auto-instrumentation ──\n\n")
	plan.Execute()

	return nil
}
