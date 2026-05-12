package installer

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var otelNodePackages = []string{
	"@opentelemetry/auto-instrumentations-node",
	"@opentelemetry/sdk-node",
	"@opentelemetry/exporter-trace-otlp-http",
	"@opentelemetry/exporter-metrics-otlp-http",
	"@opentelemetry/exporter-logs-otlp-http",
}

// generateOtelNodeEnvVars extends base OTel env vars with Node.js-specific settings.
func generateOtelNodeEnvVars(apiURL, token, serviceName string) map[string]string {
	envVars := generateBaseOtelEnvVars(apiURL, token, serviceName)
	envVars["OTEL_NODE_RESOURCE_DETECTORS"] = "all"
	return envVars
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
		docLink := termLink(
			"Instrument your JavaScript application on Node.js with OpenTelemetry",
			"https://docs.dynatrace.com/docs/ingest-from/opentelemetry/walkthroughs/nodejs",
		)
		fmt.Println()
		fmt.Println("  This project can't be auto-instrumented.")
		fmt.Printf("  See %s to instrument it manually.\n", docLink)
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

func DetectNodePlan(apiURL, token string) (*NodeInstrumentationPlan, bool) {
	if _, err := exec.LookPath("node"); err != nil {
		logger.Debug("node not found on PATH", "skipping Node.js instrumentation")
		return nil, false
	}

	projects, processes := runInParallel(detectNodeProjects, detectNodeProcesses)
	matchProcessesToProjects(projects, processes)

	if len(projects) == 0 {
		logger.Debug("no Node.js projects detected", "skipping Node.js instrumentation")
		return nil, false
	}

	for {
		sel := promptProjectSelection("Node.js", projects)
		if sel == nil {
			return nil, true
		}

		plan := buildNodeInstrumentationPlan(*sel, apiURL, token)
		if plan != nil {
			return plan, true
		}

		// Project can't be auto-instrumented; ask if the user wants to try another.
		ok, err := confirmProceed("  Select another project?")
		if err != nil || !ok {
			return nil, true
		}
	}
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
	logger.Debug("creating .otel/ directory", "dir", plan.OtelDir)
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

	logger.Debug("wrote .otel/package.json", "path", filepath.Join(plan.OtelDir, "package.json"))
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
	var sb strings.Builder
	sb.WriteString("// Auto-generated by dtwiz — do not edit\n")
	sb.WriteString("import { register } from 'node:module';\n")
	sb.WriteString("import { createRequire } from 'node:module';\n\n")

	// Resolve paths relative to this script's location (.otel/) using import.meta.url.
	// This works on Windows, macOS, and Linux — import.meta.url is always a valid file:// URL.
	sb.WriteString("const hookURL = new URL('./node_modules/@opentelemetry/instrumentation/hook.mjs', import.meta.url);\n")
	sb.WriteString("register(hookURL, import.meta.url);\n\n")

	// Load the CJS OTel auto-instrumentation register.
	sb.WriteString("const require = createRequire(import.meta.url);\n")
	sb.WriteString("require('./node_modules/@opentelemetry/auto-instrumentations-node/build/src/register.js');\n")

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
	logger.Debug("running npm install", "dir", otelDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm install in %s failed: %w\n%s", otelDir, err, string(out))
	}
	logger.Debug("npm install completed", "dir", otelDir)
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
		logger.Debug("nuxt build output found", "path", nitroEntry)
	}

	// For regular and Next.js apps, verify that project dependencies are installed.
	// Nuxt is exempt — it runs a pre-built .output/server/index.mjs and does not
	// need the project's node_modules/ at runtime.
	if p.Framework == "" || p.Framework == "next" {
		nodeModulesDir := filepath.Join(proj.Path, "node_modules")
		if _, err := os.Stat(nodeModulesDir); os.IsNotExist(err) {
			fmt.Println()
			fmt.Printf("    Project dependencies are not installed in %s\n", proj.Path)
			fmt.Printf("    Run '%s install' in that directory first, then re-run dtwiz.\n", p.PackageManager)
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
		logger.Debug("writing bootstrap script", "path", scriptPath, "framework", p.Framework)
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
		logger.Debug("writing bootstrap script", "path", scriptPath, "framework", p.Framework)
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
		epEnvVars := maps.Clone(p.EnvVars)
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
		epEnvVars := maps.Clone(p.EnvVars)
		epEnvVars["OTEL_SERVICE_NAME"] = svcName

		// Nuxt CLI "preview/start" spawns a child process (via tinyexec) to run
		// the built Nitro server, so OTel registration in the parent is lost.
		// The Nitro server is ESM, so CJS-only --require hooks can't intercept
		// ESM imports like 'node:http'. We run the Nitro server directly with
		// --import of an ESM bootstrap that uses module.register() for ESM hooks.
		nitroEntry := filepath.Join(".output", "server", "index.mjs")

		// Use a relative path for --import to avoid Windows ESM loader rejecting
		// raw absolute paths like C:\...  (ERR_UNSUPPORTED_ESM_URL_SCHEME).
		// CWD is set to proj.Path, so .otel/ is directly accessible.
		bootstrap := "./" + filepath.ToSlash(filepath.Join(".otel", "nuxt-otel-bootstrap.mjs"))
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
			epEnvVars := maps.Clone(p.EnvVars)
			epEnvVars["OTEL_SERVICE_NAME"] = svcName

			relEntrypoint := "../" + filepath.ToSlash(ep)
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

// launchEntrypoint starts a managed process for a single entrypoint.
func launchEntrypoint(svcName, projectPath, entrypoint string, cmd *exec.Cmd) *ManagedProcess {
	logName := svcName + ".log"
	logPath := filepath.Join(projectPath, logName)
	logger.Debug("launching entrypoint", "service", svcName, "entrypoint", entrypoint, "logFile", logPath)
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

func InstallOtelNode(envURL, token, platformToken, serviceName, projectPath string, dryRun bool) error {
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

	fmt.Println()
	display.Header("Dynatrace Node.js Auto-Instrumentation")

	var plan *NodeInstrumentationPlan
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err != nil {
			return fmt.Errorf("project path not found: %s", projectPath)
		}
		plan = buildNodeInstrumentationPlan(ScannedProject{Path: projectPath, Markers: []string{"package.json"}}, apiURL, token)
		if plan == nil {
			return nil
		}
	} else {
		var userInteracted bool
		plan, userInteracted = DetectNodePlan(apiURL, token)
		if plan == nil {
			if !userInteracted {
				fmt.Println()
				fmt.Println("  No Node.js projects detected. Make sure you are in or near a project directory")
				fmt.Println("  containing a package.json with a recognizable entrypoint.")
			}
			return nil
		}
	}

	fmt.Println()
	display.ColorMessage.Println("  Steps:")
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
