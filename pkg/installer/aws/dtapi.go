package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/api/extension"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const extensionName = "com.dynatrace.extension.da-aws"

type dtclient interface {
	installExtension() (bool, error)
	isExtensionActive() (bool, error)
	latestExtensionVersion() (string, error)
	findExistingMonitoringConfig(accountID string) (string, error)
	createMonitoringConfig(accountID, region, version string) (string, error)
	enableMonitoringConfig(id string) error
	deleteMonitoringConfig(id string) error
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
	_, err := d.LatestExtensionVersion(extensionName)
	if err == nil {
		logger.Debug("extension already installed", "extension", extensionName)
		return false, nil
	}
	if !isExtensionNotFoundErr(err) {
		return false, fmt.Errorf("checking installed extension version: %w", err)
	}
	if err := d.InstallExtension(extensionName, ""); err != nil {
		return false, err
	}
	return true, nil
}

// isExtensionNotFoundErr reports whether err is a 404 "extension not installed"
// response. The SDK discards the typed *httpclient.APIError for its own 404
// handling, so as a fallback we match its exact, fixed error message instead
// of a generic "not found" substring.
func isExtensionNotFoundErr(err error) bool {
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

func (d *sdkDTClient) isExtensionActive() (bool, error) {
	return d.IsExtensionActive(extensionName)
}

func (d *sdkDTClient) latestExtensionVersion() (string, error) {
	return d.LatestExtensionVersion(extensionName)
}

// awsMonitoringConfigValue is used to locate a monitoring config by AWS account ID.
type awsMonitoringConfigValue struct {
	AWS struct {
		Credentials []struct {
			AccountID string `json:"accountId"`
		} `json:"credentials"`
	} `json:"aws"`
}

func (d *sdkDTClient) findExistingMonitoringConfig(accountID string) (string, error) {
	list, err := d.Extension.ListMonitoringConfigurations(context.Background(), extensionName, "", 0)
	if err != nil {
		if isExtensionNotFoundErr(err) {
			return "", nil
		}
		return "", fmt.Errorf("listing monitoring configs: %w", err)
	}
	for _, item := range list.Items {
		var v awsMonitoringConfigValue
		if err := json.Unmarshal(item.Value, &v); err != nil {
			continue
		}
		for _, cred := range v.AWS.Credentials {
			if cred.AccountID == accountID {
				logger.Debug("found existing monitoring config", "objectId", item.ObjectID, "accountId", accountID)
				return item.ObjectID, nil
			}
		}
	}
	return "", nil
}

func (d *sdkDTClient) createMonitoringConfig(accountID, region, version string) (string, error) {
	desc := fmt.Sprintf("dtwiz — account %s / %s", accountID, region)
	body := extension.MonitoringConfigurationCreate{
		Scope: "integration-aws",
		Value: map[string]any{
			"enabled":           false,
			"description":       desc,
			"version":           version,
			"featureSets":       defaultFeatureSets,
			"activationContext": "DATA_ACQUISITION",
			"aws": map[string]any{
				"deploymentRegion": region,
				"credentials": []map[string]any{
					{
						"description":  desc,
						"enabled":      false,
						"connectionId": "*",
						"accountId":    accountID,
					},
				},
				"regionFiltering":             []string{region},
				"tagFiltering":                []any{},
				"tagEnrichment":               []any{},
				"smartscapeConfiguration":     map[string]any{"enabled": true},
				"metricsConfiguration":        map[string]any{"enabled": true, "regions": []string{region}},
				"cloudWatchLogsConfiguration": map[string]any{"enabled": false, "regions": []string{region}},
				"namespaces":                  []any{},
				"configurationMode":           "QUICK_START",
				"deploymentMode":              "AUTOMATED",
				"deploymentScope":             "SINGLE_ACCOUNT",
				"manualDeploymentStatus":      "NA",
				"automatedDeploymentStatus":   "NA",
			},
		},
	}
	result, err := d.Extension.CreateMonitoringConfiguration(context.Background(), extensionName, body)
	if err != nil {
		return "", err
	}
	logger.Debug("monitoring config created", "objectId", result.ObjectID)
	return result.ObjectID, nil
}

// enableMonitoringConfig flips the da-aws monitoring configuration and all its
// credentials to enabled=true. Mirrors `dtctl enable aws monitoring`:
// without this step the CloudFormation stack is deployed but Dynatrace will
// not actually collect anything.
func (d *sdkDTClient) enableMonitoringConfig(id string) error {
	cfg, err := d.Extension.GetMonitoringConfiguration(context.Background(), extensionName, id)
	if err != nil {
		return fmt.Errorf("get monitoring config: %w", err)
	}

	var value map[string]any
	if err := json.Unmarshal(cfg.Value, &value); err != nil {
		return fmt.Errorf("parse monitoring config value: %w", err)
	}

	value["enabled"] = true
	if aws, ok := value["aws"].(map[string]any); ok {
		if creds, ok := aws["credentials"].([]any); ok {
			for _, cred := range creds {
				if m, ok := cred.(map[string]any); ok {
					m["enabled"] = true
				}
			}
		}
	}

	update := extension.MonitoringConfigurationCreate{
		Scope: cfg.Scope,
		Value: value,
	}
	_, err = d.Extension.UpdateMonitoringConfiguration(context.Background(), extensionName, id, update)
	return err
}

func (d *sdkDTClient) deleteMonitoringConfig(id string) error {
	return d.DeleteMonitoringConfiguration(extensionName, id)
}
