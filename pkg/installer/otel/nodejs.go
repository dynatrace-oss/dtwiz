package otel

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
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var otelNodePackages = []string{
	"@opentelemetry/auto-instrumentations-node",
	"@opentelemetry/sdk-node",
	"@opentelemetry/exporter-trace-otlp-http",
	"@opentelemetry/exporter-metrics-otlp-http",
	"@opentelemetry/exporter-logs-otlp-http",
}

// nodeServiceNameFromEntrypoint derives OTEL_SERVICE_NAME for a Node.js entrypoint.
// Unlike the Python variant, the project name is NOT prefixed when the entrypoint
// lives in a subdirectory — the subdirectory name alone is the service name.
// Examples:
//
//	"index.js"              in "my-app"  → "my-app"
//	"s-load-balancer/index.js" in "my-app" → "s-load-balancer"
func nodeServiceNameFromEntrypoint(projectPath, entrypoint string) string {
	dir := filepath.Dir(entrypoint)
	if dir == "." || dir == "" {
		return filepath.Base(projectPath)
	}
	return filepath.Base(dir)
}

// generateOtelNodeEnvVars extends base OTel env vars with Node.js-specific settings.
func generateOtelNodeEnvVars(collectorEndpoint, serviceName string) map[string]string {
	envVars := generateBaseOtelEnvVars(collectorEndpoint, serviceName)
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

func buildNodeInstrumentationPlan(proj ScannedProject, collectorEndpoint string) *NodeInstrumentationPlan {
	framework := detectNodeFramework(proj.Path)
	entrypoints := detectNodeEntrypoints(proj.Path)
	if len(entrypoints) == 0 && framework == "" {
		docLink := termLink(
			"Instrument your JavaScript application on Node.js with OpenTelemetry",
			"https://docs.dynatrace.com/docs/ingest-from/opentelemetry/walkthroughs/nodejs",
		)
		fmt.Println()
		fmt.Println("  This project can't be auto-instrumented because of an unsupported runtime. Only Node.js/Next.js/Nuxt are supported at the moment.")
		fmt.Printf("  See %s to instrument it manually.\n", docLink)
		return nil
	}

	svcName := projectServiceName(proj.Path)
	envVars := generateOtelNodeEnvVars(collectorEndpoint, svcName)
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

func DetectNodePlan(collectorEndpoint string) (*NodeInstrumentationPlan, bool) {
	if _, err := exec.LookPath("node"); err != nil {
		logger.Debug("node not found on PATH", "skipping Node.js instrumentation")
		return nil, false
	}

	projects, processes := runInParallel(
		func() []ScannedProject { return detectNodeProjects(defaultScanRoots()) },
		detectNodeProcesses,
	)
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

		plan := buildNodeInstrumentationPlan(*sel, collectorEndpoint)
		if plan != nil {
			return plan, true
		}

		// Project can't be auto-instrumented; ask if the user wants to try another.
		ok, err := installer.ConfirmProceed("  Select another project?")
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
	fmt.Printf("     npm install (in project dir, if node_modules/ missing)\n")
	switch p.Framework {
	case "next":
		if _, err := os.Stat(filepath.Join(p.Project.Path, ".next")); os.IsNotExist(err) {
			fmt.Printf("     npm run build (produce .next/)\n")
		}
	case "nuxt":
		nitroEntry := filepath.Join(p.Project.Path, ".output", "server", "index.mjs")
		if _, err := os.Stat(nitroEntry); os.IsNotExist(err) {
			fmt.Printf("     npm run build (produce .output/server/index.mjs)\n")
		}
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
			svcName := nodeServiceNameFromEntrypoint(p.Project.Path, ep)
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

	// Exit with code 1 when the port is already in use so dtwiz can report the
	// crash. This handler is registered before Nitro's own uncaughtException
	// handler, so it fires first and terminates the process before the heartbeat
	// timer (or any other async work) keeps it alive indefinitely.
	sb.WriteString("process.on('uncaughtException', (err) => {\n")
	sb.WriteString("  if (err.code === 'EADDRINUSE') {\n")
	sb.WriteString("    process.stderr.write('Error: ' + err.message + '\\n');\n")
	sb.WriteString("    process.exit(1);\n")
	sb.WriteString("  } else {\n")
	sb.WriteString("    throw err;\n")
	sb.WriteString("  }\n")
	sb.WriteString("});\n\n")

	// Resolve paths relative to this script's location (.otel/) using import.meta.url.
	// This works on Windows, macOS, and Linux — import.meta.url is always a valid file:// URL.
	sb.WriteString("const hookURL = new URL('./node_modules/@opentelemetry/instrumentation/hook.mjs', import.meta.url);\n")
	sb.WriteString("register(hookURL, import.meta.url);\n\n")

	// Load the CJS OTel auto-instrumentation register.
	sb.WriteString("const require = createRequire(import.meta.url);\n")
	sb.WriteString("require('./node_modules/@opentelemetry/auto-instrumentations-node/build/src/register.js');\n")

	return sb.String()
}

// npmCmd returns the npm binary name for the current platform.
func npmCmd() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

// npmRunner is the function used to execute npm commands.
// Tests replace it with a stub to avoid hitting the real npm binary.
var npmRunner = func(dir string, args ...string) error {
	subCmd := strings.Join(args, " ")
	cmd := exec.Command(npmCmd(), args...)
	cmd.Dir = dir
	logger.Debug("running npm "+subCmd, "dir", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm %s in %s failed: %w\n%s", subCmd, dir, err, string(out))
	}
	logger.Debug("npm "+subCmd+" completed", "dir", dir)
	return nil
}

// runNpm executes `npm <args...>` in dir and returns a descriptive error on failure.
func runNpm(dir string, args ...string) error {
	return npmRunner(dir, args...)
}

// installNodeProjectDeps installs project dependencies if node_modules is missing.
// If package-lock.json is present it runs npm ci; otherwise npm install.
// Returns nil immediately when node_modules already exists.
func installNodeProjectDeps(projPath string) error {
	nodeModulesDir := filepath.Join(projPath, "node_modules")
	if _, err := os.Stat(nodeModulesDir); err == nil {
		return nil // already installed
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check node_modules: %w", err)
	}

	subCmd := "install"
	if _, err := os.Stat(filepath.Join(projPath, "package-lock.json")); err == nil {
		logger.Debug("package-lock.json found, running npm ci", "dir", projPath)
		subCmd = "ci"
	} else {
		logger.Debug("no package-lock.json, running npm install", "dir", projPath)
	}

	return runNpm(projPath, subCmd)
}

// installOtelNodeDeps runs npm install inside the .otel/ directory.
func installOtelNodeDeps(otelDir string) error {
	return runNpm(otelDir, "install")
}

// runBuildScript runs the project's `build` npm script (npm run build).
// Returns an error if no build script is defined in package.json or if the build fails.
func runBuildScript(projPath string) error {
	data, err := os.ReadFile(filepath.Join(projPath, "package.json"))
	if err != nil {
		return fmt.Errorf("read package.json: %w", err)
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("parse package.json: %w", err)
	}
	if pkg.Scripts["build"] == "" {
		return fmt.Errorf("no 'build' script in package.json — add one or run the build manually")
	}
	logger.Debug("running build script", "dir", projPath)
	return runNpm(projPath, "run", "build")
}

func (p *NodeInstrumentationPlan) Execute() error {
	proj := p.Project

	// Ensure project dependencies are installed before any build step.
	// Framework builds (npm run build) require node_modules to be present.
	fmt.Print("  Installing project dependencies... ")
	if err := installNodeProjectDeps(proj.Path); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return fmt.Errorf("installing project dependencies: %w", err)
	}
	fmt.Println("done.")

	// Nuxt requires a pre-built Nitro server. If the build output is missing,
	// automatically run `npm run build` (using the project's build script) so
	// the user doesn't have to run a separate step before re-running dtwiz.
	if p.Framework == "nuxt" {
		nitroEntry := filepath.Join(proj.Path, ".output", "server", "index.mjs")
		if _, err := os.Stat(nitroEntry); os.IsNotExist(err) {
			fmt.Print("  Building Nuxt project (npm run build)... ")
			if err := runBuildScript(proj.Path); err != nil {
				fmt.Println("failed.")
				fmt.Printf("    %v\n", err)
				return fmt.Errorf("building Nuxt project: %w", err)
			}
			fmt.Println("done.")
			// Re-verify the build produced the expected output.
			if _, err := os.Stat(nitroEntry); err != nil {
				fmt.Printf("    Build completed but %s was not produced.\n", nitroEntry)
				fmt.Println("    Check the build output above for errors.")
				return fmt.Errorf("nuxt build did not produce %s", nitroEntry)
			}
		} else if err != nil {
			fmt.Printf("  Cannot access %s: %v\n", nitroEntry, err)
			return fmt.Errorf("accessing Nuxt build output: %w", err)
		}
		logger.Debug("nuxt build output found", "path", nitroEntry)
	}

	// Next.js requires a production build (.next/) for `next start`.
	// If the build output is missing, run `npm run build` automatically.
	if p.Framework == "next" {
		nextBuildDir := filepath.Join(proj.Path, ".next")
		if _, err := os.Stat(nextBuildDir); os.IsNotExist(err) {
			fmt.Print("  Building Next.js project (npm run build)... ")
			if err := runBuildScript(proj.Path); err != nil {
				fmt.Println("failed.")
				fmt.Printf("    %v\n", err)
				return fmt.Errorf("building Next.js project: %w", err)
			}
			fmt.Println("done.")
			// Re-verify the build produced the expected output.
			if _, err := os.Stat(nextBuildDir); err != nil {
				fmt.Printf("    Build completed but .next/ was not produced.\n")
				fmt.Println("    Check the build output above for errors.")
				return fmt.Errorf("next.js build did not produce .next/")
			}
		} else if err != nil {
			fmt.Printf("  Cannot access %s: %v\n", nextBuildDir, err)
			return fmt.Errorf("accessing Next.js build output: %w", err)
		}
		logger.Debug("next.js build output found", "path", nextBuildDir)
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
		return fmt.Errorf("creating .otel/ directory: %w", err)
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
			return fmt.Errorf("writing %s: %w", scriptName, err)
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
			return fmt.Errorf("writing %s: %w", scriptName, err)
		}
		fmt.Println("done.")
	}

	fmt.Print("  Installing OTel packages (npm install)... ")
	if err := installOtelNodeDeps(p.OtelDir); err != nil {
		fmt.Println("failed.")
		fmt.Printf("    %v\n", err)
		return fmt.Errorf("installing OTel packages: %w", err)
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
			// Nitro may spawn a cluster worker that holds the TCP socket while
			// the parent acts as manager — fall back to child-process detection.
			mp.portDetector = detectProcessOrChildListeningPort
			procs = append(procs, mp)
		}
	default:
		for _, ep := range p.Entrypoints {
			svcName := nodeServiceNameFromEntrypoint(proj.Path, ep)
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
		return fmt.Errorf("no services started — all processes failed to start")
	}
	if len(startedServices) < len(procs) {
		return fmt.Errorf("%d of %d service(s) failed to start — check the logs above for errors", len(procs)-len(startedServices), len(procs))
	}

	fmt.Println()
	fmt.Println("  Waiting for traffic — send requests to your services to generate traces and metrics.")
	return nil
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

	collectorEndpoint := fmt.Sprintf("http://127.0.0.1:%d", otlpHTTPPortFromConfig(findExistingCollectorConfig()))
	if serviceName == "" {
		serviceName = "my-service"
	}

	envVars := generateOtelNodeEnvVars(collectorEndpoint, serviceName)

	if dryRun {
		fmt.Println("[dry-run] Would set up OpenTelemetry Node.js auto-instrumentation")
		fmt.Printf("  Collector endpoint: %s\n", collectorEndpoint)
		fmt.Printf("  Service name:       %s\n", serviceName)
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
		info, err := os.Stat(projectPath)
		if err != nil {
			return fmt.Errorf("cannot use project path %q: %w", projectPath, err)
		}
		normalizedProjectPath := filepath.Clean(projectPath)
		if !info.IsDir() {
			if strings.EqualFold(filepath.Base(normalizedProjectPath), "package.json") {
				normalizedProjectPath = filepath.Dir(normalizedProjectPath)
			} else {
				return fmt.Errorf("project path must be a directory or package.json: %s", projectPath)
			}
		}
		plan = buildNodeInstrumentationPlan(ScannedProject{Path: normalizedProjectPath, Markers: []string{"package.json"}}, collectorEndpoint)
		if plan == nil {
			return fmt.Errorf("provided project path is not a valid Node.js project for auto-instrumentation: %s (missing package.json or no recognizable entrypoint/framework detected)", projectPath)
		}
	} else {
		var userInteracted bool
		plan, userInteracted = DetectNodePlan(collectorEndpoint)
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

	ok, err := installer.ConfirmProceed("  Proceed with installation?")
	if err != nil {
		return err
	}
	if !ok {
		return installer.ErrInstallCancelled
	}

	plan.EnvURL = envURL
	plan.PlatformToken = platformToken
	plan.EnvVars = envVars

	fmt.Printf("\n  ── Node.js auto-instrumentation ──\n\n")
	if err := plan.Execute(); err != nil {
		return err
	}
	warnIfCollectorUnreachable(collectorEndpoint)
	return nil
}
