# Why

The existing `dtwiz install oneagent` flow reuses the caller-supplied `--access-token` directly to download the installer binary. Passing a long-lived, broadly-scoped API token to an external binary is against Dynatrace's recommended installer practice and unnecessary: the token only needs `InstallerDownload` scope for the duration of a single install.

This change introduces mandatory short-lived token minting for the OneAgent installer flow. A scoped token is minted via `POST /api/v2/tokens`, used exclusively for the installer download, and then discarded. There is no fallback to the user's long-lived token.

## What Changes

- `MintInstallerToken(c *client.ClassicClient) (string, error)` added to `pkg/installer/oneagent_v2.go`: posts `{name: "dtwiz-oneagent-installer", scopes: ["InstallerDownload"], expiresIn: {value: 1, unit: "HOURS"}}` to `/api/v2/tokens`.
- On any non-2xx or network error, `InstallOneAgentV2` aborts — no fallback.
- The minted token value is held only in memory and never appears in logs, stdout, or files at any verbosity level.
- `logger.Debug` lines record the request URL/scopes and the response HTTP status only; the response body (which contains the token) is never logged on success.

## Capabilities

### New Capabilities

- `oneagent-installer-token`: Mandatory minting of a short-lived `InstallerDownload`-scoped token; no fallback to the user-supplied token.

### Modified Capabilities

- `oneagent-client-injection`: `InstallOneAgentV2` accepts a `*client.Client` (carrying both Classic and Platform halves) and constructs the minted-token request against the Classic client.

## Impact

- **Modified files:** `pkg/installer/oneagent_v2.go` (extend — `MintInstallerToken` added), `pkg/installer/oneagent_v2_test.go` (extend)
- **No breaking changes** to existing callers during development — the new flow is behind `ONEAGENT_POC`.
- **Security improvement:** the user's long-lived `--access-token` is no longer passed to or accessible by the installer subprocess.
