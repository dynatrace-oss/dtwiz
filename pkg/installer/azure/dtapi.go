package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/api/extension"
	"github.com/dynatrace-oss/dtctl/sdk/api/settings"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	// Path constants kept for test assertions.
	settingsAPI        = "/platform/classic/environment-api/v2/settings/objects"
	extensionAPI       = "/platform/extensions/v2/extensions/" + extensionName
	monitoringAPI      = extensionAPI + "/monitoring-configurations"
	connectionSchemaID = "builtin:hyperscaler-authentication.connections.azure"
	extensionName      = "com.dynatrace.extension.da-azure"

	azureLocationEnumKey   = "dynatrace.datasource.azure:location"
	azureFeatureSetEnumKey = "FeatureSetsType"
)

// connRef identifies a Dynatrace Azure connection settings object together with
// the Azure application (client) ID it is bound to, if any.
type connRef struct {
	objectID string
	clientID string
}

// dtclient performs the Dynatrace Platform API calls needed for the Azure integration.
type dtclient interface {
	createConnection(name string) (objectID string, err error)
	updateConnection(objectID, name, tenantID, clientID string) error
	createMonitoring(configName, connectionObjectID, clientID, subscriptionID string) error
	// cleanup — finders return every match so cleanup removes all duplicates.
	findAllConnections(name string) ([]connRef, error)
	deleteConnection(objectID string) error
	findAllMonitoringConfigs(name string) (configIDs []string, err error)
	deleteMonitoring(configID string) error
}

// ─── SDK implementation ───────────────────────────────────────────────────────

type sdkDTClient struct {
	c         *httpclient.Client
	settings  *settings.Handler
	extension *extension.Handler
}

func newSDKDTClient(envURL, platformToken string) (*sdkDTClient, error) {
	appsURL := installer.AppsURL(envURL)
	c, err := httpclient.New(appsURL, httpclient.WithToken(platformToken))
	if err != nil {
		return nil, fmt.Errorf("creating Dynatrace API client: %w", err)
	}
	return &sdkDTClient{
		c:         c,
		settings:  settings.NewHandler(c),
		extension: extension.NewHandler(c),
	}, nil
}

// ─── createConnection ─────────────────────────────────────────────────────────

func (d *sdkDTClient) createConnection(name string) (string, error) {
	resp, err := d.settings.Create(context.Background(), settings.SettingsObjectCreate{
		SchemaID: connectionSchemaID,
		Scope:    "environment",
		Value: map[string]any{
			"name": name,
			"type": "federatedIdentityCredential",
			"federatedIdentityCredential": map[string]any{
				"consumers": []string{"SVC:com.dynatrace.da"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create connection: %w", err)
	}
	logger.Debug("connection created", "objectId", resp.ObjectID)
	return resp.ObjectID, nil
}

// ─── updateConnection ─────────────────────────────────────────────────────────

func (d *sdkDTClient) updateConnection(objectID, name, tenantID, clientID string) error {
	obj, err := d.settings.Get(context.Background(), objectID)
	if err != nil {
		return fmt.Errorf("update connection: get current: %w", err)
	}
	logger.Debug("updating connection", "objectId", objectID, "tenantID", tenantID, "clientID", clientID)
	return d.settings.Update(context.Background(), objectID, obj.SchemaVersion, map[string]any{
		"name": name,
		"type": "federatedIdentityCredential",
		"federatedIdentityCredential": map[string]any{
			"directoryId":   tenantID,
			"applicationId": clientID,
			"consumers":     []string{"SVC:com.dynatrace.da"},
		},
	})
}

// ─── findAllConnections ───────────────────────────────────────────────────────

// findAllConnections returns every Azure connection settings object whose name
// matches. dtwiz always uses a fixed connection name, so a healthy environment
// has at most one; returning all of them lets cleanup remove duplicates left
// behind by earlier interrupted runs.
func (d *sdkDTClient) findAllConnections(name string) ([]connRef, error) {
	list, err := d.settings.ListObjects(context.Background(), connectionSchemaID, "environment", 0)
	if err != nil {
		return nil, fmt.Errorf("find connections: %w", err)
	}
	var refs []connRef
	for _, item := range list.Items {
		n, _ := item.Value["name"].(string)
		if n != name {
			continue
		}
		appID := ""
		if fc, ok := item.Value["federatedIdentityCredential"].(map[string]any); ok {
			appID, _ = fc["applicationId"].(string)
		}
		logger.Debug("found connection", "objectId", item.ObjectID, "name", name, "appId", appID)
		refs = append(refs, connRef{objectID: item.ObjectID, clientID: appID})
	}
	if len(refs) == 0 {
		logger.Debug("connection not found", "name", name)
	}
	return refs, nil
}

// ─── deleteConnection ─────────────────────────────────────────────────────────

func (d *sdkDTClient) deleteConnection(objectID string) error {
	obj, err := d.settings.Get(context.Background(), objectID)
	if err != nil {
		return fmt.Errorf("delete connection: get current: %w", err)
	}
	return d.settings.Delete(context.Background(), objectID, obj.SchemaVersion)
}

// ─── createMonitoring ─────────────────────────────────────────────────────────

func (d *sdkDTClient) createMonitoring(configName, connectionObjectID, clientID, subscriptionID string) error {
	version, err := d.latestExtensionVersion()
	if err != nil {
		return fmt.Errorf("create monitoring: %w", err)
	}
	logger.Debug("using extension version", "version", version)

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

	_, err = d.extension.CreateMonitoringConfiguration(context.Background(), extensionName, extension.MonitoringConfigurationCreate{
		Scope: "integration-azure",
		Value: map[string]any{
			"enabled":     true,
			"description": configName,
			"version":     version,
			"featureSets": featureSets,
			"azure": map[string]any{
				"subscriptionFilteringMode": "INCLUDE",
				"subscriptionFiltering":     []string{subscriptionID},
				"locationFiltering":         locations,
				"credentials": []map[string]any{{
					"enabled":            true,
					"description":        configName,
					"connectionId":       connectionObjectID,
					"servicePrincipalId": clientID,
					"type":               "FEDERATED",
				}},
			},
		},
	})
	return err
}

// ─── findAllMonitoringConfigs ─────────────────────────────────────────────────

// findAllMonitoringConfigs returns the object IDs of every da-azure monitoring
// configuration whose description matches. As with connections, cleanup removes
// all matches so duplicates from interrupted runs don't linger.
func (d *sdkDTClient) findAllMonitoringConfigs(name string) ([]string, error) {
	list, err := d.extension.ListMonitoringConfigurations(context.Background(), extensionName, "", 0)
	if err != nil {
		return nil, fmt.Errorf("find monitoring configs: %w", err)
	}
	var ids []string
	for _, item := range list.Items {
		var val map[string]any
		if err := json.Unmarshal(item.Value, &val); err != nil {
			continue
		}
		if desc, _ := val["description"].(string); desc == name {
			logger.Debug("found monitoring config", "objectId", item.ObjectID, "name", name)
			ids = append(ids, item.ObjectID)
		}
	}
	if len(ids) == 0 {
		logger.Debug("monitoring config not found", "name", name)
	}
	return ids, nil
}

// ─── deleteMonitoring ─────────────────────────────────────────────────────────

func (d *sdkDTClient) deleteMonitoring(configID string) error {
	return d.extension.DeleteMonitoringConfiguration(context.Background(), extensionName, configID)
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

func (d *sdkDTClient) fetchExtensionSchema(version string) (*extensionSchema, error) {
	raw, err := d.extension.GetMonitoringConfigurationSchema(context.Background(), extensionName, version)
	if err != nil {
		return nil, fmt.Errorf("fetch extension schema: %w", err)
	}
	var schema extensionSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("parse extension schema: %w", err)
	}
	keys := make([]string, 0, len(schema.Enums))
	for k := range schema.Enums {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	logger.Debug("extension schema enum keys", "count", len(keys), "keys", keys)
	return &schema, nil
}

func (d *sdkDTClient) latestExtensionVersion() (string, error) {
	versionList, err := d.extension.Get(context.Background(), extensionName)
	if err != nil {
		return "", fmt.Errorf("get extension versions: %w", err)
	}
	if len(versionList.Items) == 0 {
		return "", fmt.Errorf("no versions found for extension %s", extensionName)
	}
	versions := make([]string, 0, len(versionList.Items))
	for _, item := range versionList.Items {
		if item.Version != "" {
			versions = append(versions, item.Version)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return cmpSemver(versions[i], versions[j]) > 0
	})
	return versions[0], nil
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
