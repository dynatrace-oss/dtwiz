# Design: Instrument Setup Self-Monitoring

## Context

Self-monitoring is an existing, feature-flag-gated mechanism (`featureflags.SelfMonitoringPoC`) that fires async telemetry events to the Dynatrace Events v2 API. It is currently used in `cmd/watch.go`. The infrastructure in `pkg/selfmonitoring` and `cmd/selfmonitoring.go` is already in place; this change only adds new constants and call sites.

The `setup` command is a multi-step interactive wizard: it runs analysis, displays ranked recommendations, reads user selection, and then delegates to a method-specific installer. Its internal structure provides natural checkpoints that map directly to the existing `EventParams` fields.

**Current event vocabulary:**

| Field | Existing values |
|---|---|
| `StepID` | `"inv"` (invoked), `"com"` (completed) |
| `SubID` | method shortcodes from `normSubMap` |
| `Err` | (unused in practice so far) |

**normSubMap gaps:** Three update-variant method strings (`"otel-update"`, `"azure-update"`, `"gcp-update"`) returned by `pkg/recommender` have no entry in `normSubMap` and would fall through to the raw string, violating the character-budget convention.

## Goals / Non-Goals

**Goals:**

- Instrument all 4 natural checkpoints of the setup wizard (invocation, analysis, selection, install)
- Enable correlating all events from a single run via the shared `execID`
- Enable measuring step durations and identifying drop-off points
- Cover all selection paths including the demo (`"d"`) and uninstall-help (`"u"`) branches
- Fill the 3 missing `normSubMap` entries for update-variant methods

**Non-Goals:**

- Changing any user-visible behavior in the setup flow
- Adding new feature flags (the existing `SelfMonitoringPoC` flag is sufficient)
- Guaranteed delivery of early events when credentials are absent (best-effort only)
- Fine-grained error categorization (deferred; a single `"err"` value is used for now)

## Decisions

### Event model: 4 events per setup run

**Decision:** Fire one event at each wizard checkpoint rather than a single completion event.

**Rationale:** A single completion event would miss all partial runs (cancelled, credential failure, analysis error). The 4-event model lets the backend reconstruct a session from the `execID` shared across all events, measure time between steps, and identify exactly where a run ended.

**Alternatives considered:**

- Single event at completion: simpler but blind to partial runs, which are the most interesting failure cases.
- Two events (inv + com): misses the analysis and selection signals that are the highest-value data points for understanding user behavior.

### StepID vocabulary: 3 new constants

**Decision:** Add `StepAnalyze = "ana"`, `StepRecommend = "rec"`, `StepInstall = "ist"` to `pkg/selfmonitoring`.

**Rationale:** Keeps step identity in the typed constants layer rather than scattering raw strings across call sites. `"ist"` is chosen over `"ins"` to avoid visual confusion with the `"ins"` cmdID abbreviation for the `install` command (different field, but clearer to readers).

### SubID strategy for setup events

**Decision:** Early events (`inv`, `ana`) carry `SubID = ""`. The `rec` and `ist` events override SubID with the selected method shortcode via `normSub(string(selected.Method))`.

**Rationale:** The method is unknown until the user makes a selection, so early events cannot carry it. The pattern of overriding a field after `buildEventParams` is already established in `watch.go` (which clears `CmdID`).

Special cases:

- `"u"` path: `SubID = "uni"` on `rec` event, no `ist` event
- `"d"` path: `SubID = "demo"` on both `rec` and `ist` events
- `"0"` / cancel: `rec` event not fired; missing `rec` signals cancellation

### normSubMap additions for update variants

**Decision:** Map update variants to distinct 4-char shortcodes: `"otel-update"` → `"otlu"`, `"azure-update"` → `"azu"`, `"gcp-update"` → `"gcpu"`.

**Rationale:** Collapsing update variants to the same code as their install counterparts (e.g., both `"otel"` and `"otel-update"` → `"otel"`) would lose the install-vs-update distinction in telemetry. Distinct shortcodes stay within the 4-char budget and use a consistent `u`-suffix convention — the method name reads first, matching the "full version" (`otel-update`, `azure-update`, `gcp-update`).

### Error field: single "err" value

**Decision:** Set `Err = "err"` for any failure at a step; leave empty on success.

**Rationale:** The step that carries the error already identifies where the failure occurred. Fine-grained error categories (credential, network, permission, etc.) are deferred until there is evidence they are needed for analysis.

## Risks / Trade-offs

- **Credential gap for early events:** `fireSelfMonitoringEvent` resolves credentials internally via `getDtEnvironment()`. For `inv` and `ana` events fired before credentials are validated, if `DT_ENVIRONMENT`/`DT_PLATFORM_TOKEN` are absent the goroutine will log at debug level and return. This is inherent to the onboarding nature of `setup` and acceptable given the best-effort design of self-monitoring.

- **Goroutine lifetime:** Each `fireSelfMonitoringEvent` call launches a goroutine with a 3-second HTTP timeout. For a fast setup run (e.g., credential failure → immediate exit), `inv` goroutine may outlive the process. This is the same risk that exists for `watch.go` and is accepted.

- **normSubMap is not exhaustive:** If the recommender adds new method strings in the future, they will fall through `normSub()` to the raw string value. This is a pre-existing issue, not introduced by this change.
