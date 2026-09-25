# Design

## Event structure

Each recommend event is sent via the existing self-monitoring pipeline. The pipeline encodes fields into two places:

- **User-Agent header** (64-char HAProxy capture limit): abbreviated key=value pairs; compact but queryable.
- **Event body properties** (no size constraint): full, human-readable names; stable query surface.

## New step constants

The distinction between "options shown" and "option picked" is encoded as two new step constants rather than a separate field. This keeps the step as the primary signal for filtering events and avoids collisions with other uses of existing fields.

| Event phase            | Step constant                  | Step shortcode | Body `step` value              |
| ---------------------- | ------------------------------ | -------------- | ------------------------------ |
| Recommendation shown   | `StepRecommendationsPresented` | `rpr`          | `recommendations_presented`    |
| User picks option      | `StepRecommendationsSelected`  | `rsl`          | `recommendations_selected`     |

## Option field

The selected/presented ingestion method is encoded in a dedicated `opt=` header field, distinct from `s=` (subcommand). `s=` is reserved for actual Cobra subcommands. Both fields reuse the same abbreviated method identifiers.

Body key: `option` (full method name, e.g. `otel-collector`).

## Technology field

For the OTel recommendation, the event also carries the detected application runtime. Because each event carries at most one runtime, splitting into one event per runtime is the correct model rather than a comma-separated list in a single event.

Header field: `tx=<short>` (e.g. `tx=nd` for Node.js).
Body key: `technology` (full name, e.g. `Node.js`).

Non-OTel methods never carry a technology field.

## Worst-case header example

```
dtwiz/0.18.0;c=set;st=rpr;opt=otlc;tx=nd
```

Length: 39 chars, well within the 64-char limit.

## One event per runtime for OTel

When the OTel option is presented and runtimes are detected, one event is emitted per runtime. This avoids encoding multi-value lists in the User-Agent and keeps every event atomic. When no runtimes are detected, a single OTel event is still emitted without a technology field.

## Correlation across events

All events emitted for a single `dtwiz setup` invocation share the same `executionId` in the event body. This allows DQL queries to correlate presented and selected events for the same session.

## Helper method boundary

`fireSetupMenuEvent` in `cmd/selfmonitoring.go` hides the multi-event fan-out from the call site in `cmd/setup.go`. The call site passes the list of actionable recommendations and detected tech names; the helper decides how many events to fire.

Encoding (abbreviation, body key names) stays in `pkg/selfmonitoring`.
