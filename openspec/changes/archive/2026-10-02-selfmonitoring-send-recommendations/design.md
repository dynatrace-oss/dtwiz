# Design

## Event structure

Each recommend event is sent via the existing self-monitoring pipeline, which encodes fields into two places:

- **User-Agent header** (64-char HAProxy capture limit): abbreviated key=value pairs; compact but queryable.
- **Event body properties** (no size constraint): full, human-readable names; stable query surface.

## New step constants

The distinction between "options shown" and "option picked" is encoded as two new step constants rather than a separate field. This keeps the step as the primary signal for filtering events and avoids collisions with existing fields. In particular, the `t=` / `Type` field could not be reused, as the `watch` command already uses it to carry timing data.

| Event phase          | Step constant                  | Step shortcode | Body `step` value           |
| -------------------- | ------------------------------ | -------------- | --------------------------- |
| Recommendation shown | `StepRecommendationsPresented` | `rpr`          | `recommendations_presented` |
| User picks option    | `StepRecommendationsSelected`  | `rsl`          | `recommendations_selected`  |

## Option field

The presented or selected ingestion method is encoded in a dedicated `opt=` header field, distinct from `s=` (subcommand), which is reserved for actual Cobra subcommands. Both fields reuse the same abbreviated method identifiers.

Body key: `option` (full method name, for example `otel-update`).

### Possible options

Every value `option` can take, with the abbreviated code used in the `opt=` header field:

| Option         | `opt=` code | Source                                              |
| -------------- | ----------- | --------------------------------------------------- |
| `oneagent`     | `oa`        | Recommended method                                  |
| `kubernetes`   | `k8s`       | Recommended method                                  |
| `docker`       | `dock`      | Recommended method (experimental only)              |
| `otel`         | `otel`      | Recommended method (OTel Collector)                 |
| `otel-update`  | `otlu`      | Recommended method (experimental only)              |
| `aws`          | `aws`       | Recommended method                                  |
| `azure`        | `az`        | Recommended method                                  |
| `azure-update` | `azu`       | Recommended method                                  |
| `gcp`          | `gcp`       | Recommended method                                  |
| `gcp-update`   | `gcpu`      | Recommended method                                  |
| `uninstall`    | `uni`       | Setup menu entry `[u]` (selected event only)        |
| `demo`         | `demo`      | Setup menu entry `[d]` (selected event only)        |

The recommended methods are those returned by `recommender.ActionableItems`, so `already-installed` and `not-supported` never appear. `opt=` is only emitted on the two recommendation steps.

## Worst-case header example

```text
dtwiz/0.18.0;c=set;st=rpr;opt=otlu
```

Length: 34 chars, well within the 64-char limit. The longest `opt=` code is 4 chars (`dock`, `otlu`, `gcpu`).

## Event volume

Self-monitoring is always enabled, so this is not an opt-in PoC path: one `dtwiz setup` run now emits one presented event per menu entry instead of a single selection event. A typical menu has a handful of actionable entries, so the per-run cost is a small, bounded fan-out, and the count is capped by the number of recommendations rather than by anything user-controlled.

Emission stays fire-and-forget on the existing pipeline: sends happen on background goroutines and are flushed on exit, delivery failures are logged at debug level only, and an unconfigured tenant simply drops the event. Adding events to the menu path therefore cannot block the prompt or surface an error to the user.

## Correlation across events

All events emitted for a single `dtwiz setup` invocation share the same `executionId` in the event body, letting DQL queries correlate presented and selected events for the same session. This is what turns the two steps into a funnel: group by `executionId`, then compare the `recommendations_presented` set against the single `recommendations_selected` entry.

## Helper method boundary

`fireSetupMenuEvent` in `cmd/selfmonitoring.go` owns the fan-out across recommendations, so the call site in `cmd/setup.go` passes the list of actionable recommendations and nothing more. Encoding (abbreviation, body key names) stays in `pkg/selfmonitoring`.
