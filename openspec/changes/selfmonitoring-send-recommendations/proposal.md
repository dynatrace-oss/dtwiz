# Proposal

## Why

When `dtwiz setup` presents the recommendation menu, we have no visibility into which options were offered to the user or which one they selected. The existing self-monitoring pipeline only captures post-selection data, leaving us blind to the full funnel: what was shown, what was picked, and — for OTel — what application runtimes were detected.

## What Changes

- Add a new `opt=` header field to `pkg/selfmonitoring` for encoding the presented/selected method — semantically distinct from `s=` (subcommand), which is reserved for actual Cobra subcommands.
- Add `StepRecommendationsPresented` and `StepRecommendationsSelected` step constants to `pkg/selfmonitoring`, giving each phase of the recommendation flow its own step identifier. The event body uses `option` and `technology` as full-name keys.
- Add `Tech string` to `EventParams` with a tech short map (8 runtimes from `detect_project.go`), encoded as `tx=<short>` in the User-Agent header and `technology` (full name) in the event body.
- Add `fireSetupMenuEvent` to `cmd/selfmonitoring.go` — fires one `StepRecommendationsPresented` event per presented recommendation, splitting into one event per detected runtime for the OTel option. Hides the multi-event detail from the call site.
- **BREAKING**: Update `fireSetupRecommendEvent` to use `opt=` instead of `s=` and `StepRecommendationsSelected` instead of `StepRecommend`, making the selection event consistent with the new scheme.
- Wire `fireSetupMenuEvent` into `cmd/setup.go` immediately after the menu is printed, before `ReadString`.

## Capabilities

### New Capabilities

- `selfmonitoring-recommendation-menu`: Captures which ingestion methods were presented to the user in the setup menu and which one they selected, including detected application runtimes for the OTel option. Enables funnel analysis in DQL by correlating presented and selected events via `executionId`.

### Modified Capabilities

None — the self-monitoring event schema is internal and has no external contract.

## Impact

- `pkg/selfmonitoring/selfmonitoring.go`: new step constants, new `Opt` and `Tech` fields on `EventParams`, updated `buildUserAgent` and `buildEventProps`.
- `cmd/selfmonitoring.go`: new `fireSetupMenuEvent` helper; updated `fireSetupRecommendEvent` (**BREAKING** header change).
- `cmd/setup.go`: one new call site.
- No new dependencies, no feature flag required (gated by existing self-monitoring feature flag).
