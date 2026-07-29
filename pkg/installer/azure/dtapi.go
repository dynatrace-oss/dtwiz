package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/api/extension"
	"github.com/dynatrace-oss/dtctl/sdk/api/settings"

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

type connRef struct {
	objectID string
	clientID string
}

type dtclient interface {
	// installExtension ensures the extension is installed. Returns true if it was freshly
	// installed (202 Accepted from hub), false if it was already present.
	installExtension() (bool, error)
	// isExtensionActive reports whether the extension has an Active version. Use after a
	// fresh install to poll until the async hub activation completes.
	isExtensionActive() (bool, error)
	createConnection(name string) (objectID string, err error)
	updateConnection(objectID, name, tenantID, clientID string) error
	createMonitoring(configName, connectionObjectID, clientID, subscriptionID string) error
	updateMonitoring(configID, configName, connectionObjectID, clientID, subscriptionID string) error
	// findAll variants return every name-matching object so cleanup removes duplicates from interrupted runs.
	findAllConnections(name string) ([]connRef, error)
	deleteConnection(objectID string) error
	findAllMonitoringConfigs(name string) (configIDs []string, err error)
	deleteMonitoring(configID string) error
}

type sdkDTClient struct {
	*installer.ExtensionClient
}

func newSDKDTClient(envURL, platformToken string) (*sdkDTClient, error) {
	ec, err := installer.NewExtensionClient(envURL, platformToken)
	if err != nil {
		return nil, err
	}
	return &sdkDTClient{ExtensionClient: ec}, nil
}

func (d *sdkDTClient) installExtension() (bool, error) {
	return d.EnsureInstalled(extensionName)
}

func (d *sdkDTClient) isExtensionActive() (bool, error) {
	return d.IsExtensionActive(extensionName)
}

func (d *sdkDTClient) createConnection(name string) (string, error) {
	resp, err := d.Settings.Create(context.Background(), settings.SettingsObjectCreate{
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

func (d *sdkDTClient) updateConnection(objectID, name, tenantID, clientID string) error {
	obj, err := d.Settings.Get(context.Background(), objectID)
	if err != nil {
		return fmt.Errorf("update connection: get current: %w", err)
	}
	logger.Debug("updating connection", "objectId", objectID, "schemaVersion", obj.SchemaVersion, "currentValue", obj.Value, "tenantID", tenantID, "clientID", clientID)
	return d.Settings.Update(context.Background(), objectID, obj.SchemaVersion, map[string]any{
		"name": name,
		"type": "federatedIdentityCredential",
		"federatedIdentityCredential": map[string]any{
			"directoryId":   tenantID,
			"applicationId": clientID,
			"consumers":     []string{"SVC:com.dynatrace.da"},
		},
	})
}

func (d *sdkDTClient) findAllConnections(name string) ([]connRef, error) {
	list, err := d.Settings.ListObjects(context.Background(), connectionSchemaID, "", 0)
	if err != nil {
		return nil, fmt.Errorf("find connections: %w", err)
	}
	var refs []connRef
	for _, item := range list.Items {
		n, _ := item.Value["name"].(string)
		if !installer.MatchesIntegrationName(n, name) {
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

func (d *sdkDTClient) deleteConnection(objectID string) error {
	return d.DeleteConnection(objectID)
}

// buildMonitoringConfig is shared by createMonitoring and updateMonitoring so both produce identical configs.
// Empty enums are a hard error; a missing location or feature-set list must not silently create a partial config.
func (d *sdkDTClient) buildMonitoringConfig(configName, connectionObjectID, clientID, subscriptionID string) (extension.MonitoringConfigurationCreate, error) {
	var body extension.MonitoringConfigurationCreate

	version, err := d.LatestExtensionVersion(extensionName)
	if err != nil {
		return body, err
	}
	logger.Debug("using extension version", "version", version)

	schema, err := d.FetchExtensionSchema(extensionName, version)
	if err != nil {
		return body, err
	}

	locations := schema.EnumValues(azureLocationEnumKey)
	if len(locations) == 0 {
		return body, fmt.Errorf("no locations found under enum %q in extension schema", azureLocationEnumKey)
	}
	featureSets, err := schema.EssentialFeatureSets(azureFeatureSetEnumKey)
	if err != nil {
		return body, err
	}
	logger.Debug("monitoring defaults from schema", "locations", len(locations), "featureSets", len(featureSets))

	return extension.MonitoringConfigurationCreate{
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
	}, nil
}

func (d *sdkDTClient) createMonitoring(configName, connectionObjectID, clientID, subscriptionID string) error {
	body, err := d.buildMonitoringConfig(configName, connectionObjectID, clientID, subscriptionID)
	if err != nil {
		return fmt.Errorf("create monitoring: %w", err)
	}
	_, err = d.Extension.CreateMonitoringConfiguration(context.Background(), extensionName, body)
	return err
}

// updateMonitoring rewrites the monitoring config in place; the auth chain (connection, SP, fed cred, role) is never touched.
func (d *sdkDTClient) updateMonitoring(configID, configName, connectionObjectID, clientID, subscriptionID string) error {
	body, err := d.buildMonitoringConfig(configName, connectionObjectID, clientID, subscriptionID)
	if err != nil {
		return fmt.Errorf("update monitoring: %w", err)
	}
	_, err = d.Extension.UpdateMonitoringConfiguration(context.Background(), extensionName, configID, body)
	return err
}

func (d *sdkDTClient) findAllMonitoringConfigs(name string) ([]string, error) {
	return d.FindAllMonitoringConfigs(extensionName, name)
}

func (d *sdkDTClient) deleteMonitoring(configID string) error {
	err := d.DeleteMonitoringConfiguration(extensionName, configID)
	if err == nil {
		return nil
	}
	// The extension SDK does not surface 404 as httpclient.ErrNotFound (no %w wrapping),
	// so errors.Is won't work. Fall back to string matching to treat already-gone as success.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "404") {
		logger.Debug("monitoring config already gone", "configId", configID)
		return nil
	}
	return err
}
