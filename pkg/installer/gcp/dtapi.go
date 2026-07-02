package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
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

	// VERIFY: the following GCP-specific identifiers/value shapes mirror the Azure
	// integration by analogy. They are isolated here so a single edit corrects the
	// whole flow if they differ from the live environment.
	//
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

type connRef struct {
	objectID            string
	serviceAccountEmail string
}

type dtclient interface {
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
	if logger.IsDebug() {
		c.EnableVerboseLogging(2, os.Stderr)
	}
	return &sdkDTClient{
		c:         c,
		settings:  settings.NewHandler(c),
		extension: extension.NewHandler(c),
	}, nil
}

func (d *sdkDTClient) createConnection(name string) (string, error) {
	resp, err := d.settings.Create(context.Background(), settings.SettingsObjectCreate{
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
	list, err := d.settings.ListObjects(context.Background(), dtPrincipalSchemaID, "environment", 0)
	if err != nil {
		return "", fmt.Errorf("resolve Dynatrace GCP principal: %w", err)
	}
	for _, item := range list.Items {
		if email := findServiceAccountEmail(item.Value); email != "" {
			logger.Debug("resolved Dynatrace GCP principal", "email", email)
			return email, nil
		}
	}
	return "", fmt.Errorf("no Dynatrace GCP principal found under schema %q", dtPrincipalSchemaID)
}

func (d *sdkDTClient) updateConnection(objectID, name, serviceAccountEmail string) error {
	obj, err := d.settings.Get(context.Background(), objectID)
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
	resp, err := d.c.HTTP().R().SetContext(context.Background()).
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
	list, err := d.settings.ListObjects(context.Background(), connectionSchemaID, "", 0)
	if err != nil {
		return nil, fmt.Errorf("find connections: %w", err)
	}
	var refs []connRef
	for _, item := range list.Items {
		n, _ := item.Value["name"].(string)
		if n != name {
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
	obj, err := d.settings.Get(context.Background(), objectID)
	if err != nil {
		if errors.Is(err, httpclient.ErrNotFound) {
			logger.Debug("connection already gone", "objectId", objectID)
			return nil
		}
		return fmt.Errorf("delete connection: get current: %w", err)
	}
	logger.Debug("deleting connection", "objectId", objectID, "schemaVersion", obj.SchemaVersion)
	if err := d.settings.Delete(context.Background(), objectID, obj.SchemaVersion); err != nil {
		if errors.Is(err, httpclient.ErrNotFound) {
			logger.Debug("connection already gone", "objectId", objectID)
			return nil
		}
		return err
	}
	return nil
}

// buildMonitoringConfig is shared by createMonitoring and updateMonitoring so both produce identical configs.
// An empty feature-set list is a hard error; a partial config must not be created silently.
func (d *sdkDTClient) buildMonitoringConfig(configName, connectionObjectID, serviceAccountEmail, projectID string) (extension.MonitoringConfigurationCreate, error) {
	var body extension.MonitoringConfigurationCreate

	version, err := d.latestExtensionVersion()
	if err != nil {
		return body, err
	}
	logger.Debug("using extension version", "version", version)

	schema, err := d.fetchExtensionSchema(version)
	if err != nil {
		return body, err
	}

	featureSets := make([]string, 0)
	for _, fs := range schema.enumValues(gcpFeatureSetEnumKey) {
		if strings.HasSuffix(fs, "_essential") {
			featureSets = append(featureSets, fs)
		}
	}
	if len(featureSets) == 0 {
		return body, fmt.Errorf("no \"_essential\" feature sets found under enum %q in extension schema", gcpFeatureSetEnumKey)
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
	_, err = d.extension.CreateMonitoringConfiguration(context.Background(), extensionName, body)
	return err
}

// updateMonitoring rewrites the monitoring config in place; the auth chain (connection, SA, role grants) is never touched.
func (d *sdkDTClient) updateMonitoring(configID, configName, connectionObjectID, serviceAccountEmail, projectID string) error {
	body, err := d.buildMonitoringConfig(configName, connectionObjectID, serviceAccountEmail, projectID)
	if err != nil {
		return fmt.Errorf("update monitoring: %w", err)
	}
	_, err = d.extension.UpdateMonitoringConfiguration(context.Background(), extensionName, configID, body)
	return err
}

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

func (d *sdkDTClient) deleteMonitoring(configID string) error {
	err := d.extension.DeleteMonitoringConfiguration(context.Background(), extensionName, configID)
	if errors.Is(err, httpclient.ErrNotFound) {
		logger.Debug("monitoring config already gone", "configId", configID)
		return nil
	}
	return err
}

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
	if len(versions) == 0 {
		return "", fmt.Errorf("no non-empty versions found for extension %s", extensionName)
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
