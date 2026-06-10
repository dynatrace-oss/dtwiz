package extensions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

const (
	// extensionPath is the Platform API path for a single extension version.
	extensionPath = "/platform/extensions/v2/extensions/%s"
	// monitoringConfigsPath is the Platform API path for Extensions v2 monitoring configurations.
	monitoringConfigsPath = extensionPath + "/monitoring-configurations"
	// monitoringConfigPath is the Platform API path for a single Extensions v2 monitoring configuration.
	monitoringConfigPath = monitoringConfigsPath + "/%s"
)

// InstallExtension activates a specific version of a Dynatrace extension.
// When silent is true, an HTTP 400 and HTTP 409 response is treated as a no-op rather than an error
// (useful when the extension may already be installed).
// Required scope: extensions:definitions:write
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
	if sc != http.StatusOK && sc != http.StatusCreated && sc != http.StatusAccepted {
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

// CreateMonitoringConfig posts a single monitoring configuration for the given
// extension and returns the objectId of the created entry.
func CreateMonitoringConfig(c *client.PlatformClient, extensionName string, payload map[string]interface{}) (string, error) {
	path := fmt.Sprintf(monitoringConfigsPath, extensionName)
	var result struct {
		ObjectID string `json:"objectId"`
	}
	resp, err := c.HTTP().R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		SetResult(&result).
		Post(path)
	if err != nil {
		return "", fmt.Errorf("creating monitoring config for %s: %w", extensionName, err)
	}
	sc := resp.StatusCode()
	if sc != http.StatusOK && sc != http.StatusCreated {
		return "", fmt.Errorf("creating monitoring config for %s (HTTP %d)", extensionName, sc)
	}
	if result.ObjectID == "" {
		return "", fmt.Errorf("monitoring config creation for %s returned no objectId", extensionName)
	}
	return result.ObjectID, nil
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
		return fmt.Errorf("deleting monitoring config %s (HTTP %d)", objectID, sc)
	}
	return nil
}

// ExtensionVersionItem is a single entry returned by the extension versions list endpoint.
type ExtensionVersionItem struct {
	Version string `json:"version"`
}

type extensionVersionsResponse struct {
	Items []ExtensionVersionItem `json:"items"`
}

// ListInstalledVersions returns all installed versions of the given extension.
func ListInstalledVersions(c *client.PlatformClient, extensionName string) ([]ExtensionVersionItem, error) {
	path := fmt.Sprintf(extensionPath, extensionName)
	var result extensionVersionsResponse
	resp, err := c.HTTP().R().SetResult(&result).Get(path)
	if err != nil {
		return nil, fmt.Errorf("listing versions for %s: %w", extensionName, err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("listing versions for %s: HTTP %d", extensionName, resp.StatusCode())
	}
	return result.Items, nil
}

// GetLatestInstalledVersion returns the highest dotted-numeric version of the
// given extension currently installed in the tenant. Non-numeric segments are
// treated as 0 when comparing.
func GetLatestInstalledVersion(c *client.PlatformClient, extensionName string) (string, error) {
	items, err := ListInstalledVersions(c, extensionName)
	if err != nil {
		return "", err
	}
	versions := make([]string, 0, len(items))
	for _, it := range items {
		if it.Version != "" {
			versions = append(versions, it.Version)
		}
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no installed versions found for %s", extensionName)
	}
	sort.Slice(versions, func(i, j int) bool { return compareDottedVersions(versions[i], versions[j]) > 0 })
	return versions[0], nil
}

// MonitoringConfig is a single monitoring configuration with its scope and
// mutable value payload, as returned by the get-by-id endpoint.
type MonitoringConfig struct {
	Scope string                 `json:"scope"`
	Value map[string]interface{} `json:"value"`
}

// GetMonitoringConfig fetches a single monitoring configuration by objectId.
func GetMonitoringConfig(c *client.PlatformClient, extensionName, objectID string) (*MonitoringConfig, error) {
	path := fmt.Sprintf(monitoringConfigPath, extensionName, objectID)
	var result MonitoringConfig
	resp, err := c.HTTP().R().SetResult(&result).Get(path)
	if err != nil {
		return nil, fmt.Errorf("getting monitoring config %s: %w", objectID, err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("getting monitoring config %s (HTTP %d): %s", objectID, resp.StatusCode(), truncateBody(resp.String()))
	}
	if result.Value == nil {
		return nil, fmt.Errorf("getting monitoring config %s: empty value", objectID)
	}
	return &result, nil
}

// UpdateMonitoringConfig replaces an existing monitoring configuration in place
// via PUT. The cfg.Scope and cfg.Value are sent as the request body.
func UpdateMonitoringConfig(c *client.PlatformClient, extensionName, objectID string, cfg *MonitoringConfig) error {
	path := fmt.Sprintf(monitoringConfigPath, extensionName, objectID)
	payload := map[string]interface{}{
		"scope": cfg.Scope,
		"value": cfg.Value,
	}
	resp, err := c.HTTP().R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Put(path)
	if err != nil {
		return fmt.Errorf("updating monitoring config %s: %w", objectID, err)
	}
	sc := resp.StatusCode()
	if sc != http.StatusOK && sc != http.StatusCreated && sc != http.StatusNoContent {
		return fmt.Errorf("updating monitoring config %s (HTTP %d): %s", objectID, sc, truncateBody(resp.String()))
	}
	return nil
}

// compareDottedVersions compares two dotted-numeric version strings (e.g.
// "1.2.10" vs "1.10.0"). Returns 1 if a > b, -1 if a < b, 0 if equal. Missing
// or non-numeric segments are treated as 0.
func compareDottedVersions(a, b string) int {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bv, _ = strconv.Atoi(bp[i])
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

func truncateBody(s string) string {
	if len(s) > 400 {
		return s[:400]
	}
	return s
}
