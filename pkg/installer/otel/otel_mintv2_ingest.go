package otel

import (
	"context"
	"fmt"
	"maps"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const mintV2IngestSchema = "builtin:opentelemetry-metrics"

// mintV2IngestDocsURL points to the Dynatrace docs page for enabling OTLP metric dimensions via Grail.
const mintV2IngestDocsURL = "https://docs.dynatrace.com/docs/ingest-from/opentelemetry/otlp-api/ingest-otlp-metrics/configure-otlp-metrics#advanced-otlp-metric-dimensions"

type mintV2IngestClient interface {
	getMintV2IngestSetting(ctx context.Context) (objectID, schemaVersion string, value map[string]any, err error)
	putMintV2IngestSetting(ctx context.Context, objectID, schemaVersion string, value map[string]any) error
}

type sdkMintV2IngestClient struct {
	*installer.ExtensionClient
}

func newSDKMintV2IngestClient(envURL, platformToken string) (*sdkMintV2IngestClient, error) {
	ec, err := installer.NewExtensionClient(envURL, platformToken)
	if err != nil {
		return nil, err
	}
	return &sdkMintV2IngestClient{ExtensionClient: ec}, nil
}

func (c *sdkMintV2IngestClient) getMintV2IngestSetting(ctx context.Context) (string, string, map[string]any, error) {
	list, err := c.Settings.ListObjects(ctx, mintV2IngestSchema, "environment", 0)
	if err != nil {
		return "", "", nil, installer.WithScopeHint(fmt.Errorf("get MINTv2 ingest setting: %w", err), "settings:objects:read")
	}
	if len(list.Items) == 0 {
		return "", "", nil, fmt.Errorf("no objects returned for schema %s", mintV2IngestSchema)
	}
	obj := list.Items[0]
	if obj.Value == nil {
		return "", "", nil, fmt.Errorf("MINTv2 ingest setting value is nil")
	}
	return obj.ObjectID, obj.SchemaVersion, obj.Value, nil
}

func (c *sdkMintV2IngestClient) putMintV2IngestSetting(ctx context.Context, objectID, schemaVersion string, value map[string]any) error {
	logger.Debug("putting MINTv2 ingest setting", "objectId", objectID)
	resp, err := c.C.HTTP().R().SetContext(ctx).
		SetBody(map[string]any{"value": value}).
		SetHeader("If-Match", schemaVersion).
		Put(fmt.Sprintf("/platform/classic/environment-api/v2/settings/objects/%s", objectID))
	if err != nil {
		return fmt.Errorf("put MINTv2 ingest setting %q: %w", objectID, err)
	}
	if checkErr := httpclient.CheckResponse(resp); checkErr != nil {
		if violations := installer.ParseConstraintViolations(resp.Body()); len(violations) > 0 {
			logger.Debug("MINTv2 ingest setting update rejected", "objectId", objectID, "violations", installer.FormatConstraintViolations(violations))
			return installer.WithScopeHint(
				fmt.Errorf("put MINTv2 ingest setting %q: %w (%s)", objectID, checkErr, installer.FormatConstraintViolations(violations)),
				"settings:objects:write",
			)
		}
		return installer.WithScopeHint(fmt.Errorf("put MINTv2 ingest setting %q: %w", objectID, checkErr), "settings:objects:write")
	}
	return nil
}

type mintV2IngestPlan struct {
	enabled       bool
	objectID      string
	schemaVersion string
	value         map[string]any
}

var buildMintV2IngestPlanFn = buildMintV2IngestPlan

func buildMintV2IngestPlan(envURL, platformToken string) (mintV2IngestClient, *mintV2IngestPlan, error) {
	c, err := newSDKMintV2IngestClient(envURL, platformToken)
	if err != nil {
		return nil, nil, fmt.Errorf("create MINTv2 ingest client: %w", err)
	}
	objectID, schemaVersion, value, err := c.getMintV2IngestSetting(context.Background())
	if err != nil {
		return nil, nil, err
	}
	enabled, _ := value["enableMintV2Ingest"].(bool)
	return c, &mintV2IngestPlan{
		enabled:       enabled,
		objectID:      objectID,
		schemaVersion: schemaVersion,
		value:         value,
	}, nil
}

func applyMintV2IngestPlan(ctx context.Context, c mintV2IngestClient, plan *mintV2IngestPlan) error {
	if plan == nil || plan.enabled {
		return nil
	}
	mergedValue := make(map[string]any, len(plan.value))
	maps.Copy(mergedValue, plan.value)
	mergedValue["enableMintV2Ingest"] = true
	return c.putMintV2IngestSetting(ctx, plan.objectID, plan.schemaVersion, mergedValue)
}

func printMintV2IngestPlan(plan *mintV2IngestPlan) {
	display.ColorMessage.Println("  OTLP metric dimensions (Grail)")
	display.PrintSectionDivider()
	if plan.enabled {
		display.PrintStatusLine("MINTv2 ingest", "already enabled", display.ColorMuted)
	} else {
		display.PrintStatusLine("MINTv2 ingest", "needs enabling", display.ColorOK)
		display.ColorMuted.Printf("                  %s\n", mintV2IngestDocsURL)
	}
}
