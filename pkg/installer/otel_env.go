package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
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
	if baseName == "" || baseName == "." || baseName == "/" {
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
	Result struct {
		Records []map[string]interface{} `json:"records"`
	} `json:"result"`
}

func waitForServices(envURL, platformToken string, serviceNames []string) {
	if len(serviceNames) == 0 || platformToken == "" {
		return
	}

	appsURL := AppsURL(envURL)
	queryURL := appsURL + "/platform/storage/query/v1/query:execute"
	logger.Debug("waiting for services in Dynatrace", "services", strings.Join(serviceNames, ","), "url", queryURL)

	conditions := make([]string, len(serviceNames))
	for i, name := range serviceNames {
		conditions[i] = fmt.Sprintf("name == \"%s\"", name)
	}
	dql := fmt.Sprintf("smartscapeNodes SERVICE | filter %s", strings.Join(conditions, " or "))

	remainingServices := make(map[string]bool, len(serviceNames))
	for _, name := range serviceNames {
		remainingServices[name] = true
	}

	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Println()
			if len(remainingServices) > 0 {
				pendingNames := make([]string, 0, len(remainingServices))
				for name := range remainingServices {
					pendingNames = append(pendingNames, name)
				}
				fmt.Printf("  Timed out waiting for: %s\n", strings.Join(pendingNames, ", "))
				fmt.Println("  Services may take a few more minutes to appear in Dynatrace.")
			}
			return
		case <-ticker.C:
			logger.Debug("polling smartscape for services", "remaining", len(remainingServices))
			foundServices := fetchSmartscapeServiceNames(queryURL, platformToken, dql)
			for _, name := range foundServices {
				if remainingServices[name] {
					delete(remainingServices, name)
					fmt.Printf("  ✓ \"%s\" appeared in Dynatrace → %s\n", name, appsURL+"/ui/apps/dynatrace.quickstart/")
				}
			}
			if len(remainingServices) == 0 {
				fmt.Println()
				fmt.Println("  All services are reporting to Dynatrace.")
				return
			}
		}
	}
}

func fetchSmartscapeServiceNames(queryURL, platformToken, dql string) []string {
	payload := map[string]interface{}{
		"query":                      dql,
		"requestTimeoutMilliseconds": 10000,
		"maxResultRecords":           100,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(http.MethodPost, queryURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+platformToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("smartscape DQL request failed", "err", err)
		return nil
	}
	defer resp.Body.Close()

	logger.Debug("smartscape DQL response", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	var result dqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	serviceNames := make([]string, 0, len(result.Result.Records))
	for _, record := range result.Result.Records {
		name, ok := record["name"].(string)
		if ok {
			serviceNames = append(serviceNames, name)
		}
	}
	logger.Debug("smartscape DQL found services", "count", len(serviceNames), "names", strings.Join(serviceNames, ","))
	return serviceNames
}
