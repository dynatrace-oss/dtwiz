# Why

The v2 OneAgent installer flow needs to explicitly select which credential to use for the installer binary download. Rather than making an external API call to mint a scoped token, the installer token is resolved directly from the credentials the user already supplied — access token preferred, platform token as fallback. This keeps the install fast, removes any dependency on the `tokens.write` scope, and avoids an extra network round-trip.

## What Changes

- `ResolveInstallerToken(c *client.Client) (string, error)` added to `pkg/installer/oneagent_v2.go`: returns the access token if set, otherwise the platform token; returns an error if neither is available.
- `InstallOneAgentV2` calls `ResolveInstallerToken` before `DownloadInstaller` and aborts on error.
- The resolved token value is held only in memory and never appears in logs, stdout, or files at any verbosity level. Only the credential source (`access` or `platform`) is logged at Debug.

## Capabilities

### New Capabilities

- `oneagent-installer-token`: Credential resolution for the installer download; access token preferred, platform token as fallback; aborts with a clear error if neither is set.

### Modified Capabilities

- `oneagent-client-injection`: `InstallOneAgentV2` accepts a `*client.Client` (carrying both Classic and Platform halves) and passes the resolved token to `DownloadInstaller`.

## Impact

- **Modified files:** `pkg/installer/oneagent_v2.go` (extend — `ResolveInstallerToken` added), `pkg/installer/oneagent_v2_test.go` (extend)
- **No breaking changes** to existing callers during development — the new flow is behind `ONEAGENT_POC`.
- **No new scope requirements:** unlike token minting, this requires no `tokens.write` permission.
