package installer

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type SubModule struct {
	Name string
	Path string
}

type MultiModuleProject struct {
	BuildTool    string
	Modules      []SubModule
	BuildCommand string
}

type mavenPOM struct {
	Modules []string `xml:"modules>module"`
}

// gradleIncludeLineRe matches an include directive and captures everything after
// the keyword up to the end of the statement (closing paren or end-of-line).
// This handles all common forms:
//
//	include("api", "web")          - Kotlin DSL, parenthesised
//	include ':api', ':web'          - Groovy DSL, no parens
//	include(":api:sub", ":other")   - colon-prefixed nested paths
var gradleIncludeLineRe = regexp.MustCompile(`(?m)include\s*\(?([^)\n]+)`)

// gradleQuotedArgRe extracts individual quoted values from an include argument list.
var gradleQuotedArgRe = regexp.MustCompile(`['"]([^'"]+)['"]`)

func parseMavenModules(projectPath string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, "pom.xml"))
	if err != nil {
		return nil, err
	}
	var pom mavenPOM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, fmt.Errorf("parsing pom.xml: %w", err)
	}
	return pom.Modules, nil
}

func isMavenMultiModule(projectPath string) bool {
	modules, err := parseMavenModules(projectPath)
	return err == nil && len(modules) > 0
}

// converts Gradle colon notation to path separators (e.g. ":ui:web" → "ui/web").
func parseGradleSubprojects(projectPath string) ([]string, error) {
	var data []byte
	var readErr error
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		data, readErr = os.ReadFile(filepath.Join(projectPath, name))
		if readErr == nil {
			break
		}
	}
	if readErr != nil {
		return nil, readErr
	}
	var result []string
	for _, lineMatch := range gradleIncludeLineRe.FindAllSubmatch(data, -1) {
		for _, argMatch := range gradleQuotedArgRe.FindAllSubmatch(lineMatch[1], -1) {
			path := strings.TrimPrefix(string(argMatch[1]), ":")
			path = strings.ReplaceAll(path, ":", "/")
			result = append(result, path)
		}
	}
	return result, nil
}

func isGradleMultiProject(projectPath string) bool {
	subs, err := parseGradleSubprojects(projectPath)
	return err == nil && len(subs) > 0
}

func mavenBuildCommand(projectPath string) string {
	mvnCmd, _ := resolveMavenCmd(projectPath)
	if mvnCmd == "" {
		return ""
	}
	return mvnCmd + " clean package -DskipTests"
}

func gradleBuildCommand(projectPath string) string {
	gradleCmd, _ := resolveGradleCmd(projectPath)
	if gradleCmd == "" {
		return ""
	}
	return gradleCmd + " build -x test"
}

// Maven is checked before Gradle — pom.xml takes precedence when both exist.
func detectMultiModule(projectPath string) *MultiModuleProject {
	modules, err := parseMavenModules(projectPath)
	if err == nil && len(modules) > 0 {
		subs := make([]SubModule, len(modules))
		for i, mod := range modules {
			subs[i] = SubModule{
				Name: mod,
				Path: filepath.Join(projectPath, mod),
			}
		}
		logger.Debug("detected maven multi-module project", "modules", len(subs))
		return &MultiModuleProject{
			BuildTool:    "maven",
			Modules:      subs,
			BuildCommand: mavenBuildCommand(projectPath),
		}
	}

	if !isGradleMultiProject(projectPath) {
		return nil
	}
	subprojects, err := parseGradleSubprojects(projectPath)
	if err == nil && len(subprojects) > 0 {
		subs := make([]SubModule, len(subprojects))
		for i, sub := range subprojects {
			subs[i] = SubModule{
				Name: filepath.Base(sub),
				Path: filepath.Join(projectPath, filepath.FromSlash(sub)),
			}
		}
		logger.Debug("detected gradle multi-module project", "modules", len(subs))
		return &MultiModuleProject{
			BuildTool:    "gradle",
			Modules:      subs,
			BuildCommand: gradleBuildCommand(projectPath),
		}
	}

	return nil
}

// buildMultiModulePlan constructs a JavaInstrumentationPlan for a multi-module project.
func buildMultiModulePlan(mm *MultiModuleProject, proj ScannedProject, apiURL, token, envURL string) *JavaInstrumentationPlan {
	logger.Debug("building multi-module plan", "tool", mm.BuildTool, "modules", len(mm.Modules), "build_cmd", mm.BuildCommand)
	agentPath, err := javaAgentPath()
	if err != nil {
		agentPath = "opentelemetry-javaagent.jar"
	}

	subs := make([]SubModulePlan, len(mm.Modules))
	for i, mod := range mm.Modules {
		svcName := normalizeServiceName(mod.Name)
		envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

		var launchCmd string
		entrypoints := detectJavaEntrypoints(mod.Path)
		if len(entrypoints) > 0 {
			launchCmd = displayInstrumentedCmd(entrypoints[0], agentPath)
		} else {
			launchCmd = "(will be resolved after build)"
		}

		subs[i] = SubModulePlan{
			Name:          svcName,
			Path:          mod.Path,
			LaunchCommand: launchCmd,
			EnvVars:       envVars,
		}
	}

	return &JavaInstrumentationPlan{
		Project:      proj,
		EnvURL:       envURL,
		Token:        token,
		BuildCommand: mm.BuildCommand,
		SubModules:   subs,
	}
}

// executeMultiModule runs the multi-module build (when BuildCommand is set), launches each sub-module,
// prints a process summary, and updates the OTel Collector config if present.
// It returns an error if any critical step fails (build, agent download, no services started/running).
func (p *JavaInstrumentationPlan) executeMultiModule() error {
	if p.BuildCommand != "" {
		display.PrintStatusLine("build", "Running "+p.BuildCommand+"...", display.ColorOK)
		fields := strings.Fields(p.BuildCommand)
		var cmd *exec.Cmd
		if len(fields) > 0 && (strings.HasSuffix(fields[0], ".cmd") || strings.HasSuffix(fields[0], ".bat")) {
			cmd = exec.Command("cmd", append([]string{"/c", fields[0]}, fields[1:]...)...)
		} else {
			cmd = exec.Command(fields[0], fields[1:]...)
		}
		cmd.Dir = p.Project.Path
		if logger.IsDebug() {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			display.PrintStatusLine("error", "build failed: "+err.Error(), display.ColorError)
			return fmt.Errorf("build failed: %w", err)
		}
	}

	agentPath, err := downloadJavaAgent()
	if err != nil {
		display.PrintStatusLine("error", fmt.Sprintf("failed to download agent: %v", err), display.ColorError)
		return fmt.Errorf("failed to download agent: %w", err)
	}

	var procs []*ManagedProcess
	for i, sub := range p.SubModules {
		entrypoints := detectJavaEntrypoints(sub.Path)
		if len(entrypoints) == 0 {
			display.PrintStatusLine("skip", sub.Name+": no runnable entrypoint found", display.ColorMuted)
			continue
		}
		ep := &entrypoints[0]

		svcName := sub.Name
		logPath := filepath.Join(sub.Path, svcName+".log")
		logFile, err := os.Create(logPath)
		if err != nil {
			display.PrintStatusLine("error", fmt.Sprintf("failed to create log file for %s: %v", svcName, err), display.ColorError)
			continue
		}

		envVars := p.SubModules[i].EnvVars
		logger.Debug("launching instrumented java process", "cmd", ep.Command, "dir", sub.Path)
		cmd := buildInstrumentedCmd(*ep, agentPath, sub.Path, envVars)

		proc, err := StartManagedProcess(svcName, svcName+".log", "", cmd, logFile)
		if err != nil {
			display.PrintStatusLine("error", fmt.Sprintf("failed to start %s: %v", svcName, err), display.ColorError)
			continue
		}
		proc.portDetector = func(pid int) string { return detectJavaListeningPort(pid, sub.Path) }
		procs = append(procs, proc)
	}

	if len(procs) == 0 {
		display.PrintStatusLine("error", "No services started.", display.ColorError)
		return fmt.Errorf("no services started")
	}

	aliveNames, _ := PrintProcessSummary(procs, processSettleDelay)
	if len(aliveNames) == 0 {
		display.PrintStatusLine("error", "No services are running — check the logs above for errors.", display.ColorError)
		return fmt.Errorf("no services are running")
	}

	updateOtelCollectorIfPresent(p.EnvURL, p.Token, false)
	return nil
}
