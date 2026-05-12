# Why

The existing `InstallOneAgent` flow treats the installer subprocess's exit code as the install's final verdict: exit 0 = success. There is no confirmation that the agent actually registered with the tenant. A user can `dtwiz install oneagent`, see "success", and only discover minutes later (in the UI) that the agent is absent.

This change adds post-install verification: `WaitForHostRegistration` polls Grail via the Platform API (`POST <apps-url>/platform/storage/query/v1/query:execute`) with a DQL `smartscapeNodes HOST` query filtered on the local hostname. On success it prints the registered entity ID; on timeout it warns rather than fails.

## What Changes

- `WaitForHostRegistration(p *client.PlatformClient, hostname string, timeout time.Duration) (string, error)` added to `pkg/installer/oneagent_v2.go`.
- Uses the Platform API / Grail DQL stack already used by `WatchIngest` (`pkg/installer/ingest_watch.go`) — no new URL family or auth scheme.
- Timeout (2 minutes) is a warning, not a failure: the installer subprocess already exited 0, and host registration is eventually consistent.
- If no platform token is configured, the step is skipped with a clear warning — the install still reports success.

## Capabilities

### New Capabilities

- `oneagent-post-install-verification`: Polling Grail via DQL (`smartscapeNodes HOST`) for agent registration with a 2-minute timeout and warning-on-timeout semantics.

## Impact

- **Modified files:** `pkg/installer/oneagent_v2.go` (extend), `pkg/installer/oneagent_v2_test.go` (extend)
- **No new flags** — verification is automatic; the only opt-out is not supplying a platform token.
- **No breaking change** — the function is called after `ExecuteInstallCommand` returns 0; timeout returns `("", nil)` so the overall install exit code remains 0.
