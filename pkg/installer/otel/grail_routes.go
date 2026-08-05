// Package otel provides OpenTelemetry Collector installer logic.
package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/api/settings"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const grailPipelineName = "OpenTelemetry Host Monitoring"

const otelExtensionName = "com.dynatrace.extension.opentelemetry"

// Documented matching conditions for each signal type.
// Metrics uses uppercase AND; logs/spans use lowercase and — confirmed against
// a live tenant and the docs above.
const (
	grailMatcherMetrics = `matchesValue(metric.key, {"system.*", "process.*"}) AND isNotNull(host.id)`
	grailMatcherLogs    = `isNotNull(host.id) and isNotNull(host.name) and matchesValue(dt.openpipeline.source, "/api/v2/otlp/v1/logs")`
	grailMatcherSpans   = `isNotNull(host.id) and isNotNull(host.name) and matchesValue(telemetry.sdk.name, {"opentelemetry", "odin", "otel"})`
)

type grailSignal struct {
	name           string // lowercase, used in error messages
	displayName    string // title-case, used in output
	pipelineSchema string // builtin:openpipeline.<signal>.pipelines — where the extension's pipeline object lives
	routingSchema  string // builtin:openpipeline.<signal>.routing
	matcher        string // documented matching condition
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

// routingEntry mirrors the shape confirmed against a live tenant for all
// three builtin:openpipeline.<signal>.routing schemas.
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
	// checkPipeline finds the OTel-extension-owned pipeline in pipelineSchema and
	// returns its Settings objectId (used as the routing entry's pipelineId, which is
	// a setting reference). Returns ("", nil) when absent (extension not installed or
	// no matching pipeline); propagates other errors.
	checkPipeline(ctx context.Context, pipelineSchema string) (objectID string, err error)
	getRoutingEntries(ctx context.Context, schemaID string) (objectID, schemaVersion string, entries []routingEntry, err error)
	putRoutingEntries(ctx context.Context, objectID, schemaVersion string, entries []routingEntry) error
	// createRoutingObject creates the singleton routing config for schemaID with the
	// given entries. Used when the routing object does not exist yet (fresh tenant).
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
		// The routing config is a singleton (maxObjects=1) that does not exist until
		// the first route is written. On a fresh tenant it is simply absent — a valid
		// state, not an error. Return empty entries with no objectID; applyGrailPlan
		// then POST-creates the object instead of PUT-updating it.
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
	// Bypass settings.Handler.Update here so the raw response body is accessible:
	// CheckResponse parses the top-level "Constraints violated." message but discards
	// the nested constraintViolations array that names the specific offending field.
	// The raw body is needed to surface that detail. This mirrors the pattern in
	// pkg/installer/gcp/dtapi.go (updateConnection).
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

// findRoutingEntry scans entries for one whose pipelineId equals pipelineID.
func findRoutingEntry(entries []routingEntry, pipelineID string) (found bool, idx int, enabled bool) {
	for i, e := range entries {
		if e.PipelineID == pipelineID {
			return true, i, e.Enabled
		}
	}
	return false, -1, false
}

// buildGrailPlans resolves pipeline existence and computes the per-signal action plan.
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

		// Routing entries reference the pipeline by its Settings objectId, so match on
		// pipelineObjID (not the customId) to detect an existing route.
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

// applyGrailPlan writes the route change for a single signal. Noop and skip
// plans are ignored. The caller is responsible for checking ShouldProceed first.
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
		// An empty routingObjID means the singleton routing config does not exist yet
		// (fresh tenant) — POST-create it. Otherwise PUT the appended entry list.
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

// printGrailApplyResults prints the per-signal outcome after route reconciliation runs.
func printGrailApplyResults(plans []grailSignalPlan, errs []error) {
	fmt.Printf("\n  ── OpenPipeline dynamic routes ──\n\n")
	for i, p := range plans {
		var msg string
		var colorFn = display.ColorDefault
		if i < len(errs) && errs[i] != nil {
			msg = fmt.Sprintf("warning — %v", errs[i])
			colorFn = display.ColorWarning
		} else {
			switch p.action {
			case grailActionCreate:
				msg = "route created"
				colorFn = display.ColorOK
			case grailActionReEnable:
				msg = "route re-enabled"
				colorFn = display.ColorOK
			case grailActionNoop:
				msg = "already configured"
				colorFn = display.ColorMuted
			case grailActionSkip:
				msg = "skip — pipeline not found (activate the OpenTelemetry Host Monitoring extension first)"
				colorFn = display.ColorMuted
			}
		}
		display.PrintStatusLine(p.signal.displayName, msg, colorFn)
	}
	fmt.Println()
}

// printGrailPlan prints a one-line summary per signal as part of the install preview.
func printGrailPlan(plans []grailSignalPlan) {
	display.ColorMessage.Println("  OpenPipeline dynamic routes (Smartscape on Grail)")
	display.PrintSectionDivider()
	for _, p := range plans {
		var msg string
		var colorFn = display.ColorDefault
		switch p.action {
		case grailActionCreate:
			msg = "create route"
			colorFn = display.ColorOK
		case grailActionReEnable:
			msg = "re-enable route"
			colorFn = display.ColorWarning
		case grailActionNoop:
			msg = "already configured"
			colorFn = display.ColorMuted
		case grailActionSkip:
			msg = "skip — pipeline not found (activate the OpenTelemetry Host Monitoring extension first)"
			colorFn = display.ColorMuted
		}
		display.PrintStatusLine(p.signal.displayName, msg, colorFn)
	}
}

// buildGrailRoutePlans creates an SDK client and builds the route plan. Intended
// for use from the install preview phase in otel.go so the plan can be shown
// before the confirmation prompt and applied afterwards without a second prompt.
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

// reconcileGrailRoutes is the internal implementation of ReconcileGrailRoutes.
// It includes its own plan/print/confirm/apply cycle and is kept for standalone
// use (e.g. a future "update otel" command). The install flow in otel.go uses
// buildGrailRoutePlans + applyGrailPlan directly instead.
func reconcileGrailRoutes(ctx context.Context, c grailRouteClient, dryRun bool) error {
	plans, err := buildGrailPlans(ctx, c)
	if err != nil {
		return err
	}

	fmt.Println()
	printGrailPlan(plans)
	fmt.Println()

	if proceed, err := installer.ShouldProceed(dryRun, "Route reconciliation"); !proceed {
		return err
	}

	for _, plan := range plans {
		if err := applyGrailPlan(ctx, c, plan); err != nil {
			return fmt.Errorf("apply %s route: %w", plan.signal.name, err)
		}
	}
	return nil
}

// ReconcileGrailRoutes ensures the three OpenPipeline dynamic routes for
// Smartscape on Grail are present. It is additive and idempotent: it only
// creates routes that are absent and re-enables routes that are disabled; it
// never modifies or deletes existing routes. A missing pipeline for a signal
// causes that signal to be skipped safely. This function includes its own
// preview+confirm cycle; the install flow in otel.go uses buildGrailRoutePlans
// and applyGrailPlan directly instead.
func ReconcileGrailRoutes(envURL, platformToken string, dryRun bool) error {
	c, err := newSDKGrailClient(envURL, platformToken)
	if err != nil {
		return fmt.Errorf("create Grail route client: %w", err)
	}
	return reconcileGrailRoutes(context.Background(), c, dryRun)
}
