# Why

The v2 OneAgent installer flow uses the credentials already carried by `c.Classic` (the configured Authorization header) to authenticate the installer binary download. No external API call or explicit token extraction is needed — this keeps the install fast and removes any dependency on the `tokens.write` scope.

## What Changes

- `InstallOneAgentV2` accepts a `*client.Client` and passes `c.Classic` directly to `DownloadInstaller`; no explicit token extraction is required.
- Credentials are never surfaced in logs, stdout, or files at any verbosity level.

## Capabilities

### Modified Capabilities

- `oneagent-client-injection`: `InstallOneAgentV2` accepts a `*client.Client` (carrying both Classic and Platform halves) and uses `c.Classic` to authenticate the installer download.

## Impact

- **Modified files:** `pkg/installer/oneagent_v2.go` (extend), `pkg/installer/oneagent_v2_test.go` (extend)
- **No breaking changes** to existing callers during development — the new flow is behind `ONEAGENT_POC`.
- **No new scope requirements:** unlike token minting, this requires no `tokens.write` permission.
