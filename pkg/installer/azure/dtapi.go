package azure

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	settingsAPI        = "/platform/classic/environment-api/v2/settings/objects"
	connectionSchemaID = "builtin:hyperscaler-authentication.connections.azure"
	extensionName      = "com.dynatrace.extension.da-azure"
	extensionAPI       = "/platform/extensions/v2/extensions/" + extensionName
	monitoringAPI      = extensionAPI + "/monitoring-configurations"

	// Settings-schema enum keys exposing the valid values for the monitoring
	// configuration. These match the keys dtctl reads from the same schema.
	azureLocationEnumKey   = "dynatrace.datasource.azure:location"
	azureFeatureSetEnumKey = "FeatureSetsType"
)

// dtclient performs the Dynatrace Platform API calls needed for the Azure integration.
type dtclient interface {
	createConnection(name string) (objectID string, err error)
	updateConnection(objectID, name, tenantID, clientID string) error
	createMonitoring(configName, connectionObjectID, clientID, subscriptionID string) error
	// uninstall
	findConnection(name string) (objectID, clientID string, err error)
	deleteConnection(objectID string) error
	findMonitoringConfig(name string) (configID string, err error)
	deleteMonitoring(configID string) error
}

// ─── SDK implementation ───────────────────────────────────────────────────────

type sdkDTClient struct {
	c *httpclient.Client
}

func newSDKDTClient(envURL, platformToken string) (*sdkDTClient, error) {
	appsURL := installer.AppsURL(envURL)
	c, err := httpclient.New(appsURL, httpclient.WithToken(platformToken))
	if err != nil {
		return nil, fmt.Errorf("creating Dynatrace API client: %w", err)
	}
	return &sdkDTClient{c: c}, nil
}

// ─── connection types ─────────────────────────────────────────────────────────

type connFedCred struct {
	DirectoryID   string   `json:"directoryId,omitempty"`
	ApplicationID string   `json:"applicationId,omitempty"`
	Consumers     []string `json:"consumers"`
}

type connValue struct {
	Name                        string       `json:"name"`
	Type                        string       `json:"type"`
	FederatedIdentityCredential *connFedCred `json:"federatedIdentityCredential,omitempty"`
}

// ─── createConnection ─────────────────────────────────────────────────────────

func (d *sdkDTClient) createConnection(name string) (string, error) {
	type createBody struct {
		SchemaID string    `json:"schemaId"`
		Scope    string    `json:"scope"`
		Value    connValue `json:"value"`
	}
	type createResp struct {
		ObjectID string `json:"objectId"`
		Error    *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	body := []createBody{{
		SchemaID: connectionSchemaID,
		Scope:    "environment",
		Value: connValue{
			Name: name,
			Type: "federatedIdentityCredential",
			FederatedIdentityCredential: &connFedCred{
				Consumers: []string{"SVC:com.dynatrace.da"},
			},
		},
	}}

	resp, err := d.c.HTTP().R().SetBody(body).Post(settingsAPI)
	if err != nil {
		return "", fmt.Errorf("create connection: %w", err)
	}
	if resp.IsError() {
		return "", fmt.Errorf("create connection: status %d: %s", resp.StatusCode(), resp.String())
	}

	var result []createResp
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("create connection: parse response: %w", err)
	}
	if len(result) == 0 {
		return "", fmt.Errorf("create connection: empty response")
	}
	if result[0].Error != nil {
		return "", fmt.Errorf("create connection: %s", result[0].Error.Message)
	}
	logger.Debug("connection created", "objectId", result[0].ObjectID)
	return result[0].ObjectID, nil
}

// ─── updateConnection ─────────────────────────────────────────────────────────

func (d *sdkDTClient) updateConnection(objectID, name, tenantID, clientID string) error {
	// Fetch current object to get schemaVersion for If-Match optimistic lock.
	type getResp struct {
		SchemaVersion string    `json:"schemaVersion"`
		Value         connValue `json:"value"`
	}

	var current getResp
	getResp_, err := d.c.HTTP().R().SetResult(&current).Get(fmt.Sprintf("%s/%s", settingsAPI, objectID))
	if err != nil {
		return fmt.Errorf("update connection: get current: %w", err)
	}
	if getResp_.IsError() {
		return fmt.Errorf("update connection: get current: status %d: %s", getResp_.StatusCode(), getResp_.String())
	}

	body := map[string]interface{}{
		"value": connValue{
			Name: name,
			Type: "federatedIdentityCredential",
			FederatedIdentityCredential: &connFedCred{
				DirectoryID:   tenantID,
				ApplicationID: clientID,
				Consumers:     []string{"SVC:com.dynatrace.da"},
			},
		},
	}
	logger.Debug("updating connection", "objectId", objectID, "tenantID", tenantID, "clientID", clientID)

	resp, err := d.c.HTTP().R().
		SetBody(body).
		SetHeader("If-Match", current.SchemaVersion).
		Put(fmt.Sprintf("%s/%s", settingsAPI, objectID))
	if err != nil {
		return fmt.Errorf("update connection: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("update connection: status %d: %s", resp.StatusCode(), resp.String())
	}
	return nil
}

// ─── createMonitoring ─────────────────────────────────────────────────────────

func (d *sdkDTClient) createMonitoring(configName, connectionObjectID, clientID, subscriptionID string) error {
	version, err := d.latestExtensionVersion()
	if err != nil {
		return fmt.Errorf("create monitoring: %w", err)
	}
	logger.Debug("using extension version", "version", version)

	type credential struct {
		Enabled            bool   `json:"enabled"`
		Description        string `json:"description"`
		ConnectionID       string `json:"connectionId"`
		ServicePrincipalID string `json:"servicePrincipalId"`
		Type               string `json:"type"`
	}
	type azureBlock struct {
		SubscriptionFilteringMode string       `json:"subscriptionFilteringMode"`
		SubscriptionFiltering     []string     `json:"subscriptionFiltering"`
		LocationFiltering         []string     `json:"locationFiltering"`
		Credentials               []credential `json:"credentials"`
	}
	type monBody struct {
		Enabled     bool       `json:"enabled"`
		Description string     `json:"description"`
		Version     string     `json:"version"`
		FeatureSets []string   `json:"featureSets"`
		Azure       azureBlock `json:"azure"`
	}

	// Mirror dtctl's defaults: monitor all locations and all "_essential" feature
	// sets, both derived from the extension's settings schema enums.
	schema, err := d.fetchExtensionSchema(version)
	if err != nil {
		return fmt.Errorf("create monitoring: %w", err)
	}
	locations := schema.enumValues(azureLocationEnumKey)
	if len(locations) == 0 {
		return fmt.Errorf("create monitoring: no locations found under enum %q in extension schema", azureLocationEnumKey)
	}
	featureSets := make([]string, 0)
	for _, fs := range schema.enumValues(azureFeatureSetEnumKey) {
		if strings.HasSuffix(fs, "_essential") {
			featureSets = append(featureSets, fs)
		}
	}
	if len(featureSets) == 0 {
		return fmt.Errorf("create monitoring: no \"_essential\" feature sets found under enum %q in extension schema", azureFeatureSetEnumKey)
	}
	logger.Debug("monitoring defaults from schema", "locations", len(locations), "featureSets", len(featureSets))

	v := monBody{
		Enabled:     true,
		Description: configName,
		Version:     version,
		FeatureSets: featureSets,
		Azure: azureBlock{
			SubscriptionFilteringMode: "INCLUDE",
			SubscriptionFiltering:     []string{subscriptionID},
			LocationFiltering:         locations,
			Credentials: []credential{{
				Enabled:            true,
				Description:        configName,
				ConnectionID:       connectionObjectID,
				ServicePrincipalID: clientID,
				Type:               "FEDERATED",
			}},
		},
	}

	envelope := map[string]interface{}{
		"scope": "integration-azure",
		"value": v,
	}
	bodyBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("create monitoring: marshal: %w", err)
	}
	logger.Debug("create monitoring request", "url", monitoringAPI, "body", string(bodyBytes))

	resp, err := d.c.HTTP().R().
		SetHeader("Content-Type", "application/json").
		SetBody(bodyBytes).
		Post(monitoringAPI)
	if err != nil {
		return fmt.Errorf("create monitoring: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("create monitoring: status %d: %s", resp.StatusCode(), resp.String())
	}
	logger.Debug("create monitoring response", "status", resp.StatusCode(), "body", resp.String())
	return nil
}

// ─── extension schema ─────────────────────────────────────────────────────────

// extensionSchema is the minimal view of the da-azure settings schema we need:
// a top-level map of enum name → list of allowed values.
type extensionSchema struct {
	Enums map[string]struct {
		Items []struct {
			Value string `json:"value"`
		} `json:"items"`
	} `json:"enums"`
}

// enumValues returns all values of the named enum, or nil if absent.
func (s *extensionSchema) enumValues(key string) []string {
	e, ok := s.Enums[key]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(e.Items))
	for _, it := range e.Items {
		if it.Value != "" {
			out = append(out, it.Value)
		}
	}
	return out
}

// fetchExtensionSchema retrieves the settings schema for a given extension version.
func (d *sdkDTClient) fetchExtensionSchema(version string) (*extensionSchema, error) {
	resp, err := d.c.HTTP().R().Get(fmt.Sprintf("%s/%s/schema", extensionAPI, version))
	if err != nil {
		return nil, fmt.Errorf("fetch extension schema: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("fetch extension schema: status %d: %s", resp.StatusCode(), resp.String())
	}
	logger.Debug("raw extension schema", "version", version, "body", string(resp.Body()))
	var schema extensionSchema
	if err := json.Unmarshal(resp.Body(), &schema); err != nil {
		return nil, fmt.Errorf("parse extension schema: %w", err)
	}
	// Log the enum keys (small, readable) so a wrong/renamed key surfaces even
	// when the raw body is truncated in the terminal.
	keys := make([]string, 0, len(schema.Enums))
	for k := range schema.Enums {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	logger.Debug("extension schema enum keys", "count", len(keys), "keys", keys)
	return &schema, nil
}

func (d *sdkDTClient) latestExtensionVersion() (string, error) {
	type extItem struct {
		Version string `json:"version"`
	}
	type extResp struct {
		Items []extItem `json:"items"`
	}

	var result extResp
	resp, err := d.c.HTTP().R().SetResult(&result).Get(extensionAPI)
	if err != nil {
		return "", fmt.Errorf("get extension versions: %w", err)
	}
	if resp.IsError() {
		return "", fmt.Errorf("get extension versions: status %d: %s", resp.StatusCode(), resp.String())
	}
	if len(result.Items) == 0 {
		return "", fmt.Errorf("no versions found for extension %s", extensionName)
	}

	versions := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		if item.Version != "" {
			versions = append(versions, item.Version)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return cmpSemver(versions[i], versions[j]) > 0
	})
	return versions[0], nil
}

// ─── findConnection ───────────────────────────────────────────────────────────

func (d *sdkDTClient) findConnection(name string) (objectID, clientID string, err error) {
	type item struct {
		ObjectID string    `json:"objectId"`
		Value    connValue `json:"value"`
	}
	type listResp struct {
		Items []item `json:"items"`
	}
	var result listResp
	r, err := d.c.HTTP().R().
		SetResult(&result).
		SetQueryParam("schemaIds", connectionSchemaID).
		SetQueryParam("scopes", "environment").
		Get(settingsAPI)
	if err != nil {
		return "", "", fmt.Errorf("find connection: %w", err)
	}
	if r.IsError() {
		return "", "", fmt.Errorf("find connection: status %d: %s", r.StatusCode(), r.String())
	}
	for _, it := range result.Items {
		if it.Value.Name == name {
			appID := ""
			if it.Value.FederatedIdentityCredential != nil {
				appID = it.Value.FederatedIdentityCredential.ApplicationID
			}
			logger.Debug("found connection", "objectId", it.ObjectID, "name", name, "appId", appID)
			return it.ObjectID, appID, nil
		}
	}
	logger.Debug("connection not found", "name", name)
	return "", "", nil
}

// ─── deleteConnection ─────────────────────────────────────────────────────────

func (d *sdkDTClient) deleteConnection(objectID string) error {
	r, err := d.c.HTTP().R().Delete(fmt.Sprintf("%s/%s", settingsAPI, objectID))
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	if r.IsError() {
		return fmt.Errorf("delete connection: status %d: %s", r.StatusCode(), r.String())
	}
	return nil
}

// ─── findMonitoringConfig ─────────────────────────────────────────────────────

func (d *sdkDTClient) findMonitoringConfig(name string) (string, error) {
	type item struct {
		ObjectID string `json:"objectId"`
		Value    struct {
			Description string `json:"description"`
		} `json:"value"`
	}
	type listResp struct {
		Items []item `json:"items"`
	}
	var result listResp
	r, err := d.c.HTTP().R().SetResult(&result).Get(monitoringAPI)
	if err != nil {
		return "", fmt.Errorf("find monitoring config: %w", err)
	}
	if r.IsError() {
		return "", fmt.Errorf("find monitoring config: status %d: %s", r.StatusCode(), r.String())
	}
	for _, it := range result.Items {
		if it.Value.Description == name {
			logger.Debug("found monitoring config", "objectId", it.ObjectID, "name", name)
			return it.ObjectID, nil
		}
	}
	logger.Debug("monitoring config not found", "name", name)
	return "", nil
}

// ─── deleteMonitoring ─────────────────────────────────────────────────────────

func (d *sdkDTClient) deleteMonitoring(configID string) error {
	r, err := d.c.HTTP().R().Delete(fmt.Sprintf("%s/%s", monitoringAPI, configID))
	if err != nil {
		return fmt.Errorf("delete monitoring config: %w", err)
	}
	if r.IsError() {
		return fmt.Errorf("delete monitoring config: status %d: %s", r.StatusCode(), r.String())
	}
	return nil
}

func cmpSemver(a, b string) int {
	ap, bp := strings.Split(a, "."), strings.Split(b, ".")
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := range n {
		ai, bi := 0, 0
		if i < len(ap) {
			ai, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bi, _ = strconv.Atoi(bp[i])
		}
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	return 0
}
