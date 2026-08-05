package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/api/extension"
	"github.com/dynatrace-oss/dtctl/sdk/api/settings"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// ExtensionClient bundles the Dynatrace Settings and Extensions API handlers
// shared by every hyperscaler installer (azure, gcp, ...). Each installer wraps
// it in its own package-local client type and adds the cloud-specific methods
// (createConnection, updateConnection, createMonitoring, ...) that depend on
// its own settings schema and extension name.
type ExtensionClient struct {
	C         *httpclient.Client
	Settings  *settings.Handler
	Extension *extension.Handler
}

// NewExtensionClient builds an ExtensionClient authenticated against the
// Platform (apps) API of the given Dynatrace environment.
func NewExtensionClient(envURL, platformToken string) (*ExtensionClient, error) {
	appsURL := AppsURL(envURL)
	c, err := httpclient.New(appsURL, httpclient.WithToken(platformToken))
	if err != nil {
		return nil, fmt.Errorf("creating Dynatrace API client: %w", err)
	}
	if logger.IsDebug() {
		c.EnableVerboseLogging(2, os.Stderr)
	}
	return &ExtensionClient{
		C:         c,
		Settings:  settings.NewHandler(c),
		Extension: extension.NewHandler(c),
	}, nil
}

// EnsureInstalled installs extensionName if it isn't already, returning true if it
// was freshly installed. Any error other than "not installed" (a genuine API or
// network failure) is returned immediately instead of being treated as absence.
func (e *ExtensionClient) EnsureInstalled(extensionName string) (bool, error) {
	_, err := e.LatestExtensionVersion(extensionName)
	if err == nil {
		logger.Debug("extension already installed", "extension", extensionName)
		return false, nil
	}
	if !IsExtensionNotFound(err, extensionName) {
		return false, fmt.Errorf("checking installed extension version: %w", err)
	}
	if err := e.InstallExtension(extensionName, ""); err != nil {
		return false, err
	}
	return true, nil
}

// IsExtensionNotFound reports whether err means extensionName is not installed (404),
// as opposed to a real API or network error. The dtctl SDK discards the typed
// *httpclient.APIError for its own 404 handling and returns a plain fmt.Errorf instead
// (see dtctl/sdk/api/extension.go Get/ListMonitoringConfigurations), so errors.Is/
// errors.As cannot detect it there. The typed checks are kept in case a future SDK
// version preserves wrapping; the string check matches the SDK's exact, fixed
// message format rather than a generic "not found" substring.
func IsExtensionNotFound(err error, extensionName string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, httpclient.ErrNotFound) {
		return true
	}
	var apiErr *httpclient.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return true
	}
	return strings.Contains(err.Error(), fmt.Sprintf("extension %q not found", extensionName))
}

// InstallExtension activates a Dynatrace extension version. HTTP 409 is treated
// as success (extension already installed) so install/update flows can safely
// reconcile prerequisites. The SDK wraps 409 into a named error rather than
// *httpclient.APIError, so both the status-code and the string check are required
// to cover all SDK-surfaced forms of "already installed".
func (e *ExtensionClient) InstallExtension(extensionName, version string) error {
	logger.Debug("installing extension", "extension", extensionName, "version", version)
	installed, err := e.Extension.InstallFromHub(context.Background(), extensionName, version)
	if err != nil {
		var apiErr *httpclient.APIError
		alreadyInstalled := (errors.As(err, &apiErr) && apiErr.StatusCode == 409) ||
			strings.Contains(strings.ToLower(err.Error()), "already installed")
		if alreadyInstalled {
			logger.Debug("extension already installed", "extension", extensionName, "version", version)
			return nil
		}
		return fmt.Errorf("install extension %s@%s: %w", extensionName, version, err)
	}
	logger.Debug("extension installed", "extension", installed.ExtensionName, "version", installed.Version)
	return nil
}

// ActivateExtension sets extensionName@version as the active environment configuration.
// HTTP 409 (already active) is treated as success — the call is idempotent.
func (e *ExtensionClient) ActivateExtension(extensionName, version string) error {
	logger.Debug("activating extension", "extension", extensionName, "version", version)
	resp, err := e.C.HTTP().R().SetContext(context.Background()).
		SetBody(map[string]string{"version": version}).
		Post(fmt.Sprintf("/platform/extensions/v2/extensions/%s/environment-configuration", url.PathEscape(extensionName)))
	if err != nil {
		return fmt.Errorf("activate extension %s@%s: %w", extensionName, version, err)
	}
	if err := httpclient.CheckResponse(resp); err != nil {
		var apiErr *httpclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			logger.Debug("extension environment configuration already active", "extension", extensionName, "version", version)
			return nil
		}
		return fmt.Errorf("activate extension %s@%s: %w", extensionName, version, err)
	}
	logger.Debug("extension activated", "extension", extensionName, "version", version)
	return nil
}

// DeleteConnection deletes a Settings object by ID; a 404 (already gone) is treated as success.
func (e *ExtensionClient) DeleteConnection(objectID string) error {
	obj, err := e.Settings.Get(context.Background(), objectID)
	if err != nil {
		if errors.Is(err, httpclient.ErrNotFound) {
			logger.Debug("connection already gone", "objectId", objectID)
			return nil
		}
		return fmt.Errorf("delete connection: get current: %w", err)
	}
	logger.Debug("deleting connection", "objectId", objectID, "schemaVersion", obj.SchemaVersion)
	if err := e.Settings.Delete(context.Background(), objectID, obj.SchemaVersion); err != nil {
		if errors.Is(err, httpclient.ErrNotFound) {
			logger.Debug("connection already gone", "objectId", objectID)
			return nil
		}
		return err
	}
	return nil
}

// FindAllMonitoringConfigs returns the object IDs of every monitoring configuration
// under extensionName whose "description" field equals name.
// A 404 (extension not installed) is treated as an empty result. If the extension
// is absent, there are no monitoring configs to find.
func (e *ExtensionClient) FindAllMonitoringConfigs(extensionName, name string) ([]string, error) {
	list, err := e.Extension.ListMonitoringConfigurations(context.Background(), extensionName, "", 0)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not found") || strings.Contains(msg, "404") {
			logger.Debug("extension not installed, no monitoring configs", "extension", extensionName)
			return nil, nil
		}
		return nil, fmt.Errorf("find monitoring configs: %w", err)
	}
	var ids []string
	for _, item := range list.Items {
		var val map[string]any
		if err := json.Unmarshal(item.Value, &val); err != nil {
			continue
		}
		if desc, _ := val["description"].(string); MatchesIntegrationName(desc, name) {
			logger.Debug("found monitoring config", "objectId", item.ObjectID, "name", name)
			ids = append(ids, item.ObjectID)
		}
	}
	if len(ids) == 0 {
		logger.Debug("monitoring config not found", "name", name)
	}
	return ids, nil
}

// DeleteMonitoringConfiguration deletes a monitoring configuration; a 404 (already gone) is treated as success.
func (e *ExtensionClient) DeleteMonitoringConfiguration(extensionName, configID string) error {
	err := e.Extension.DeleteMonitoringConfiguration(context.Background(), extensionName, configID)
	if errors.Is(err, httpclient.ErrNotFound) {
		logger.Debug("monitoring config already gone", "configId", configID)
		return nil
	}
	return err
}

// ExtensionSchema is the subset of a Dynatrace extension's monitoring-configuration
// schema needed to read enum values (e.g. supported locations or feature sets).
type ExtensionSchema struct {
	Enums map[string]struct {
		Items []struct {
			Value string `json:"value"`
		} `json:"items"`
	} `json:"enums"`
}

// EnumValues returns the non-empty values of the schema enum identified by key.
func (s *ExtensionSchema) EnumValues(key string) []string {
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

// EssentialFeatureSets returns every value in the enum identified by key whose
// name ends with "_essential". Returns an error if none are found — a monitoring
// config created without feature sets would silently collect no data.
func (s *ExtensionSchema) EssentialFeatureSets(key string) ([]string, error) {
	var out []string
	for _, fs := range s.EnumValues(key) {
		if strings.HasSuffix(fs, "_essential") {
			out = append(out, fs)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no \"_essential\" feature sets found under enum %q in extension schema", key)
	}
	return out, nil
}

// FetchExtensionSchema fetches and parses the monitoring-configuration schema for
// the given extension version.
func (e *ExtensionClient) FetchExtensionSchema(extensionName, version string) (*ExtensionSchema, error) {
	raw, err := e.Extension.GetMonitoringConfigurationSchema(context.Background(), extensionName, version)
	if err != nil {
		return nil, fmt.Errorf("fetch extension schema: %w", err)
	}
	var schema ExtensionSchema
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

// IsExtensionActive reports whether any installed version of extensionName has Active == true.
// The DT hub install is asynchronous (202 Accepted); this is the readiness signal.
func (e *ExtensionClient) IsExtensionActive(extensionName string) (bool, error) {
	versionList, err := e.Extension.Get(context.Background(), extensionName)
	if err != nil {
		return false, fmt.Errorf("get extension versions: %w", err)
	}
	for _, item := range versionList.Items {
		if item.Active {
			return true, nil
		}
	}
	return false, nil
}

// ConstraintViolation is a single rejected-field detail from the Dynatrace Settings API.
// httpclient.CheckResponse parses only the top-level "Constraints violated." message and
// discards the nested constraintViolations array, so callers that need field-level detail
// must parse the raw response body with ParseConstraintViolations.
type ConstraintViolation struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ParseConstraintViolations extracts the constraintViolations array from a raw
// Dynatrace Settings API error response body.
func ParseConstraintViolations(body []byte) []ConstraintViolation {
	var envelope struct {
		Error struct {
			ConstraintViolations []ConstraintViolation `json:"constraintViolations"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	return envelope.Error.ConstraintViolations
}

// FormatConstraintViolations renders violations as "path: message; path: message".
func FormatConstraintViolations(violations []ConstraintViolation) string {
	details := make([]string, len(violations))
	for i, v := range violations {
		details[i] = fmt.Sprintf("%s: %s", v.Path, v.Message)
	}
	return strings.Join(details, "; ")
}

// WithScopeHint annotates a Settings API error with the platform-token scope that is
// likely missing, when the failure is a 401 or 403. Without this, the API returns only
// a bare "Access denied" with no indication of which scope is required.
func WithScopeHint(err error, scope string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, httpclient.ErrForbidden) || errors.Is(err, httpclient.ErrUnauthorized) {
		return fmt.Errorf("%w (platform token may be missing the %q scope)", err, scope)
	}
	return err
}

// LatestExtensionVersion returns the highest semver version installed for extensionName.
func (e *ExtensionClient) LatestExtensionVersion(extensionName string) (string, error) {
	versionList, err := e.Extension.Get(context.Background(), extensionName)
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

// cmpSemver compares two dotted version strings numerically, segment by segment
// (missing trailing segments count as 0), returning -1, 0, or 1.
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
