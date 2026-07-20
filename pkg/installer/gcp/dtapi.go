package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/api/extension"
	"github.com/dynatrace-oss/dtctl/sdk/api/settings"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// constraintViolation mirrors the detail Dynatrace includes in a rejected settings
// write. httpclient.CheckResponse successfully parses this error shape and keeps only
// the generic top-level "Constraints violated." message, discarding exactly the part
// that says what's actually wrong (e.g. "Unknown property" at a given path) — so it's
// parsed here from the raw response body instead.
type constraintViolation struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func parseConstraintViolations(body []byte) []constraintViolation {
	var envelope struct {
		Error struct {
			ConstraintViolations []constraintViolation `json:"constraintViolations"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	return envelope.Error.ConstraintViolations
}

func formatConstraintViolations(violations []constraintViolation) string {
	details := make([]string, len(violations))
	for i, v := range violations {
		details[i] = fmt.Sprintf("%s: %s", v.Path, v.Message)
	}
	return strings.Join(details, "; ")
}

const (
	// Path constants kept for test assertions.
	settingsAPI   = "/platform/classic/environment-api/v2/settings/objects"
	extensionAPI  = "/platform/extensions/v2/extensions/" + extensionName
	monitoringAPI = extensionAPI + "/monitoring-configurations"

	//   connectionSchemaID   — DT settings schema for the GCP connection.
	//   dtPrincipalSchemaID  — read-only schema exposing the Dynatrace principal
	//                          (the service account that must be granted
	//                          roles/iam.serviceAccountTokenCreator).
	//   extensionName        — the GCP monitoring extension.
	//   monitoringScope      — scope used when creating the monitoring config.
	//   gcpFeatureSetEnumKey — schema enum key listing available feature sets.
	connectionSchemaID  = "builtin:hyperscaler-authentication.connections.gcp"
	dtPrincipalSchemaID = "builtin:hyperscaler-authentication.connections.gcp-dynatrace-principal"
	connectionType      = "serviceAccountImpersonation"
	extensionName       = "com.dynatrace.extension.da-gcp"
	monitoringScope     = "integration-gcp"

	gcpFeatureSetEnumKey = "FeatureSetsType"
)

// errNoPrincipal is returned by dtServiceAccount when the gcp-dynatrace-principal
// schema exists but contains no service-account email — indicating the GCP integration
// is not yet provisioned on this tenant (currently a Preview feature).
var errNoPrincipal = errors.New("no Dynatrace GCP principal found")

type connRef struct {
	objectID            string
	serviceAccountEmail string
}

// splitConnectionsByCompleteness separates connections that have a bound service account
// (complete — the DT connection is fully usable) from those that don't (incomplete — left
// behind by an install that failed between step 2 and step 6).
func splitConnectionsByCompleteness(conns []connRef) (complete, incomplete []connRef) {
	for _, c := range conns {
		if c.serviceAccountEmail != "" {
			complete = append(complete, c)
		} else {
			incomplete = append(incomplete, c)
		}
	}
	return complete, incomplete
}

type dtclient interface {
	installExtension() error
	createConnection(name string) (objectID string, err error)
	// dtServiceAccount returns the Dynatrace principal granted impersonation rights.
	dtServiceAccount() (email string, err error)
	updateConnection(objectID, name, serviceAccountEmail string) error
	createMonitoring(configName, connectionObjectID, serviceAccountEmail, projectID string) error
	updateMonitoring(configID, configName, connectionObjectID, serviceAccountEmail, projectID string) error
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

func (d *sdkDTClient) installExtension() error {
	if _, err := d.LatestExtensionVersion(extensionName); err == nil {
		logger.Debug("extension already installed", "extension", extensionName)
		return nil
	}
	return d.InstallExtension(extensionName, "")
}

func (d *sdkDTClient) createConnection(name string) (string, error) {
	resp, err := d.Settings.Create(context.Background(), settings.SettingsObjectCreate{
		SchemaID: connectionSchemaID,
		Scope:    "environment",
		Value: map[string]any{
			"name": name,
			"type": connectionType,
			connectionType: map[string]any{
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

// dtServiceAccount resolves the Dynatrace principal from the read-only
// gcp-dynatrace-principal schema. The exact field name is environment-managed,
// so the value is located by scanning for a Google service-account email.
func (d *sdkDTClient) dtServiceAccount() (string, error) {
	// Same quirk as findAllConnections: filtering by scopes=environment returns zero
	// results for this schema even when the object is environment-scoped — drop the
	// filter so the query param is omitted entirely.
	list, err := d.Settings.ListObjects(context.Background(), dtPrincipalSchemaID, "", 0)
	if err != nil {
		return "", fmt.Errorf("resolve Dynatrace GCP principal: %w", err)
	}
	for _, item := range list.Items {
		if email := findServiceAccountEmail(item.Value); email != "" {
			logger.Debug("resolved Dynatrace GCP principal", "email", email)
			return email, nil
		}
	}
	return "", errNoPrincipal
}

func (d *sdkDTClient) updateConnection(objectID, name, serviceAccountEmail string) error {
	obj, err := d.Settings.Get(context.Background(), objectID)
	if err != nil {
		return fmt.Errorf("update connection: get current: %w", err)
	}
	logger.Debug("updating connection", "objectId", objectID, "schemaVersion", obj.SchemaVersion, "serviceAccount", serviceAccountEmail)

	// Confirmed against the live schema (GET .../settings/schemas/builtin:hyperscaler-
	// authentication.connections.gcp?schemaVersion=0.0.5): the property under
	// serviceAccountImpersonation is "serviceAccountId", not "serviceAccount" — the
	// latter produced a permanent "Unknown property" 400 on every attempt.
	value := map[string]any{
		"name": name,
		"type": connectionType,
		connectionType: map[string]any{
			"serviceAccountId": serviceAccountEmail,
			"consumers":        []string{"SVC:com.dynatrace.da"},
		},
	}

	// Bypass settings.Update here (rather than delegating to the SDK) so the raw
	// response body is available: CheckResponse discards constraintViolations for
	// this endpoint's error shape, and that detail is exactly what tells a schema
	// mismatch (permanent) apart from a propagation delay (worth retrying).
	resp, err := d.C.HTTP().R().SetContext(context.Background()).
		SetBody(map[string]any{"value": value}).
		SetHeader("If-Match", obj.SchemaVersion).
		Put(fmt.Sprintf("/platform/classic/environment-api/v2/settings/objects/%s", objectID))
	if err != nil {
		return fmt.Errorf("update settings object %q: %w", objectID, err)
	}
	if checkErr := httpclient.CheckResponse(resp); checkErr != nil {
		if violations := parseConstraintViolations(resp.Body()); len(violations) > 0 {
			logger.Debug("connection update rejected", "objectId", objectID, "violations", formatConstraintViolations(violations))
			return fmt.Errorf("update settings object %q: %w (%s)", objectID, checkErr, formatConstraintViolations(violations))
		}
		return fmt.Errorf("update settings object %q: %w", objectID, checkErr)
	}
	return nil
}

func (d *sdkDTClient) findAllConnections(name string) ([]connRef, error) {
	// Confirmed live: filtering by scopes=environment returns zero results for this
	// schema even for objects whose own "scope" field is literally "environment" —
	// dropping the scopes filter (empty string omits the query param entirely, see
	// PaginationParams.QueryParams) is what actually surfaces them. Do not "fix" this
	// back to "environment"; that reintroduces the bug that made dtwiz think every
	// existing connection was invisible.
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
		email := findServiceAccountEmail(item.Value)
		logger.Debug("found connection", "objectId", item.ObjectID, "name", name, "serviceAccount", email)
		refs = append(refs, connRef{objectID: item.ObjectID, serviceAccountEmail: email})
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
// An empty feature-set list is a hard error; a partial config must not be created silently.
func (d *sdkDTClient) buildMonitoringConfig(configName, connectionObjectID, serviceAccountEmail, projectID string) (extension.MonitoringConfigurationCreate, error) {
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

	featureSets, err := schema.EssentialFeatureSets(gcpFeatureSetEnumKey)
	if err != nil {
		return body, err
	}
	logger.Debug("monitoring defaults from schema", "featureSets", len(featureSets))

	// Field names/shape mirror the da-gcp extension schema and dtctl's reference
	// gcpmonitoringconfig types exactly: the block is "googleCloud" (not "gcp"), the
	// credential key is "serviceAccount" (not "serviceAccountId"), and neither a
	// credential "type" nor a "projectFilteringMode" field exists in the schema.
	return extension.MonitoringConfigurationCreate{
		Scope: monitoringScope,
		Value: map[string]any{
			"enabled":     true,
			"description": configName,
			"version":     version,
			"featureSets": featureSets,
			"googleCloud": map[string]any{
				"projectFiltering": []string{projectID},
				"credentials": []map[string]any{{
					"enabled":        true,
					"description":    configName,
					"connectionId":   connectionObjectID,
					"serviceAccount": serviceAccountEmail,
				}},
			},
		},
	}, nil
}

func (d *sdkDTClient) createMonitoring(configName, connectionObjectID, serviceAccountEmail, projectID string) error {
	body, err := d.buildMonitoringConfig(configName, connectionObjectID, serviceAccountEmail, projectID)
	if err != nil {
		return fmt.Errorf("create monitoring: %w", err)
	}
	_, err = d.Extension.CreateMonitoringConfiguration(context.Background(), extensionName, body)
	return err
}

// updateMonitoring rewrites the monitoring config in place; the auth chain (connection, SA, role grants) is never touched.
func (d *sdkDTClient) updateMonitoring(configID, configName, connectionObjectID, serviceAccountEmail, projectID string) error {
	body, err := d.buildMonitoringConfig(configName, connectionObjectID, serviceAccountEmail, projectID)
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
	return d.DeleteMonitoringConfiguration(extensionName, configID)
}
