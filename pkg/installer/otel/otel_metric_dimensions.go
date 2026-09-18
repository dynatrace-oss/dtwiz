package otel

import (
	"context"
	"fmt"
	"maps"
	"net/http"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	otlpMetricDimensionsSchema  = "builtin:opentelemetry-metrics"
	otlpMetricDimensionsDocsURL = "https://docs.dynatrace.com/docs/ingest-from/opentelemetry/otlp-api/ingest-otlp-metrics/configure-otlp-metrics#advanced-otlp-metric-dimensions"
)

type otlpMetricDimensionsClient interface {
	schemaExists(ctx context.Context) (bool, error)
	getOTLPMetricDimensionsSetting(ctx context.Context) (objectID, schemaVersion string, value map[string]any, err error)
	putOTLPMetricDimensionsSetting(ctx context.Context, objectID, schemaVersion string, value map[string]any) error
}

type sdkOTLPMetricDimensionsClient struct {
	*installer.ExtensionClient
}

func newSDKOTLPMetricDimensionsClient(envURL, platformToken string) (*sdkOTLPMetricDimensionsClient, error) {
	ec, err := installer.NewExtensionClient(envURL, platformToken)
	if err != nil {
		return nil, err
	}
	return &sdkOTLPMetricDimensionsClient{ExtensionClient: ec}, nil
}

// schemaExists returns false when the schema is absent on this tenant (phase 3).
// Checking the schema registry endpoint — not the objects endpoint — makes the
// 404 unambiguous: a routing bug on the objects path would otherwise look identical.
func (c *sdkOTLPMetricDimensionsClient) schemaExists(ctx context.Context) (bool, error) {
	resp, err := c.C.HTTP().R().SetContext(ctx).
		Get(fmt.Sprintf("/platform/classic/environment-api/v2/settings/schemas/%s", otlpMetricDimensionsSchema))
	if err != nil {
		return false, fmt.Errorf("check Advanced OTLP metric dimensions schema: %w", err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return false, nil
	}
	if checkErr := httpclient.CheckResponse(resp); checkErr != nil {
		return false, fmt.Errorf("check Advanced OTLP metric dimensions schema: %w", checkErr)
	}
	return true, nil
}

func (c *sdkOTLPMetricDimensionsClient) getOTLPMetricDimensionsSetting(ctx context.Context) (string, string, map[string]any, error) {
	list, err := c.Settings.ListObjects(ctx, otlpMetricDimensionsSchema, "environment", 0)
	if err != nil {
		return "", "", nil, installer.WithScopeHint(fmt.Errorf("get Advanced OTLP metric dimensions setting: %w", err), "settings:objects:read")
	}
	if len(list.Items) == 0 {
		return "", "", nil, fmt.Errorf("no objects returned for schema %s", otlpMetricDimensionsSchema)
	}
	obj := list.Items[0]
	if obj.Value == nil {
		return "", "", nil, fmt.Errorf("advanced OTLP metric dimensions setting value is nil")
	}
	return obj.ObjectID, obj.SchemaVersion, obj.Value, nil
}

func (c *sdkOTLPMetricDimensionsClient) putOTLPMetricDimensionsSetting(ctx context.Context, objectID, schemaVersion string, value map[string]any) error {
	logger.Debug("putting Advanced OTLP metric dimensions setting", "objectId", objectID)
	resp, err := c.C.HTTP().R().SetContext(ctx).
		SetBody(map[string]any{"value": value}).
		SetHeader("If-Match", schemaVersion).
		Put(fmt.Sprintf("/platform/classic/environment-api/v2/settings/objects/%s", objectID))
	if err != nil {
		return fmt.Errorf("put Advanced OTLP metric dimensions setting %q: %w", objectID, err)
	}
	if checkErr := httpclient.CheckResponse(resp); checkErr != nil {
		if violations := installer.ParseConstraintViolations(resp.Body()); len(violations) > 0 {
			logger.Debug("Advanced OTLP metric dimensions setting update rejected", "objectId", objectID, "violations", installer.FormatConstraintViolations(violations))
			return installer.WithScopeHint(
				fmt.Errorf("put Advanced OTLP metric dimensions setting %q: %w (%s)", objectID, checkErr, installer.FormatConstraintViolations(violations)),
				"settings:objects:write",
			)
		}
		return installer.WithScopeHint(fmt.Errorf("put Advanced OTLP metric dimensions setting %q: %w", objectID, checkErr), "settings:objects:write")
	}
	return nil
}

type otlpMetricDimensionsPlan struct {
	enabled       bool
	objectID      string
	schemaVersion string
	value         map[string]any
}

var buildOTLPMetricDimensionsPlanFn = buildOTLPMetricDimensionsPlan

func buildOTLPMetricDimensionsPlan(envURL, platformToken string) (otlpMetricDimensionsClient, *otlpMetricDimensionsPlan, error) {
	c, err := newSDKOTLPMetricDimensionsClient(envURL, platformToken)
	if err != nil {
		return nil, nil, fmt.Errorf("create Advanced OTLP metric dimensions client: %w", err)
	}
	exists, err := c.schemaExists(context.Background())
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		logger.Debug("Advanced OTLP metric dimensions schema not present on this tenant (phase 3)")
		return nil, nil, nil
	}
	objectID, schemaVersion, value, err := c.getOTLPMetricDimensionsSetting(context.Background())
	if err != nil {
		return nil, nil, err
	}
	enabled, _ := value["enableMintV2Ingest"].(bool)
	return c, &otlpMetricDimensionsPlan{
		enabled:       enabled,
		objectID:      objectID,
		schemaVersion: schemaVersion,
		value:         value,
	}, nil
}

func applyOTLPMetricDimensionsPlan(ctx context.Context, c otlpMetricDimensionsClient, plan *otlpMetricDimensionsPlan) error {
	if plan == nil || plan.enabled {
		return nil
	}
	mergedValue := make(map[string]any, len(plan.value))
	maps.Copy(mergedValue, plan.value)
	mergedValue["enableMintV2Ingest"] = true
	return c.putOTLPMetricDimensionsSetting(ctx, plan.objectID, plan.schemaVersion, mergedValue)
}

func applyOTLPMetricDimensions(ctx context.Context, c otlpMetricDimensionsClient, plan *otlpMetricDimensionsPlan) {
	if plan == nil {
		return
	}
	if err := applyOTLPMetricDimensionsPlan(ctx, c, plan); err != nil {
		display.PrintWarning("Advanced OTLP metric dimensions", err)
		return
	}
	display.ColorOK.Println("  ✓ Advanced OTLP metric dimensions enabled")
}

func printOTLPMetricDimensionsPlan(plan *otlpMetricDimensionsPlan) {
	if plan == nil {
		return
	}
	fmt.Println()
	display.ColorMessage.Println("  Advanced OTLP metric dimensions")
	display.PrintSectionDivider()
	display.PrintStatusLine("Advanced OTLP metric dimensions", "will be enabled", display.ColorOK)
	display.ColorMuted.Printf("                  %s\n", otlpMetricDimensionsDocsURL)
}
