# Context

`pkg/installer/ingest_watch.go` implements `watchIngest`, which polls seven DQL queries every 5 seconds and re-renders the terminal in-place using ANSI cursor movement. Prior to this change:

- No timeout: the watch ran indefinitely.
- Logs query: `fetch logs, from:X | summarize count=count(), by:{loglevel}` on every tick — full scan even when empty.
- Requests query: `fetch spans, from:X | filter request.is_root_span == true | summarize failed=countIf(...), success=countIf(...)` on every tick — same problem.

Attempting to replace the summarize queries with `timeseries` (pre-aggregated metrics) proved unreliable: `log.record.count` and `dt.service.request.count` are not standard pre-aggregated metrics in most Dynatrace SaaS environments. The two-phase fetch+summarize approach is guaranteed to work.

## Goals / Non-Goals

**Goals:**

- Cap unbounded watch sessions at 10 minutes with a user-facing continue prompt
- Eliminate expensive full-scan queries during the "waiting for first data" phase for Logs and Requests
- Display a meaningful status ("Logs ingested" / "Requests ingested") during the transition between phases
- Preserve exact in-place rendering behavior after the user continues watching

**Non-Goals:**

- Applying the two-phase optimization to Services, Cloud, Kubernetes, Relationships, or Exceptions sections (those queries are already lightweight or entity-based)
- Persisting the watch state across restarts
- Configurable timeout duration

## Decisions

### Timeout prompt without clearing display

The timeout prompt is printed inline after the last rendered frame without using cursor-up/clear. This preserves visibility of the last known state while asking the user to continue. After "y", `prevLines` is incremented by 1 (to account for the prompt line) so the next render's cursor-up reaches the top of the previous frame and overwrites cleanly.

### Two-phase state machine via `watchQueryState`

A `watchQueryState` struct with two `watchPhase` fields (`logs`, `requests`) is initialized once in `watchIngest` and passed as a pointer to `pollAll`. `watchPhase` is a `uint8` enum with two values: `watchPhaseProbe` and `watchPhaseMetrics`. State transitions are one-way (probe → metrics) and never reset.

### Probe queries

| Section | Probe query |
|---|---|
| Logs | `fetch logs, from:X \| limit 1` |
| Requests | `fetch spans, from:X \| filter request.is_root_span == true \| limit 1` |

A non-empty result (at least one record) triggers the transition to metrics phase.

### `watchSection.Status` field

A new `Status string` field on `watchSection` carries a short status string shown when `Count == 0` but the probe phase has already detected data (e.g., during the first metrics-phase poll that returns zero — unlikely but possible). `renderSection` shows `Status` instead of "waiting..." when it is non-empty and `Count == 0`.

### stdin goroutine and inputCh sharing

The existing byte-by-byte `os.Stdin.Read` goroutine was replaced with a `bufio.NewReader.ReadString('\n')` goroutine writing to `inputCh chan watchInput` (buffer 1). Both the normal key-press stop and the timeout continue response share the same channel. After the user responds to the continue prompt, two non-blocking drains are performed:

1. `select { case <-ticker.C: default: }` — per Go docs requirement after `ticker.Reset`
2. `select { case <-inputCh: default: }` — discard any extra bytes the terminal/bufio may have buffered (prevents spurious stop on next tick)

## Risks / Trade-offs

- [Probe query on every tick until first data] → Acceptable: `| limit 1` is nearly free regardless of dataset size; the full summarize was the expensive operation.
- [Metrics-phase first poll may return zero if data arrives between probe and metrics] → Handled: `watchSection.Status` keeps "Logs ingested" / "Requests ingested" visible until `Count > 0`.
- [bufio.NewReader may buffer more than one line] → Mitigated: the `inputCh` drain after continue discards the extra read; normal watch stop is unaffected because any key press (including Enter) delivers a line and exits.
