package otel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// ── fake client ───────────────────────────────────────────────────────────────

type fakeRoutingObj struct {
	objectID      string
	schemaVersion string
	entries       []routingEntry
}

type putCall struct {
	objectID      string
	schemaVersion string
	entries       []routingEntry
}

type createCall struct {
	schemaID string
	entries  []routingEntry
}

type fakeGrailClient struct {
	// pipelines maps pipelineSchema -> pipeline Settings objectId. An empty string
	// means no OTel-owned pipeline exists in that schema.
	pipelines map[string]string
	// routing maps routingSchema -> fakeRoutingObj
	routing map[string]fakeRoutingObj
	// absentRouting marks routingSchemas whose singleton object does not exist yet
	absentRouting map[string]bool
	// putCalls records every putRoutingEntries invocation in order
	putCalls []putCall
	// createCalls records every createRoutingObject invocation in order
	createCalls []createCall
	// putErr is returned by putRoutingEntries when non-nil
	putErr error
}

// pipelineObjID is the synthetic Settings objectId the fake returns for a given
// pipeline schema, so tests can assert routing entries reference it.
func pipelineObjID(pipelineSchema string) string {
	return "objid-" + pipelineSchema
}

func (f *fakeGrailClient) checkPipeline(_ context.Context, pipelineSchema string) (string, error) {
	return f.pipelines[pipelineSchema], nil
}

func (f *fakeGrailClient) getRoutingEntries(_ context.Context, schemaID string) (string, string, []routingEntry, error) {
	if f.absentRouting[schemaID] {
		// Singleton routing object not created yet — empty objectID signals create.
		return "", "", nil, nil
	}
	obj, ok := f.routing[schemaID]
	if !ok {
		return "obj-" + schemaID, "v1", nil, nil
	}
	return obj.objectID, obj.schemaVersion, obj.entries, nil
}

func (f *fakeGrailClient) putRoutingEntries(_ context.Context, objectID, schemaVersion string, entries []routingEntry) error {
	f.putCalls = append(f.putCalls, putCall{objectID: objectID, schemaVersion: schemaVersion, entries: entries})
	if f.routing != nil {
		for schemaID, obj := range f.routing {
			if obj.objectID == objectID {
				obj.schemaVersion = schemaVersion
				obj.entries = entries
				f.routing[schemaID] = obj
				break
			}
		}
	}
	return f.putErr
}

func (f *fakeGrailClient) createRoutingObject(_ context.Context, schemaID string, entries []routingEntry) error {
	f.createCalls = append(f.createCalls, createCall{schemaID: schemaID, entries: entries})
	if f.routing != nil {
		f.routing[schemaID] = fakeRoutingObj{objectID: "created-" + schemaID, schemaVersion: "1", entries: entries}
	}
	if f.absentRouting != nil {
		delete(f.absentRouting, schemaID)
	}
	return nil
}

// happyFakeClient returns a fake with all three pipelines present and empty
// routing-entries for every signal (so buildGrailPlans produces create plans).
func happyFakeClient() *fakeGrailClient {
	c := &fakeGrailClient{
		pipelines: map[string]string{},
		routing:   map[string]fakeRoutingObj{},
	}
	for _, sig := range grailSignals {
		c.pipelines[sig.pipelineSchema] = pipelineObjID(sig.pipelineSchema)
		c.routing[sig.routingSchema] = fakeRoutingObj{
			objectID:      "route-obj-" + sig.name,
			schemaVersion: "1",
		}
	}
	return c
}

// ── constants ─────────────────────────────────────────────────────────────────

func TestGrailMatcherConstants(t *testing.T) {
	wantMetrics := `isNotNull(host.id) AND matchesValue(metric.key, {"system.*", "process.*"})`
	wantLogs := `isNotNull(host.id) AND isNotNull(host.name) AND matchesValue(dt.openpipeline.source, "/api/v2/otlp/v1/logs")`
	wantSpans := `isNotNull(host.id) AND isNotNull(host.name) AND matchesValue(telemetry.sdk.name, {"opentelemetry", "odin", "otel"})`

	if grailMatcherMetrics != wantMetrics {
		t.Errorf("grailMatcherMetrics = %q, want %q", grailMatcherMetrics, wantMetrics)
	}
	if grailMatcherLogs != wantLogs {
		t.Errorf("grailMatcherLogs = %q, want %q", grailMatcherLogs, wantLogs)
	}
	if grailMatcherSpans != wantSpans {
		t.Errorf("grailMatcherSpans = %q, want %q", grailMatcherSpans, wantSpans)
	}
}

func TestGrailSignalSchemas(t *testing.T) {
	wantPipelines := map[string]string{
		"metrics": "builtin:openpipeline.metrics.pipelines",
		"logs":    "builtin:openpipeline.logs.pipelines",
		"spans":   "builtin:openpipeline.spans.pipelines",
	}
	wantRouting := map[string]string{
		"metrics": "builtin:openpipeline.metrics.routing",
		"logs":    "builtin:openpipeline.logs.routing",
		"spans":   "builtin:openpipeline.spans.routing",
	}
	for _, sig := range grailSignals {
		if sig.pipelineSchema != wantPipelines[sig.name] {
			t.Errorf("signal %s pipelineSchema = %q, want %q", sig.name, sig.pipelineSchema, wantPipelines[sig.name])
		}
		if sig.routingSchema != wantRouting[sig.name] {
			t.Errorf("signal %s routingSchema = %q, want %q", sig.name, sig.routingSchema, wantRouting[sig.name])
		}
	}
}

// ── withGrailScopeHint ──────────────────────────────────────────────────────────

func TestWithGrailScopeHint(t *testing.T) {
	t.Run("nil error passes through", func(t *testing.T) {
		if err := installer.WithScopeHint(nil, "settings:objects:read"); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("non-auth error is unchanged", func(t *testing.T) {
		orig := errors.New("boom")
		got := installer.WithScopeHint(orig, "settings:objects:read")
		if got != orig {
			t.Errorf("expected error to pass through unchanged, got %v", got)
		}
	})

	t.Run("403 gets scope hint appended", func(t *testing.T) {
		apiErr := httpclient.NewAPIError(403, "Access denied", "")
		got := installer.WithScopeHint(apiErr, "settings:objects:read")
		if got == apiErr {
			t.Fatal("expected error to be wrapped with a hint")
		}
		if !errors.Is(got, httpclient.ErrForbidden) {
			t.Error("wrapped error should still satisfy errors.Is(httpclient.ErrForbidden)")
		}
		if want := `"settings:objects:read"`; !strings.Contains(got.Error(), want) {
			t.Errorf("error %q should mention scope %s", got.Error(), want)
		}
	})

	t.Run("401 gets scope hint appended", func(t *testing.T) {
		apiErr := httpclient.NewAPIError(401, "Unauthorized", "")
		got := installer.WithScopeHint(apiErr, "settings:objects:write")
		if want := `"settings:objects:write"`; !strings.Contains(got.Error(), want) {
			t.Errorf("error %q should mention scope %s", got.Error(), want)
		}
	})

	t.Run("other status codes are unchanged", func(t *testing.T) {
		apiErr := httpclient.NewAPIError(500, "boom", "")
		got := installer.WithScopeHint(apiErr, "settings:objects:read")
		if got != apiErr {
			t.Errorf("expected 500 error to pass through unchanged, got %v", got)
		}
	})
}

// ── buildGrailPlans ───────────────────────────────────────────────────────────

func TestBuildGrailPlans_AllMissing(t *testing.T) {
	c := happyFakeClient()
	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(plans))
	}
	for _, p := range plans {
		if p.action != grailActionCreate {
			t.Errorf("signal %s: want action=create, got %v", p.signal.name, p.action)
		}
		if p.signal.pipelineSchema != "builtin:openpipeline."+p.signal.name+".pipelines" {
			t.Errorf("signal %s: unexpected pipelineSchema %q", p.signal.name, p.signal.pipelineSchema)
		}
		if len(p.entries) != 0 {
			t.Errorf("signal %s: expected empty entries, got %d", p.signal.name, len(p.entries))
		}
	}
}

func TestBuildGrailPlans_AllPresent_Enabled(t *testing.T) {
	c := happyFakeClient()
	for _, sig := range grailSignals {
		obj := c.routing[sig.routingSchema]
		obj.entries = []routingEntry{{
			Enabled:      true,
			PipelineType: "custom",
			PipelineID:   pipelineObjID(sig.pipelineSchema),
			Matcher:      sig.matcher,
			Description:  grailPipelineName,
		}}
		c.routing[sig.routingSchema] = obj
	}

	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range plans {
		if p.action != grailActionNoop {
			t.Errorf("signal %s: want action=noop, got %v", p.signal.name, p.action)
		}
	}
}

func TestBuildGrailPlans_DisabledEntry(t *testing.T) {
	c := happyFakeClient()
	metricsSig := grailSignals[0]
	obj := c.routing[metricsSig.routingSchema]
	obj.entries = []routingEntry{{
		Enabled:      false,
		PipelineType: "custom",
		PipelineID:   pipelineObjID(metricsSig.pipelineSchema),
		Matcher:      grailMatcherMetrics,
		Description:  grailPipelineName,
	}}
	c.routing[metricsSig.routingSchema] = obj

	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plans[0].action != grailActionReEnable {
		t.Errorf("metrics: want action=re-enable, got %v", plans[0].action)
	}
	if plans[0].entryIdx != 0 {
		t.Errorf("metrics: entryIdx = %d, want 0", plans[0].entryIdx)
	}
}

func TestFindRoutingEntry_PrefersEnabledDuplicate(t *testing.T) {
	entries := []routingEntry{
		{Enabled: false, PipelineID: "pipe-1", Matcher: "old"},
		{Enabled: true, PipelineID: "pipe-1", Matcher: "new"},
	}

	found, idx, enabled := findRoutingEntry(entries, "pipe-1")
	if !found {
		t.Fatal("expected to find matching entry")
	}
	if idx != 1 {
		t.Fatalf("idx = %d, want 1", idx)
	}
	if !enabled {
		t.Fatal("expected enabled duplicate to be preferred")
	}
}

func TestBuildGrailPlans_PipelineNotFound(t *testing.T) {
	c := happyFakeClient()
	// Remove logs and spans pipelines.
	c.pipelines[grailSignals[1].pipelineSchema] = ""
	c.pipelines[grailSignals[2].pipelineSchema] = ""

	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plans[0].action != grailActionCreate {
		t.Errorf("metrics: want create, got %v", plans[0].action)
	}
	if plans[1].action != grailActionSkip {
		t.Errorf("logs: want skip, got %v", plans[1].action)
	}
	if plans[2].action != grailActionSkip {
		t.Errorf("spans: want skip, got %v", plans[2].action)
	}
}

// TestBuildGrailPlans_RebuildAfterPipelineAppears guards the otel.go install flow's
// fix for a real bug: the route plan shown in the preview (before the user confirms)
// was being reused as-is for the apply step, even though extension activation runs
// in between and can make a previously-missing pipeline appear. Applying the stale
// preview snapshot would then skip a route that could actually be created in the same
// run. otel.go now calls buildGrailPlans again right before applying; this test
// verifies that a second call against the same client reflects a pipeline that
// became available after the first call, rather than returning a cached result.
func TestBuildGrailPlans_RebuildAfterPipelineAppears(t *testing.T) {
	c := happyFakeClient()
	metricsSchema := grailSignals[0].pipelineSchema
	c.pipelines[metricsSchema] = "" // not active yet, as during the initial preview

	preview, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview[0].action != grailActionSkip {
		t.Fatalf("preview: want skip before activation, got %v", preview[0].action)
	}

	// Simulate the extension activation step (which runs between preview and apply
	// in otel.go) making the pipeline appear.
	c.pipelines[metricsSchema] = pipelineObjID(metricsSchema)

	rebuilt, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rebuilt[0].action != grailActionCreate {
		t.Errorf("rebuilt: want create after the pipeline appears, got %v", rebuilt[0].action)
	}
}

// ── waitForGrailPipelines ───────────────────────────────────────────────────────

// TestWaitForGrailPipelines_AlreadyPresent verifies the wait returns immediately,
// without needing any retries, when a pipeline is already listable.
func TestWaitForGrailPipelines_AlreadyPresent(t *testing.T) {
	c := happyFakeClient()
	sleeps := 0
	err := waitForGrailPipelines(context.Background(), c, func(time.Duration) { sleeps++ })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sleeps != 0 {
		t.Errorf("expected no sleeps when a pipeline is already present, got %d", sleeps)
	}
}

// TestWaitForGrailPipelines_TimesOut verifies the wait gives up and returns an error
// after its bounded number of attempts when no pipeline ever becomes listable, using a
// no-op sleeper so the test doesn't actually wait out the real retry delay.
func TestWaitForGrailPipelines_TimesOut(t *testing.T) {
	c := &fakeGrailClient{pipelines: map[string]string{}}
	err := waitForGrailPipelines(context.Background(), c, func(time.Duration) {})
	if err == nil {
		t.Fatal("expected an error when no pipeline ever becomes listable")
	}
}

// TestBuildGrailPlans_UserBroadenedMatcher verifies that when an entry with the
// same pipelineId but a different matcher exists, it is treated as already-
// configured (noop) and no duplicate entry is added.
func TestBuildGrailPlans_UserBroadenedMatcher(t *testing.T) {
	c := happyFakeClient()
	metricsSig := grailSignals[0]
	obj := c.routing[metricsSig.routingSchema]
	obj.entries = []routingEntry{{
		Enabled:      true,
		PipelineType: "custom",
		PipelineID:   pipelineObjID(metricsSig.pipelineSchema),
		Matcher:      "isNotNull(host.id)", // user broadened the matcher
		Description:  "My custom description",
	}}
	c.routing[metricsSig.routingSchema] = obj

	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plans[0].action != grailActionNoop {
		t.Errorf("metrics: want noop (same pipelineId, user matcher), got %v", plans[0].action)
	}
}

// ── applyGrailPlan ────────────────────────────────────────────────────────────

// TestApplyGrailPlan_SiblingPreserved verifies that when a new route is created,
// any pre-existing sibling entries in routingEntries are preserved in the PUT body.
func TestApplyGrailPlan_SiblingPreserved(t *testing.T) {
	sibling := routingEntry{
		Enabled:      true,
		PipelineType: "custom",
		PipelineID:   "pipe-credit-card-validation",
		Matcher:      "isNotNull(credit_card)",
		Description:  "Credit Card Validation",
	}
	metricsSig := grailSignals[0]
	plan := grailSignalPlan{
		signal:        metricsSig,
		action:        grailActionCreate,
		pipelineObjID: pipelineObjID(metricsSig.pipelineSchema),
		routingObjID:  "route-obj-metrics",
		schemaVersion: "1",
		entries:       []routingEntry{sibling},
		entryIdx:      -1,
	}

	c := &fakeGrailClient{}
	if err := applyGrailPlan(context.Background(), c, plan); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.putCalls) != 1 {
		t.Fatalf("expected 1 PUT call, got %d", len(c.putCalls))
	}
	got := c.putCalls[0].entries
	if len(got) != 2 {
		t.Fatalf("expected 2 entries in PUT body (sibling + new), got %d", len(got))
	}
	// First entry must be the preserved sibling.
	if got[0].PipelineID != sibling.PipelineID {
		t.Errorf("entry[0] pipelineId = %q, want %q (sibling preserved)", got[0].PipelineID, sibling.PipelineID)
	}
	// Second entry must be the newly created route, referencing the pipeline by its
	// Settings objectId (not the customId).
	newEntry := got[1]
	if newEntry.PipelineID != pipelineObjID(metricsSig.pipelineSchema) {
		t.Errorf("entry[1] pipelineId = %q, want %q", newEntry.PipelineID, pipelineObjID(metricsSig.pipelineSchema))
	}
	if !newEntry.Enabled {
		t.Error("new entry should be enabled")
	}
	if newEntry.PipelineType != "custom" {
		t.Errorf("new entry pipelineType = %q, want custom", newEntry.PipelineType)
	}
	if newEntry.Matcher != grailMatcherMetrics {
		t.Errorf("new entry matcher = %q, want documented matcher", newEntry.Matcher)
	}
	if newEntry.Description != grailPipelineName {
		t.Errorf("new entry description = %q, want %q", newEntry.Description, grailPipelineName)
	}
}

// TestApplyGrailPlan_ReEnablePreservesFields verifies that re-enabling a
// disabled route only flips enabled from false to true; all other fields
// (matcher, description, pipelineType) and all sibling entries are unchanged.
func TestApplyGrailPlan_ReEnablePreservesFields(t *testing.T) {
	sibling := routingEntry{
		Enabled:     true,
		PipelineID:  "pipe-sibling",
		Matcher:     "isNotNull(foo)",
		Description: "Sibling",
	}
	metricsSig := grailSignals[0]
	target := routingEntry{
		Enabled:      false,
		PipelineType: "custom",
		PipelineID:   metricsSig.pipelineSchema,
		Matcher:      "isNotNull(host.id)", // user-broadened
		Description:  "User description",
	}
	plan := grailSignalPlan{
		signal:        metricsSig,
		action:        grailActionReEnable,
		routingObjID:  "route-obj-metrics",
		schemaVersion: "2",
		entries:       []routingEntry{sibling, target},
		entryIdx:      1,
	}

	c := &fakeGrailClient{}
	if err := applyGrailPlan(context.Background(), c, plan); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.putCalls) != 1 {
		t.Fatalf("expected 1 PUT call, got %d", len(c.putCalls))
	}
	got := c.putCalls[0].entries
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	// Sibling unchanged.
	if got[0].PipelineID != sibling.PipelineID {
		t.Errorf("sibling pipelineId changed: got %q", got[0].PipelineID)
	}
	// Only enabled changed on the target entry; all other fields preserved.
	re := got[1]
	if !re.Enabled {
		t.Error("re-enabled entry should have Enabled=true")
	}
	if re.Matcher != target.Matcher {
		t.Errorf("matcher changed: got %q, want %q", re.Matcher, target.Matcher)
	}
	if re.Description != target.Description {
		t.Errorf("description changed: got %q, want %q", re.Description, target.Description)
	}
	if re.PipelineType != target.PipelineType {
		t.Errorf("pipelineType changed: got %q, want %q", re.PipelineType, target.PipelineType)
	}
}

// TestApplyGrailPlan_NoopAndSkip verifies that noop and skip plans make no PUT calls.
func TestApplyGrailPlan_NoopAndSkip(t *testing.T) {
	for _, action := range []grailAction{grailActionNoop, grailActionSkip} {
		plan := grailSignalPlan{signal: grailSignals[0], action: action, entryIdx: -1}
		c := &fakeGrailClient{}
		if err := applyGrailPlan(context.Background(), c, plan); err != nil {
			t.Errorf("action %v: unexpected error: %v", action, err)
		}
		if len(c.putCalls) != 0 {
			t.Errorf("action %v: expected no PUT calls, got %d", action, len(c.putCalls))
		}
	}
}

// TestApplyGrailPlan_CreatesRoutingObjectWhenAbsent verifies that a create plan
// with an empty routingObjID (singleton routing config not yet provisioned on a
// fresh tenant) POST-creates the routing object rather than PUT-updating it.
func TestApplyGrailPlan_CreatesRoutingObjectWhenAbsent(t *testing.T) {
	metricsSig := grailSignals[0]
	plan := grailSignalPlan{
		signal:        metricsSig,
		action:        grailActionCreate,
		pipelineObjID: pipelineObjID(metricsSig.pipelineSchema),
		routingObjID:  "", // routing object does not exist yet
		entries:       nil,
		entryIdx:      -1,
	}

	c := &fakeGrailClient{}
	if err := applyGrailPlan(context.Background(), c, plan); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.putCalls) != 0 {
		t.Errorf("expected no PUT calls when routing object absent, got %d", len(c.putCalls))
	}
	if len(c.createCalls) != 1 {
		t.Fatalf("expected 1 create call, got %d", len(c.createCalls))
	}
	call := c.createCalls[0]
	if call.schemaID != metricsSig.routingSchema {
		t.Errorf("create schemaID = %q, want %q", call.schemaID, metricsSig.routingSchema)
	}
	if len(call.entries) != 1 {
		t.Fatalf("expected 1 entry in created object, got %d", len(call.entries))
	}
	e := call.entries[0]
	if e.PipelineID != pipelineObjID(metricsSig.pipelineSchema) {
		t.Errorf("created entry pipelineId = %q, want %q", e.PipelineID, pipelineObjID(metricsSig.pipelineSchema))
	}
	if !e.Enabled || e.PipelineType != "custom" || e.Matcher != grailMatcherMetrics || e.Description != grailPipelineName {
		t.Errorf("created entry has unexpected fields: %+v", e)
	}
}

func TestWaitForGrailRoutesApplied_ReadsBackEnabledRoutes(t *testing.T) {
	c := happyFakeClient()
	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("buildGrailPlans() error = %v", err)
	}
	applyErrs := make([]error, len(plans))
	for i, p := range plans {
		applyErrs[i] = applyGrailPlan(context.Background(), c, p)
	}
	validations := grailRouteValidations(plans, applyErrs)
	if len(validations) != len(grailSignals) {
		t.Fatalf("validations = %d, want %d", len(validations), len(grailSignals))
	}
	if err := waitForGrailRoutesApplied(context.Background(), c, validations, func(time.Duration) {}); err != nil {
		t.Fatalf("waitForGrailRoutesApplied() error = %v", err)
	}
}

func TestGrailRouteValidations_SkipsNoopPlans(t *testing.T) {
	plans := []grailSignalPlan{
		{signal: grailSignals[0], action: grailActionNoop, pipelineObjID: "pipe-metrics"},
		{signal: grailSignals[1], action: grailActionCreate, pipelineObjID: "pipe-logs"},
	}

	validations := grailRouteValidations(plans, make([]error, len(plans)))
	if len(validations) != 1 {
		t.Fatalf("validations = %d, want 1", len(validations))
	}
	if validations[0].signal.name != grailSignals[1].name {
		t.Fatalf("validated signal = %s, want %s", validations[0].signal.name, grailSignals[1].name)
	}
}

func TestWaitForGrailRoutesApplied_TimesOutWhenRouteNotVisible(t *testing.T) {
	c := happyFakeClient()
	validations := []grailRouteValidation{{signal: grailSignals[0], pipelineObjID: pipelineObjID(grailSignals[0].pipelineSchema)}}
	err := waitForGrailRoutesApplied(context.Background(), c, validations, func(time.Duration) {})
	if err == nil {
		t.Fatal("expected an error when the route never becomes visible")
	}
	if !strings.Contains(err.Error(), "routes not visible as enabled") {
		t.Fatalf("error = %v, want missing route message", err)
	}
}

func TestGrailApplyErrsWithValidation_MarksOnlyFailedSignals(t *testing.T) {
	plans := []grailSignalPlan{
		{signal: grailSignals[0], action: grailActionCreate},
		{signal: grailSignals[1], action: grailActionCreate},
		{signal: grailSignals[2], action: grailActionCreate},
	}
	applyErrs := make([]error, len(plans))
	validationErr := &grailRouteValidationError{signalErrors: map[string]error{grailSignals[1].name: nil}}

	finalErrs := grailApplyErrsWithValidation(plans, applyErrs, validationErr)

	if finalErrs[0] != nil {
		t.Fatalf("metrics error = %v, want nil", finalErrs[0])
	}
	if finalErrs[1] == nil || !strings.Contains(finalErrs[1].Error(), "route validation inconclusive") || !strings.Contains(finalErrs[1].Error(), "may become active shortly") {
		t.Fatalf("logs error = %v, want validation failure", finalErrs[1])
	}
	if finalErrs[2] != nil {
		t.Fatalf("spans error = %v, want nil", finalErrs[2])
	}
}

func TestGrailApplyErrsWithValidation_DifferentiatesSetupFailure(t *testing.T) {
	plans := []grailSignalPlan{{signal: grailSignals[0], action: grailActionCreate}}
	applyErrs := []error{errors.New("settings api rejected request")}

	finalErrs := grailApplyErrsWithValidation(plans, applyErrs, nil)

	if finalErrs[0] == nil || !strings.Contains(finalErrs[0].Error(), "route setup failed") {
		t.Fatalf("error = %v, want setup failure", finalErrs[0])
	}
}

// TestBuildGrailPlans_RoutingObjectAbsent verifies that when checkPipeline
// succeeds but the routing singleton is absent, the signal still plans as create
// with an empty routingObjID (to be POST-created on apply).
func TestBuildGrailPlans_RoutingObjectAbsent(t *testing.T) {
	c := happyFakeClient()
	c.absentRouting = map[string]bool{grailSignals[0].routingSchema: true}

	plans, err := buildGrailPlans(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plans[0].action != grailActionCreate {
		t.Errorf("metrics: want create, got %v", plans[0].action)
	}
	if plans[0].routingObjID != "" {
		t.Errorf("metrics: want empty routingObjID (absent singleton), got %q", plans[0].routingObjID)
	}
}

// ── removeGrailRoutes ─────────────────────────────────────────────────────────

// fakeClientWithRoutes returns a fake where every signal has a pipeline and a
// routing entry whose PipelineID matches that pipeline's objectId.
func fakeClientWithRoutes() *fakeGrailClient {
	c := &fakeGrailClient{
		pipelines: map[string]string{},
		routing:   map[string]fakeRoutingObj{},
	}
	for _, sig := range grailSignals {
		objID := pipelineObjID(sig.pipelineSchema)
		c.pipelines[sig.pipelineSchema] = objID
		c.routing[sig.routingSchema] = fakeRoutingObj{
			objectID:      "route-obj-" + sig.name,
			schemaVersion: "1",
			entries: []routingEntry{
				{Enabled: true, PipelineID: objID, Matcher: sig.matcher},
			},
		}
	}
	return c
}

func TestRemoveGrailRoutes_HappyPath(t *testing.T) {
	c := fakeClientWithRoutes()
	removed, errs := removeGrailRoutes(context.Background(), c)
	for i, err := range errs {
		if err != nil {
			t.Errorf("signal %d: unexpected error: %v", i, err)
		}
	}
	for i, r := range removed {
		if !r {
			t.Errorf("signal %d: expected removed=true, got false", i)
		}
	}
	if len(c.putCalls) != len(grailSignals) {
		t.Fatalf("expected %d put calls, got %d", len(grailSignals), len(c.putCalls))
	}
	for i, call := range c.putCalls {
		if len(call.entries) != 0 {
			t.Errorf("put call %d: expected empty entries after removal, got %d", i, len(call.entries))
		}
	}
}

func TestRemoveGrailRoutes_PipelineAbsent(t *testing.T) {
	c := fakeClientWithRoutes()
	// Remove one pipeline to simulate it not existing.
	delete(c.pipelines, grailSignals[0].pipelineSchema)
	removed, errs := removeGrailRoutes(context.Background(), c)
	if errs[0] != nil {
		t.Errorf("signal 0: expected nil error when pipeline absent, got %v", errs[0])
	}
	if removed[0] {
		t.Errorf("signal 0: expected removed=false when pipeline absent")
	}
	// Only two puts for the signals whose pipelines exist.
	if len(c.putCalls) != len(grailSignals)-1 {
		t.Errorf("expected %d put calls, got %d", len(grailSignals)-1, len(c.putCalls))
	}
}

func TestRemoveGrailRoutes_EntryAbsent(t *testing.T) {
	c := fakeClientWithRoutes()
	// Clear the routing entry for the first signal so the pipeline exists but
	// no matching entry is in the routing object.
	sig := grailSignals[0]
	c.routing[sig.routingSchema] = fakeRoutingObj{
		objectID:      "route-obj-" + sig.name,
		schemaVersion: "1",
		entries:       []routingEntry{}, // no entry for this pipeline
	}
	removed, errs := removeGrailRoutes(context.Background(), c)
	if errs[0] != nil {
		t.Errorf("signal 0: expected nil error when entry absent, got %v", errs[0])
	}
	if removed[0] {
		t.Errorf("signal 0: expected removed=false when entry absent")
	}
	// Only two puts for the signals that had entries.
	if len(c.putCalls) != len(grailSignals)-1 {
		t.Errorf("expected %d put calls, got %d", len(grailSignals)-1, len(c.putCalls))
	}
}

func TestRemoveGrailRoutes_PutFails(t *testing.T) {
	c := fakeClientWithRoutes()
	c.putErr = fmt.Errorf("simulated put failure")
	removed, errs := removeGrailRoutes(context.Background(), c)
	for i, err := range errs {
		if err == nil {
			t.Errorf("signal %d: expected error when put fails, got nil", i)
		}
		if removed[i] {
			t.Errorf("signal %d: expected removed=false when put fails", i)
		}
	}
}
