# OneAgent Validate Tasks

## 7. Post-install Verification

Confirm the agent has registered with the tenant. Timeout is a warning, not a failure.

**Files:** `pkg/installer/oneagent_v2.go` (extend — scaffolded in Task 1), `pkg/installer/oneagent_v2_test.go` (extend — scaffolded in Task 1)

- [ ] 7.1 Implement `WaitForHostRegistration(p *client.PlatformClient, hostname string, timeout time.Duration) (string, error)` polling Grail via `POST <p.BaseURL()>/platform/storage/query/v1/query:execute` every 5s. The DQL query is approximately:

  ```dql
  smartscapeNodes HOST, from:now()-1h
    | filter lower(name) == lower("<hostname>")
    | fields id, name
    | limit 1
  ```

  Request body shape (mirroring `executeDQL` in `pkg/installer/ingest_watch.go`):

  ```json
  {
    "query": "<dql>",
    "requestTimeoutMilliseconds": 10000,
    "maxResultRecords": 1
  }
  ```

  Auth: rely on the resty client's pre-configured Bearer header (no need to set `Authorization` manually).

- [ ] 7.1a If `p` is nil (no platform token configured), print `⚠ Platform token not set — skipping host registration verification.` and return `("", nil)` without issuing any HTTP request — mirrors `WatchIngest`'s early-return path in `ingest_watch.go:64-67`
- [ ] 7.1b Use `os.Hostname()` for the local hostname; pass it as a parameter for testability (don't call `os.Hostname()` inside the polling loop)
- [ ] 7.2 Parse the DQL response; on the first non-empty `result.records` array, extract the `id` field and return it as the `entityId`
- [ ] 7.3 On timeout, print `⚠ Host registration verification timed out after 2 minutes. Check the tenant UI.` and return `("", nil)` — the overall install remains successful
- [ ] 7.4 Emit `logger.Debug("polling smartscape for host", "hostname", hostname, "attempt", n, "elapsed", elapsed)` once per poll iteration
- [ ] 7.4a On transient poll errors, emit `logger.Debug("smartscape poll error", "status", status)` (or `"error", err` for network errors) and continue
- [ ] 7.4b On success, emit `logger.Verbose("host registered", "host_id", entityId, "hostname", hostname, "elapsed", elapsed)`
- [ ] 7.4c On timeout, emit `logger.Warn("host registration timed out", "timeout", timeout, "hostname", hostname)` in addition to the user-facing stdout warning
- [ ] 7.5 Unit tests with `httptest.Server`: host appears on 2nd poll (DQL returns one record with `id="HOST-abc123"`), host never appears (empty records → timeout warning), DQL endpoint returns 500 mid-poll (continues polling), nil PlatformClient (skip warning, no HTTP), persistent 401 (aborts polling), DQL request body contains the expected `smartscapeNodes HOST` query with the hostname filter. Assert Debug/Verbose/Warn lines are captured at the appropriate verbosity level
- [ ] 7.6 Wire the call in `InstallOneAgentV2`: `WaitForHostRegistration(c.Platform, hostname, 2*time.Minute)` after `ExecuteInstallCommand` returns 0; skip entirely when `opts.DryRun == true`
