# Why

The `dtwiz watch` command ran indefinitely without any timeout, and ran expensive full-scan DQL queries on every 5-second poll even when no data had been ingested yet. Two problems: (1) a session left open overnight burns API quota with no user value; (2) `fetch logs | summarize count=count(), by:{loglevel}` and `fetch spans | summarize ...` scan the entire dataset on every tick, which is slow and expensive when the dataset is empty.

## What Changes

- Watch exits after 10 minutes and prompts "Continue watching? [Y/n]"; pressing y resets the timer and resumes in-place rendering; n exits; non-TTY sessions exit automatically
- Logs queries switch to a two-phase strategy: `fetch logs | limit 1` (cheap probe) until the first log record is detected, then `fetch logs | summarize count=count(), by:{loglevel}` (full summarize only when data exists)
- Requests queries switch to the same two-phase strategy: `fetch spans | filter request.is_root_span == true | limit 1` as probe, then `fetch spans | filter ... | summarize failed=countIf(...), success=countIf(...)` after first span detected
- While in probe phase, the Logs and Requests rows show "Logs ingested" / "Requests ingested" as a status once data is first detected; the full breakdown replaces it once the metrics phase yields counts

## Capabilities

### New Capabilities

- none

### Modified Capabilities

- `ingest-watch`:
  - Add a 10-minute timeout requirement with an interactive continue prompt that resets the timer on "y" and exits on "n"
  - Add non-TTY auto-exit behavior when the timeout fires
  - Add two-phase Logs query optimization (probe → summarize) with "Logs ingested" status display during transition
  - Add two-phase Requests query optimization (probe → summarize) with "Requests ingested" status display during transition

## Impact

- `pkg/installer/ingest_watch.go`: already changed — new types (`watchPhase`, `watchQueryState`, `watchInput`), `watchSection.Status` field, two-phase query logic in `pollAll`, timeout block in main watch loop
- `openspec/specs/ingest-watch/spec.md`: needs new CHANGED requirements for timeout, two-phase Logs, two-phase Requests
