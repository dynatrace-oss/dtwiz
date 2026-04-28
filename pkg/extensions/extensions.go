package extensions

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

const (
	// extensionPath is the Platform API path for a single extension version.
	extensionPath = "/platform/extensions/v2/extensions/%s"
	// monitoringConfigsPath is the Platform API path for Extensions v2 monitoring configurations.
	monitoringConfigsPath = extensionPath + "/monitoring-configurations"
)

// InstallExtension activates a specific version of a Dynatrace extension.
// When silent is true, an HTTP 400 and HTTP 409 response is treated as a no-op rather than an error
// (useful when the extension may already be installed).
func InstallExtension(c *client.PlatformClient, extensionName, version string, silent bool) error {
	path := fmt.Sprintf(extensionPath, extensionName)
	payload := map[string]string{
		"extensionName": extensionName,
		"version":       version,
	}
	resp, err := c.HTTP().R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(path)
	if err != nil {
		return fmt.Errorf("installing extension %s@%s: %w", extensionName, version, err)
	}
	sc := resp.StatusCode()
	if silent && (sc == http.StatusBadRequest || sc == http.StatusConflict) {
		return nil
	}
	if sc != http.StatusOK && sc != http.StatusCreated {
		body := resp.String()
		return fmt.Errorf("installing extension %s@%s (HTTP %d): %s", extensionName, version, sc, body[:min(len(body), 400)])
	}
	return nil
}

// MonitoringConfigItem is a single entry returned by the monitoring configurations list endpoint.
type MonitoringConfigItem struct {
	ObjectID string          `json:"objectId"`
	Value    json.RawMessage `json:"value"`
}

type monitoringConfigPage struct {
	Items       []MonitoringConfigItem `json:"items"`
	NextPageKey string                 `json:"nextPageKey"`
}

type createResult struct {
	Code     int    `json:"code"`
	ObjectID string `json:"objectId"`
}

// ListMonitoringConfigs returns all monitoring configuration items for the given
// extension, following pagination automatically.
func ListMonitoringConfigs(c *client.PlatformClient, extensionName string) ([]MonitoringConfigItem, error) {
	basePath := fmt.Sprintf(monitoringConfigsPath, extensionName)
	var all []MonitoringConfigItem
	nextPageKey := ""
	for {
		var page monitoringConfigPage
		req := c.HTTP().R().SetResult(&page)
		if nextPageKey != "" {
			req = req.SetQueryParam("nextPageKey", nextPageKey)
		}
		resp, err := req.Get(basePath)
		if err != nil {
			return nil, fmt.Errorf("listing monitoring configs for %s: %w", extensionName, err)
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("listing monitoring configs for %s: HTTP %d", extensionName, resp.StatusCode())
		}
		all = append(all, page.Items...)
		nextPageKey = page.NextPageKey
		if nextPageKey == "" {
			break
		}
	}
	return all, nil
}

// CreateMonitoringConfigs posts a bulk monitoring configuration request for the
// given extension and returns the objectId of the first successfully created entry.
func CreateMonitoringConfigs(c *client.PlatformClient, extensionName string, payload []map[string]interface{}) (string, error) {
	path := fmt.Sprintf(monitoringConfigsPath, extensionName)
	var results []createResult
	resp, err := c.HTTP().R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		SetResult(&results).
		Post(path)
	if err != nil {
		return "", fmt.Errorf("creating monitoring config for %s: %w", extensionName, err)
	}
	sc := resp.StatusCode()
	if sc != http.StatusOK && sc != http.StatusCreated && sc != http.StatusMultiStatus {
		body := resp.String()
		return "", fmt.Errorf("creating monitoring config for %s (HTTP %d): %s", extensionName, sc, body[:min(len(body), 400)])
	}
	for _, r := range results {
		if (r.Code == http.StatusOK || r.Code == http.StatusCreated) && r.ObjectID != "" {
			return r.ObjectID, nil
		}
	}
	noIDBody := resp.String()
	return "", fmt.Errorf("monitoring config creation for %s returned no objectId: %s", extensionName, noIDBody[:min(len(noIDBody), 400)])
}

// DeleteMonitoringConfig deletes a single monitoring configuration by objectId.
func DeleteMonitoringConfig(c *client.PlatformClient, extensionName, objectID string) error {
	path := fmt.Sprintf(monitoringConfigsPath+"/%s", extensionName, objectID)
	resp, err := c.HTTP().R().Delete(path)
	if err != nil {
		return fmt.Errorf("deleting monitoring config %s: %w", objectID, err)
	}
	sc := resp.StatusCode()
	if sc != http.StatusOK && sc != http.StatusNoContent {
		body := resp.String()
		return fmt.Errorf("deleting monitoring config %s (HTTP %d): %s", objectID, sc, body[:min(len(body), 400)])
	}
	return nil
}
