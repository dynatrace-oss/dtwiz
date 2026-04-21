# Proposal: Ingest Watch

## Why

After installing a monitoring method, users have no visibility into whether data is actually flowing into Dynatrace. They must manually navigate to multiple Dynatrace apps to check. A live terminal watcher that polls for newly ingested data closes this feedback loop — users see services, cloud resources, Kubernetes entities, logs, requests, and exceptions appear in real time, with direct links to the relevant Dynatrace apps.

## What Changes

- New `dtwiz watch` standalone command that polls Dynatrace every 5 seconds and renders a live-updating terminal summary of ingested data
- Each installer's success path calls `WatchIngest()` automatically so users see data flow immediately after installation
- Seven data sections: Services, Cloud, Kubernetes, Relationships, Logs, Requests, Exceptions — each with counts, details, and deep links
- Prominent QuickStart link always shown at the bottom
- ANSI cursor-based in-place rendering (falls back to append-only if not a TTY)
- New dependency: `golang.org/x/term` for TTY detection

## Capabilities

### New Capabilities

- `ingest-watch`: Live terminal display that polls Dynatrace DQL/Smartscape APIs and shows a summary of newly ingested data with deep links to Dynatrace apps

### Modified Capabilities

## Impact

- **New files**: `pkg/installer/ingest_watch.go`, `cmd/watch.go`
- **Modified files**: Each installer file (`oneagent.go`, `kubernetes.go`, `docker.go`, `otel.go`, `aws.go`, etc.) gains a `WatchIngest()` call on success
- **Dependencies**: Adds `golang.org/x/term` (already indirect via `golang.org/x/sys`)
- **APIs**: Uses Dynatrace Platform DQL query API (`/platform/storage/query/v1/query:execute`) and Smartscape DQL commands (`smartscapeNodes`, `smartscapeEdges`)
- **Auth**: Requires platform token (Bearer auth) for DQL queries
