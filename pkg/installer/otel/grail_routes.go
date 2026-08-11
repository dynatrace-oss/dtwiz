// Package otel provides OpenTelemetry Collector installer logic.
package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtctl/sdk/api/settings"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const grailPipelineName = "OpenTelemetry Host Monitoring"

const otelExtensionName = "com.dynatrace.extension.opentelemetry"

// Case matters: metrics uses uppercase AND; logs/spans use lowercase and.
const (
	grailMatcherMetrics = `isNotNull(host.id) AND matchesValue(metric.key, {"system.*", "process.*"})`
	grailMatcherLogs    = `isNotNull(host.id) AND isNotNull(host.name) AND matchesValue(dt.openpipeline.source, "/api/v2/otlp/v1/logs")`
	grailMatcherSpans   = `isNotNull(host.id) AND isNotNull(host.name) AND matchesValue(telemetry.sdk.name, {"opentelemetry", "odin", "otel"})`
)

type grailSignal struct {
	name           string // used in logs/errors
	displayName    string // used in UI output
	pipelineSchema string // builtin:openpipeline.<signal>.pipelines
	routingSchema  string // builtin:openpipeline.<signal>.routing
	matcher        string
}

var grailSignals = []grailSignal{
	{
		name:           "metrics",
		displayName:    "Metrics",
		pipelineSchema: "builtin:openpipeline.metrics.pipelines",
		routingSchema:  "builtin:openpipeline.metrics.routing",
		matcher:        grailMatcherMetrics,
	},
	{
		name:           "logs",
		displayName:    "Logs",
		pipelineSchema: "builtin:openpipeline.logs.pipelines",
		routingSchema:  "builtin:openpipeline.logs.routing",
		matcher:        grailMatcherLogs,
	},
	{
		name:           "spans",
		displayName:    "Spans",
		pipelineSchema: "builtin:openpipeline.spans.pipelines",
		routingSchema:  "builtin:openpipeline.spans.routing",
		matcher:        grailMatcherSpans,
	},
}

// routingEntry mirrors one item of a routing schema's value.routingEntries[] array.
type routingEntry struct {
	Enabled      bool   `json:"enabled"`
	PipelineType string `json:"pipelineType"`
	PipelineID   string `json:"pipelineId"`
	Matcher      string `json:"matcher"`
	Description  string `json:"description"`
}

type grailAction int

const (
	grailActionNoop     grailAction = iota // route exists and is enabled — no write needed
	grailActionCreate                      // route is absent — append new entry
	grailActionReEnable                    // route exists but disabled — set enabled=true
	grailActionSkip                        // pipeline not found — skip safely
)

type grailSignalPlan struct {
	signal        grailSignal
	action        grailAction
	pipelineObjID string         // pipeline's Settings objectId; used as the routing entry's pipelineId (a setting reference)
	routingObjID  string         // routing settings object ID for PUT; empty when the singleton must be created
	schemaVersion string         // If-Match value
	entries       []routingEntry // current routing entries (snapshot at plan time)
	entryIdx      int            // index in entries of the matching entry (ReEnable only); -1 otherwise
}

// grailRouteClient abstracts the Dynatrace API calls needed for route
// reconciliation. The interface exists for unit-test injection.
type grailRouteClient interface {
	// checkPipeline returns the OTel-owned pipeline's Settings objectId in pipelineSchema,
	// or ("", nil) if none exists (not an error). Other failures propagate.
	checkPipeline(ctx context.Context, pipelineSchema string) (objectID string, err error)
	getRoutingEntries(ctx context.Context, schemaID string) (objectID, schemaVersion string, entries []routingEntry, err error)
	putRoutingEntries(ctx context.Context, objectID, schemaVersion string, entries []routingEntry) error
	// createRoutingObject POST-creates the singleton routing config when it doesn't exist yet.
	createRoutingObject(ctx context.Context, schemaID string, entries []routingEntry) error
}

type sdkGrailClient struct {
	*installer.ExtensionClient
}

func newSDKGrailClient(envURL, platformToken string) (*sdkGrailClient, error) {
	ec, err := installer.NewExtensionClient(envURL, platformToken)
	if err != nil {
		return nil, err
	}
	return &sdkGrailClient{ExtensionClient: ec}, nil
}

func (c *sdkGrailClient) checkPipeline(ctx context.Context, pipelineSchema string) (string, error) {
	logger.Debug("looking up OTel host-monitoring pipeline", "schema", pipelineSchema, "extension", otelExtensionName)
	list, err := c.Settings.ListObjects(ctx, pipelineSchema, "environment", 0)
	if err != nil {
		return "", installer.WithScopeHint(fmt.Errorf("list pipelines for %s: %w", pipelineSchema, err), "settings:objects:read")
	}
	for _, obj := range list.Items {
		if strings.HasPrefix(obj.ExternalID, otelExtensionName+"_") {
			logger.Debug("pipeline found", "schema", pipelineSchema, "objectId", obj.ObjectID)
			return obj.ObjectID, nil
		}
	}
	logger.Debug("pipeline not found", "schema", pipelineSchema)
	return "", nil
}

func (c *sdkGrailClient) getRoutingEntries(ctx context.Context, schemaID string) (string, string, []routingEntry, error) {
	list, err := c.Settings.ListObjects(ctx, schemaID, "environment", 0)
	if err != nil {
		return "", "", nil, installer.WithScopeHint(fmt.Errorf("get routing object for %s: %w", schemaID, err), "settings:objects:read")
	}
	if len(list.Items) == 0 {
		// The routing singleton doesn't exist until the first route is written; absence is
		// valid, not an error. Empty entries + no objectID tells applyGrailPlan to POST-create.
		logger.Debug("routing object absent — will create on first route", "schema", schemaID)
		return "", "", nil, nil
	}
	obj := list.Items[0]
	logger.Debug("got routing object", "schema", schemaID, "objectId", obj.ObjectID, "schemaVersion", obj.SchemaVersion)

	raw, err := json.Marshal(obj.Value["routingEntries"])
	if err != nil {
		return "", "", nil, fmt.Errorf("marshal routing entries for %s: %w", schemaID, err)
	}
	var entries []routingEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return "", "", nil, fmt.Errorf("parse routing entries for %s: %w", schemaID, err)
	}
	return obj.ObjectID, obj.SchemaVersion, entries, nil
}

func (c *sdkGrailClient) putRoutingEntries(ctx context.Context, objectID, schemaVersion string, entries []routingEntry) error {
	logger.Debug("putting routing entries", "objectId", objectID, "count", len(entries))
	// Raw PUT instead of settings.Handler.Update: CheckResponse drops the nested
	// constraintViolations detail we need below. Mirrors gcp/dtapi.go's updateConnection.
	resp, err := c.C.HTTP().R().SetContext(ctx).
		SetBody(map[string]any{"value": map[string]any{"routingEntries": entries}}).
		SetHeader("If-Match", schemaVersion).
		Put(fmt.Sprintf("/platform/classic/environment-api/v2/settings/objects/%s", objectID))
	if err != nil {
		return fmt.Errorf("put routing object %q: %w", objectID, err)
	}
	if checkErr := httpclient.CheckResponse(resp); checkErr != nil {
		if violations := installer.ParseConstraintViolations(resp.Body()); len(violations) > 0 {
			logger.Debug("routing object update rejected", "objectId", objectID, "violations", installer.FormatConstraintViolations(violations))
			return installer.WithScopeHint(fmt.Errorf("put routing object %q: %w (%s)", objectID, checkErr, installer.FormatConstraintViolations(violations)), "settings:objects:write")
		}
		return installer.WithScopeHint(fmt.Errorf("put routing object %q: %w", objectID, checkErr), "settings:objects:write")
	}
	return nil
}

func (c *sdkGrailClient) createRoutingObject(ctx context.Context, schemaID string, entries []routingEntry) error {
	logger.Debug("creating routing object", "schema", schemaID, "count", len(entries))
	resp, err := c.Settings.Create(ctx, settings.SettingsObjectCreate{
		SchemaID: schemaID,
		Scope:    "environment",
		Value:    map[string]any{"routingEntries": entries},
	})
	if err != nil {
		return installer.WithScopeHint(fmt.Errorf("create routing object for %s: %w", schemaID, err), "settings:objects:write")
	}
	logger.Debug("routing object created", "schema", schemaID, "objectId", resp.ObjectID)
	return nil
}

func findRoutingEntry(entries []routingEntry, pipelineID string) (found bool, idx int, enabled bool) {
	for i, e := range entries {
		if e.PipelineID == pipelineID {
			return true, i, e.Enabled
		}
	}
	return false, -1, false
}

func buildGrailPlans(ctx context.Context, c grailRouteClient) ([]grailSignalPlan, error) {
	logger.Debug("building Grail route plans", "signals", len(grailSignals))
	plans := make([]grailSignalPlan, 0, len(grailSignals))
	for _, sig := range grailSignals {
		plan := grailSignalPlan{signal: sig, entryIdx: -1}

		pipelineObjID, err := c.checkPipeline(ctx, sig.pipelineSchema)
		if err != nil {
			return nil, fmt.Errorf("check %s pipeline: %w", sig.name, err)
		}
		if pipelineObjID == "" {
			logger.Debug("pipeline absent, marking signal as skip", "signal", sig.name, "schema", sig.pipelineSchema)
			plan.action = grailActionSkip
			plans = append(plans, plan)
			continue
		}
		plan.pipelineObjID = pipelineObjID

		objID, schemaVer, entries, err := c.getRoutingEntries(ctx, sig.routingSchema)
		if err != nil {
			return nil, fmt.Errorf("get %s routing entries: %w", sig.name, err)
		}
		plan.routingObjID = objID
		plan.schemaVersion = schemaVer
		plan.entries = entries
		logger.Debug("routing entries retrieved", "signal", sig.name, "objectId", objID, "entryCount", len(entries))

		// Match on the pipeline's objectId (not customId) to detect an existing route.
		found, idx, enabled := findRoutingEntry(entries, pipelineObjID)
		switch {
		case !found:
			plan.action = grailActionCreate
			logger.Debug("route absent — will create", "signal", sig.name)
		case enabled:
			plan.action = grailActionNoop
			logger.Debug("route already enabled — noop", "signal", sig.name)
		default:
			plan.action = grailActionReEnable
			plan.entryIdx = idx
			logger.Debug("route disabled — will re-enable", "signal", sig.name, "entryIdx", idx)
		}

		plans = append(plans, plan)
	}
	logger.Debug("Grail route plans built", "create", countAction(plans, grailActionCreate), "reEnable", countAction(plans, grailActionReEnable), "noop", countAction(plans, grailActionNoop), "skip", countAction(plans, grailActionSkip))
	return plans, nil
}

func countAction(plans []grailSignalPlan, action grailAction) int {
	n := 0
	for _, p := range plans {
		if p.action == action {
			n++
		}
	}
	return n
}

// applyGrailPlan assumes the caller has already checked ShouldProceed.
func applyGrailPlan(ctx context.Context, c grailRouteClient, plan grailSignalPlan) error {
	logger.Debug("applying Grail route plan", "signal", plan.signal.name, "action", plan.action)
	switch plan.action {
	case grailActionSkip, grailActionNoop:
		return nil
	case grailActionCreate:
		newEntries := make([]routingEntry, len(plan.entries)+1)
		copy(newEntries, plan.entries)
		newEntries[len(plan.entries)] = routingEntry{
			Enabled:      true,
			PipelineType: "custom",
			PipelineID:   plan.pipelineObjID, // setting reference: the pipeline's Settings objectId
			Matcher:      plan.signal.matcher,
			Description:  grailPipelineName,
		}
		// Empty routingObjID means the singleton doesn't exist yet: POST-create instead of PUT.
		if plan.routingObjID == "" {
			logger.Debug("creating routing object with first route", "signal", plan.signal.name, "schema", plan.signal.routingSchema)
			return c.createRoutingObject(ctx, plan.signal.routingSchema, newEntries)
		}
		logger.Debug("creating route entry", "signal", plan.signal.name, "totalEntries", len(newEntries), "objectId", plan.routingObjID)
		return c.putRoutingEntries(ctx, plan.routingObjID, plan.schemaVersion, newEntries)
	case grailActionReEnable:
		newEntries := make([]routingEntry, len(plan.entries))
		copy(newEntries, plan.entries)
		newEntries[plan.entryIdx].Enabled = true
		logger.Debug("re-enabling route entry", "signal", plan.signal.name, "entryIdx", plan.entryIdx, "objectId", plan.routingObjID)
		return c.putRoutingEntries(ctx, plan.routingObjID, plan.schemaVersion, newEntries)
	}
	return nil
}

// grailApplyMessage reports the outcome after routes are applied; a skip here is final,
// since install/activate/wait have already been attempted by this point.
func grailApplyMessage(action grailAction) (string, *color.Color) {
	switch action {
	case grailActionCreate:
		return "route created", display.ColorOK
	case grailActionReEnable:
		return "route re-enabled", display.ColorOK
	case grailActionNoop:
		return "already configured", display.ColorMuted
	case grailActionSkip:
		return "skip — pipeline not found (re-run install otel once the extension is active)", display.ColorMuted
	}
	return "", display.ColorDefault
}

func printGrailApplyResults(plans []grailSignalPlan, errs []error) {
	fmt.Printf("\n  ── OpenPipeline dynamic routes ──\n\n")
	for i, p := range plans {
		msg, colorFn := grailApplyMessage(p.action)
		if i < len(errs) && errs[i] != nil {
			msg = fmt.Sprintf("warning — %v", errs[i])
			colorFn = display.ColorWarning
		}
		display.PrintStatusLine(p.signal.displayName, msg, colorFn)
	}
	fmt.Println()
}

// grailPreviewMessage is shown before extension activation runs, so a skip here is never
// final: the plan is rebuilt right before applying (see InstallOtelCollectorWithProject).
// Only grailApplyMessage's skip is a final outcome.
func grailPreviewMessage(action grailAction) (string, *color.Color) {
	switch action {
	case grailActionCreate:
		return "create route", display.ColorOK
	case grailActionReEnable:
		return "re-enable route", display.ColorWarning
	case grailActionNoop:
		return "already configured", display.ColorMuted
	case grailActionSkip:
		return "pending — extension not active yet", display.ColorMuted
	}
	return "", display.ColorDefault
}

func printGrailPlan(plans []grailSignalPlan) {
	display.ColorMessage.Println("  OpenPipeline dynamic routes (Smartscape on Grail)")
	display.PrintSectionDivider()
	for _, p := range plans {
		msg, colorFn := grailPreviewMessage(p.action)
		display.PrintStatusLine(p.signal.displayName, msg, colorFn)
	}
}

// waitForGrailPipelinesFn is overridable in tests.
var waitForGrailPipelinesFn = waitForGrailPipelines

// waitForGrailPipelines polls, bounded, until any one signal's pipeline is listable (the
// extension provisions all three together). Called unconditionally: an already-listable
// pipeline satisfies the first check immediately, while a freshly hub-installed one (async,
// 202 Accepted) gets time to propagate. A timeout is advisory; the route step's own
// skip-and-retry-later handling covers it.
func waitForGrailPipelines(ctx context.Context, c grailRouteClient, sleeper func(time.Duration)) error {
	return installer.Retry(sleeper, installer.RetryConfig{
		MaxAttempts: installer.ExtensionActiveMaxAttempts,
		Delay:       func(int) time.Duration { return installer.ExtensionActiveRetryDelay },
		OnRetry: func(attempt int, _ time.Duration, _ error) {
			logger.Debug("OTel host-monitoring pipelines not yet listable, polling", "attempt", attempt)
		},
	}, func() error {
		for _, sig := range grailSignals {
			objID, err := c.checkPipeline(ctx, sig.pipelineSchema)
			if err != nil {
				return err
			}
			if objID != "" {
				return nil
			}
		}
		return fmt.Errorf("no OTel host-monitoring pipeline listable yet")
	})
}

// buildGrailRoutePlans builds the client + plan for otel.go's install preview: shown
// before confirmation, applied afterward without a second prompt.
func buildGrailRoutePlans(envURL, platformToken string) (grailRouteClient, []grailSignalPlan, error) {
	logger.Debug("creating Grail route client", "envURL", envURL)
	c, err := newSDKGrailClient(envURL, platformToken)
	if err != nil {
		return nil, nil, fmt.Errorf("create Grail route client: %w", err)
	}
	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		return nil, nil, err
	}
	return c, plans, nil
}
