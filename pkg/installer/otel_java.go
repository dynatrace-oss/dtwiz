package installer

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const otelJavaAgentURL = "https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar"

var javaProjectMarkers = []string{
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
	"gradlew",
	".mvn",
}

func detectJavaProjects() []ScannedProject {
	return scanProjectDirs(javaProjectMarkers, nil)
}

func detectJavaProcesses() []DetectedProcess {
	return detectProcesses("java", nil)
}

func detectJava() (string, error) {
	path, err := exec.LookPath("java")
	if err != nil {
		logger.Debug("java not found on PATH")
		return "", fmt.Errorf("Java not found — install a JDK/JRE and ensure it is in PATH")  //nolint:ST1005
	}
	out, err := exec.Command(path, "-version").CombinedOutput()
	if err != nil {
		logger.Warn("java version check failed", "path", path, "err", err)
		return "", fmt.Errorf("unable to determine Java version: %w", err)
	}
	versionLine := strings.Fields(strings.TrimSpace(string(out)))[0]
	logger.Debug("java found", "path", path, "version", versionLine)
	fmt.Printf("  Java: %s (%s)\n", path, versionLine)
	return path, nil
}

type JavaInstrumentationPlan struct {
	Project ScannedProject
	EnvVars map[string]string
}

func (p *JavaInstrumentationPlan) Runtime() string { return "Java" }

func DetectJavaPlan(apiURL, token string) *JavaInstrumentationPlan {
	if _, err := exec.LookPath("java"); err != nil {
		logger.Debug("java not found on PATH", "skipping Java instrumentation")
		return nil
	}

	projects, processes := runInParallel(detectJavaProjects, detectJavaProcesses)
	matchProcessesToProjects(projects, processes)

	if len(projects) == 0 {
		logger.Debug("no Java projects detected", "skipping Java instrumentation")
		return nil
	}

	sel := promptProjectSelection("Java", projects)
	if sel == nil {
		return nil
	}
	proj := *sel
	svcName := projectServiceName(proj.Path)
	envVars := generateBaseOtelEnvVars(apiURL, token, svcName)

	return &JavaInstrumentationPlan{
		Project: proj,
		EnvVars: envVars,
	}
}

func (p *JavaInstrumentationPlan) PrintPlanSteps() {
	fmt.Printf("     Project:    %s\n", p.Project.Path)
	fmt.Printf("     Agent JAR:  %s\n", otelJavaAgentURL)
	fmt.Println("     java -javaagent:opentelemetry-javaagent.jar -jar your_app.jar")
}

func (p *JavaInstrumentationPlan) Execute() {
	fmt.Println()
	fmt.Printf("  Download the OpenTelemetry Java agent:\n")
	fmt.Printf("    %s\n", otelJavaAgentURL)
	fmt.Println()
	fmt.Println("  Set the following environment variables:")
	fmt.Println()
	for _, line := range formatEnvExportLines(p.EnvVars) {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
	fmt.Println("  Start your Java application with:")
	fmt.Println("    java -javaagent:opentelemetry-javaagent.jar -jar your_app.jar")
}

func InstallOtelJava(envURL, token, serviceName string, dryRun bool) error {
	apiURL := APIURL(envURL)

	if serviceName == "" {
		serviceName = "my-service"
	}

	envVars := generateBaseOtelEnvVars(apiURL, token, serviceName)

	if dryRun {
		fmt.Println("[dry-run] Would set up OpenTelemetry Java auto-instrumentation")
		fmt.Printf("  API URL:      %s\n", apiURL)
		fmt.Printf("  Service name: %s\n", serviceName)
		fmt.Printf("  Agent JAR:    %s\n", otelJavaAgentURL)
		fmt.Println()
		fmt.Println("  Environment variables to set:")
		for _, line := range formatPrintableEnvVars(envVars) {
			fmt.Printf("    %s\n", line)
		}
		fmt.Printf("  JVM flag:     -javaagent:opentelemetry-javaagent.jar\n")
		return nil
	}

	if _, err := detectJava(); err != nil {
		return err
	}

	fmt.Printf("  Agent JAR URL: %s\n", otelJavaAgentURL)
	fmt.Println("  (automatic download coming soon — download manually for now)")

	fmt.Println()
	fmt.Println("  Add the following to your environment:")
	fmt.Println()
	fmt.Println(GenerateEnvExportScript(envVars))
	fmt.Println("  Start your Java application with:")
	fmt.Println("    java -javaagent:opentelemetry-javaagent.jar -jar your_app.jar")

	return nil
}
