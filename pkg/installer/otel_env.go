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

const waitForServicesTimeout = 240 * time.Second

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

// waitForServices polls Dynatrace Smartscape until every service in serviceNames
// appears. When containsMatch is true, a service is considered found when its
// Dynatrace name contains the input name as a substring (useful for Lambda
// functions whose names are suffixed with the region, e.g. "fn in us-east-1").
// When containsMatch is false, an exact name match is required.
func waitForServices(envURL, platformToken string, serviceNames []string, containsMatch bool) {
	if len(serviceNames) == 0 || platformToken == "" {
		return
	}

	appsURL := AppsURL(envURL)
	queryURL := appsURL + "/platform/storage/query/v1/query:execute"
	logger.Debug("waiting for services in Dynatrace", "services", strings.Join(serviceNames, ","), "url", queryURL)

	conditions := make([]string, len(serviceNames))
	for i, name := range serviceNames {
		if containsMatch {
			conditions[i] = fmt.Sprintf("contains(name, \"%s\")", name)
		} else {
			conditions[i] = fmt.Sprintf("name == \"%s\"", name)
		}
	}
	dql := fmt.Sprintf("smartscapeNodes SERVICE, from:now() - 1m | filter %s", strings.Join(conditions, " or "))

	remainingServices := make(map[string]bool, len(serviceNames))
	for _, name := range serviceNames {
		remainingServices[name] = true
	}

	timeout := time.After(waitForServicesTimeout)
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
			for _, fullName := range foundServices {
				for inputName := range remainingServices {
					matched := (!containsMatch && fullName == inputName) ||
						(containsMatch && strings.Contains(fullName, inputName))
					if matched {
						delete(remainingServices, inputName)
						fmt.Printf("  ✓ \"%s\" appeared in Dynatrace → %s\n", fullName, appsURL+"/ui/apps/my.getting.started.dieter/")
						break
					}
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

	if result.State != "SUCCEEDED" {
		logger.Debug("smartscape DQL query not yet complete", "state", result.State)
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
