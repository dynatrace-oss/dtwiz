package installer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const otelExporterOTLPHeadersEnvVar = "OTEL_EXPORTER_OTLP_HEADERS"

func generateBaseOtelEnvVars(apiURL, token, serviceName string) map[string]string {
	return map[string]string{
		"OTEL_SERVICE_NAME":                                 serviceName,
		"OTEL_EXPORTER_OTLP_ENDPOINT":                       strings.TrimRight(apiURL, "/") + "/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_HEADERS":                        "Authorization=Api-Token%20" + token,
		"OTEL_EXPORTER_OTLP_PROTOCOL":                       "http/protobuf",
		"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE": "delta",
		"OTEL_TRACES_EXPORTER":                              "otlp",
		"OTEL_METRICS_EXPORTER":                             "otlp",
		"OTEL_LOGS_EXPORTER":                                "otlp",
	}
}

func projectServiceName(projectPath string) string {
	baseName := filepath.Base(projectPath)
	// filepath.Base("/") returns "/" on Unix and "\\" on Windows; treat either as root.
	if baseName == "" || baseName == "." || strings.Trim(baseName, `/\`) == "" {
		return "my-service"
	}
	return baseName
}

func GenerateEnvExportScript(envVars map[string]string) string {
	lines := formatEnvExportLines(envVars)
	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n") + "\n"
}

func formatEnvExportLines(envVars map[string]string) []string {
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("export %s=%q", key, printableEnvVarValue(key, envVars[key])))
	}

	return lines
}

func formatEnvVars(envVars map[string]string) []string {
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	formatted := make([]string, 0, len(keys))
	for _, key := range keys {
		formatted = append(formatted, key+"="+envVars[key])
	}
	return formatted
}

func formatPrintableEnvVars(envVars map[string]string) []string {
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	formatted := make([]string, 0, len(keys))
	for _, key := range keys {
		formatted = append(formatted, key+"="+printableEnvVarValue(key, envVars[key]))
	}
	return formatted
}

func printableEnvVarValue(key, value string) string {
	if key == otelExporterOTLPHeadersEnvVar {
		return "Authorization=Api-Token%20<redacted>"
	}

	return value
}

type dqlResponse struct {
	State  string `json:"state"`
	Result struct {
		Records []map[string]interface{} `json:"records"`
	} `json:"result"`
}
