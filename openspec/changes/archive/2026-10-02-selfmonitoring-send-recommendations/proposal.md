# Proposal

## Why

When `dtwiz setup` presents the recommendation menu, we have no visibility into which options were offered to the user or which one they selected. Self-monitoring currently captures only post-selection data, leaving us blind to the funnel: what was shown and what was picked. Without the presented side, a selection count cannot be read as a rate, because there is no denominator.

## What Changes

- Add a new `opt=` header field to `pkg/selfmonitoring` for encoding the presented/selected method, semantically distinct from `s=` (subcommand), which is reserved for actual Cobra subcommands. The event body uses `option` as the full-name key.
- Add `StepRecommendationsPresented` and `StepRecommendationsSelected` step constants to `pkg/selfmonitoring`, giving each phase of the recommendation flow its own step identifier.
- Add `fireSetupMenuEvent` to `cmd/selfmonitoring.go`, firing one `StepRecommendationsPresented` event per presented recommendation.
- **BREAKING**: Update `fireSetupRecommendEvent` to use `opt=` instead of `s=` and `StepRecommendationsSelected` instead of `StepRecommend`, making the selection event consistent with the new scheme.
- Wire `fireSetupMenuEvent` into `cmd/setup.go` immediately after the menu is printed, before `ReadString`.

## Capabilities

### New Capabilities

- `selfmonitoring-recommendation-menu`: Captures which ingestion methods were presented to the user in the setup menu and which one they selected. Enables funnel analysis in DQL by correlating presented and selected events via `executionId`.

### Modified Capabilities

None. The self-monitoring event schema is internal and has no external contract.

## Impact

- `pkg/selfmonitoring/selfmonitoring.go`: new step constants, new `Opt` field on `EventParams`, updated `buildUserAgent` and `buildEventProps`.
- `cmd/selfmonitoring.go`: new `fireSetupMenuEvent` helper; updated `fireSetupRecommendEvent` (**BREAKING** header change).
- `cmd/setup.go`: one new call site.
- No new dependencies and no new feature flag. Self-monitoring is always enabled, so these events ship to every user running `dtwiz setup` against a configured tenant, not to an opt-in subset. See the event-volume note in `design.md`.
