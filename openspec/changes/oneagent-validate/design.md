# Design

## Context

`InstallOneAgentV2` (Task 6) returns success as soon as `ExecuteInstallCommand` reports exit code 0. No post-install check confirms the host actually registered with the tenant. `WatchIngest` in `pkg/installer/ingest_watch.go` already polls Grail via the Platform API for general data flow; this change applies the same approach for host-registration confirmation.

## Goals / Non-Goals

**Goals:**

- Implement `WaitForHostRegistration(p *client.PlatformClient, hostname string, timeout time.Duration) (string, error)`.
- Use the Grail/DQL stack (`POST <apps-url>/platform/storage/query/v1/query:execute`) — no new auth scheme.
- Timeout is a warning, not a failure; missing platform token is a skip, not an error.

**Non-Goals:**

- Replacing `WatchIngest` — that function monitors ongoing data flow. This function confirms host inventory registration specifically.
- Exposing a configurable timeout flag in this change — 2 minutes is the default; a flag can be added if field data shows it's insufficient.

## Decisions

### 1. DQL over classic `/api/v1/entity/infrastructure/hosts`

The classic Hosts API would require `Api-Token` auth and introduce a second URL family in the post-install path. The Platform DQL path reuses the platform token already validated by `validateCredentials`, stays consistent with `WatchIngest`, and avoids a parallel HTTP convention.

DQL query:

```dql
smartscapeNodes HOST, from:now()-1h
  | filter lower(name) == lower("<hostname>")
  | fields id, name
  | limit 1
```

Case-insensitive match (`lower(name)`) handles Windows hosts where OS-reported casing may differ from Dynatrace's recorded casing.

### 2. Request body mirrors `executeDQL` in `ingest_watch.go`

```json
{
  "query": "<dql>",
  "requestTimeoutMilliseconds": 10000,
  "maxResultRecords": 1
}
```

This avoids introducing a parallel HTTP convention.

### 3. Polling cadence and timeout semantics

Poll every 5 seconds (matching `watchPollInterval` in `ingest_watch.go`). On the first non-empty `result.records` array, extract the `id` field (e.g. `HOST-abc123`) and return it. On timeout: print `⚠ Host registration verification timed out after 2 minutes. Check the tenant UI.` and return `("", nil)` — the install is still considered successful.

Transient 5xx or network errors are logged at Debug and polling continues. Persistent 4xx (e.g. 401 Unauthorized) aborts polling and returns an error.

### 4. No platform token: skip with warning

If `p == nil` (no `--platform-token` / `DT_PLATFORM_TOKEN` configured), `WaitForHostRegistration` prints `⚠ Platform token not set — skipping host registration verification.` and returns `("", nil)`. This mirrors `WatchIngest`'s early-return at `ingest_watch.go:64-67`.

### 5. Hostname source

`os.Hostname()` provides the local hostname. It is passed as a parameter (not called inside the polling loop) so tests can inject a fixed value without OS dependency.

### 6. Logging

| Event | Level | Message | Keys |
|---|---|---|---|
| Each poll | Debug | `"polling smartscape for host"` | `hostname`, `attempt`, `elapsed` |
| Transient error | Debug | `"smartscape poll error"` | `status` or `error` |
| Success | Verbose | `"host registered"` | `host_id`, `hostname`, `elapsed` |
| Timeout | Warn | `"host registration timed out"` | `timeout`, `hostname` |

User-facing stdout: `Waiting for agent registration... (polling)` printed once at start; `✓ Host '<hostname>' registered with ID '<entityId>'.` on success.
