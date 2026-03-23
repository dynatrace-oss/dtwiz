package installer

import (
	"fmt"
	"os/exec"
	"strings"
)

// otelJavaAgentURL is the download URL for the latest OpenTelemetry Java agent JAR.
const otelJavaAgentURL = "https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar"

// detectJava finds a usable Java runtime on the current PATH.
func detectJava() (string, error) {
	path, err := exec.LookPath("java")
	if err != nil {
		return "", fmt.Errorf("Java not found — install a JDK/JRE and ensure it is in PATH")
	}
	out, err := exec.Command(path, "-version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to determine Java version: %w", err)
	}
	fmt.Printf("  Java: %s (%s)\n", path, strings.Fields(strings.TrimSpace(string(out)))[0])
	return path, nil
}

// generateOtelJavaEnvVars returns the OTEL_* environment variables and JVM
// flags required for Java auto-instrumentation exporting to Dynatrace.
func generateOtelJavaEnvVars(apiURL, token, serviceName string) map[string]string {
	return map[string]string{
		"OTEL_SERVICE_NAME":            serviceName,
		"OTEL_EXPORTER_OTLP_ENDPOINT": strings.TrimRight(apiURL, "/") + "/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_HEADERS":  "Authorization=Api-Token " + token,
		"OTEL_TRACES_EXPORTER":        "otlp",
		"OTEL_METRICS_EXPORTER":       "otlp",
		"OTEL_LOGS_EXPORTER":          "otlp",
	}
}

// InstallOtelJava sets up OpenTelemetry auto-instrumentation for Java
// applications. Downloads the Java agent JAR and prints the required JVM flags
// and environment variables.
//
// TODO: Download the agent JAR, detect running Java processes, offer restart.
func InstallOtelJava(envURL, token, serviceName string, dryRun bool) error {
	apiURL := APIURL(envURL)

	if serviceName == "" {
		serviceName = "my-service"
	}

	envVars := generateOtelJavaEnvVars(apiURL, token, serviceName)

	if dryRun {
		fmt.Println("[dry-run] Would set up OpenTelemetry Java auto-instrumentation")
		fmt.Printf("  API URL:      %s\n", apiURL)
		fmt.Printf("  Service name: %s\n", serviceName)
		fmt.Printf("  Agent JAR:    %s\n", otelJavaAgentURL)
		fmt.Println()
		fmt.Println("  Environment variables to set:")
		for k, v := range envVars {
			fmt.Printf("    %s=%s\n", k, v)
		}
		fmt.Printf("  JVM flag:     -javaagent:opentelemetry-javaagent.jar\n")
		return nil
	}

	// 1. Detect Java.
	if _, err := detectJava(); err != nil {
		return err
	}

	// 2. TODO: Download the OpenTelemetry Java agent JAR.
	fmt.Printf("  Agent JAR URL: %s\n", otelJavaAgentURL)
	fmt.Println("  (automatic download coming soon — download manually for now)")

	// 3. Print env var and JVM flag instructions.
	fmt.Println()
	fmt.Println("  Add the following to your environment:")
	fmt.Println()
	fmt.Println(GenerateEnvExportScript(envVars))
	fmt.Println("  Start your Java application with:")
	fmt.Println("    java -javaagent:opentelemetry-javaagent.jar -jar your_app.jar")

	return nil
}
