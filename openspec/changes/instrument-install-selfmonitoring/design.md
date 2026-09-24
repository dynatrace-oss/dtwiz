# Design: Instrument Install Self-Monitoring

## Context

`dtwiz setup` is instrumented with four step events (`inv`, `ana`, `rec`, `ist`) plus a `com` event from WatchIngest. Direct `dtwiz install <method>` calls fire only the `inv` event from `installCmd.PersistentPreRun`. This leaves the install path invisible to analytics: no funnel drop-off, no duration, no feature attribution, no pass/fail signal.

The complication is that the AWS, Azure, and GCP installers call WatchIngest themselves, inside the installer function, rather than returning control to `cmd/install.go` first. Adding a callback parameter to thread telemetry through would mean updating every test that calls those functions and there are 15+ callsites in Azure alone. For all other installers, WatchIngest is called from `cmd/install.go` after the installer returns, so wiring a callback there is straightforward.

## Goals / Non-Goals

**Goals:**

- Emit `ist` on install completion for all 13 methods, carrying duration, per-method feature flags, and error indicator
- Emit `com` from WatchIngest for all 13 methods when first data is received or the session times out
- Exclude user think time from `install.duration_ms` so the metric reflects actual install execution time
- Zero behavior change for existing tests

**Non-Goals:**

- A separate `cnf` (confirmed) event — cancellation is inferred from absence of `ist`
- Dynamic feature detection (e.g. detecting actual host-monitoring extension activation success vs. attempt) — static per-method constants are sufficient for v1
- Instrumenting `dtwiz update` or `dtwiz uninstall` — out of scope for this change
- Tracking `install.rds_extension_enabled`, `install.rum_enabled`, `install.synthetic_enabled` — these features are not yet implemented in any installer; fields are absent rather than false

## Decisions

### Package-level vars for shared state (`ExecutionStart`, `OnWatchComplete`)

**Decision:** Two package-level vars in `pkg/installer` — `ExecutionStart time.Time` (set by `confirmProceed` on user confirmation) and `OnWatchComplete func(WatchSessionResult)` (set by cmd handlers for cloud installers) — thread command context into installers without changing function signatures.

**Why over alternatives:**

- *Callback parameters on cloud installers* — would require updating 15+ test callsites in Azure alone; high friction, no architectural benefit since installers are always called sequentially.
- *Return value structs from installers* — would require changing every caller including `cmd/setup.go`; broader blast radius than needed.
- *Context.Context propagation* — appropriate for cancellation/deadlines, not for optional telemetry sidecar state; would require threading context through the entire call chain.

Package-level vars follow the established pattern of `AutoConfirm` already in `pkg/installer` and are safe because only one install runs per process.

### Static per-method feature flags

**Decision:** `installMethodFeatures(method)` returns a hardcoded map per method name. Fields not applicable to a method are absent from the map (and therefore from the event) rather than set to false.

**Why:** Feature activation for a given method is deterministic — oneagent always installs host monitoring, otel always activates the collector pipelines. Dynamic detection (e.g. "did the extension API call actually succeed?") requires installer return-value changes and adds complexity for marginal analytics gain in v1. Absent fields are preferred over false-valued fields to keep events narrow and avoid misleading analytics when features are genuinely unimplemented.

### New WatchIngest `WithEvent` variants for cloud

**Decision:** Add `WatchIngestAWSWithEvent`, `WatchIngestCloudWithEvent`, and `WatchIngestCloudFromTimeWithEvent` that pass an `onEvent` callback to the existing `watchIngest` base function. Cloud installers switch from the plain variants to these, reading `installer.OnWatchComplete` as the callback.

**Why:** The base `watchIngest` already supports an `onEvent func(WatchSessionResult)` parameter — nil disables it. Adding thin public variants is the smallest possible change. Existing plain variants (`WatchIngestAWS`, `WatchIngestCloudFromTime`) remain unchanged so non-install callers are unaffected.

### `ist` event fires after WatchIngest for cloud methods

**Decision:** For AWS, Azure, and GCP, WatchIngest runs blocking inside the installer. The `ist` event fires in the cmd layer after the installer returns — which means after WatchIngest completes. For all other methods, `ist` fires before WatchIngest starts. This is accepted as-is.

**Why:** Reordering would require refactoring cloud installers to return control before WatchIngest, a much larger change. The executionId correlates all events for a run, and step names (`ist` vs `com`) identify ordering; analytics consumers do not depend on wall-clock ordering.

## Risks / Trade-offs

- **`ExecutionStart` accuracy for oneagent** — oneagent has no interactive confirmation, so the cmd layer sets `ExecutionStart = time.Now()` before calling the installer. This includes preflight checks (OS detection, download URL resolution) that run before the actual install binary executes. For analytics purposes this is acceptable; the variance is small compared to the install itself.
- **Package-level vars and future concurrency** — if a future refactor ever runs installs concurrently, `ExecutionStart` and `OnWatchComplete` would race. This is intentional debt: the comment in the code documents the assumption. A future change introducing concurrency must address this.
- **Static feature flags can drift** — if an installer stops activating host monitoring but the feature map still says `true`, the event is misleading. This is caught by code review during installer changes, not by this design.
