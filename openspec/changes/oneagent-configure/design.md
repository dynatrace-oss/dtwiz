# Design

## Context

The existing `InstallOneAgent` flow in `pkg/installer/oneagent.go` passes the user's `--access-token` directly as the `Authorization` header for the installer download request. This token is long-lived and may carry scopes well beyond `InstallerDownload`. Dynatrace's documented installer flow recommends minting a short-lived, narrowly-scoped token for this purpose.

The `pkg/featureflags` package already supports adding new flags; `pkg/client` provides the resty-based Classic client. No new infrastructure is needed.

## Goals / Non-Goals

**Goals:**

- Implement `MintInstallerToken(c *client.ClassicClient) (string, error)` in `pkg/installer/oneagent_v2.go`.
- Enforce no-fallback semantics: any mint failure (network, 4xx, 5xx) aborts the install.
- Enforce token confidentiality: the token value must never appear in any log line at any level.

**Non-Goals:**

- Persisting or caching the minted token beyond the install operation.
- Revoking the minted token after the install completes (it is 1h-scoped and will expire naturally).

## Decisions

### 1. Mint endpoint and payload

```
POST /api/v2/tokens
Authorization: Api-Token <user-access-token>
Body: {
  "name": "dtwiz-oneagent-installer",
  "scopes": ["InstallerDownload"],
  "expiresIn": { "value": 1, "unit": "HOURS" }
}
```

The user's access token is used to authenticate the mint request itself. The response's `token` field is extracted and returned. The response body is never logged on success (because it contains the token); on non-2xx, the body is safe to log (it describes the error, not a token).

### 2. No fallback

If minting returns any non-2xx status or a network error, `MintInstallerToken` returns a wrapped error and `InstallOneAgentV2` propagates it immediately. The subsequent download stage (`DownloadInstaller`) is not called. The error message includes the HTTP status and response body so the user can see which scope is missing (e.g. `tokens.write`).

### 3. Token confidentiality in logs

Logging follows the project-wide `pkg/logger` convention:

| Event | Level | Message | Keys |
|---|---|---|---|
| Before request | Debug | `"minting installer token"` | `url`, `scopes`, `expires_in` |
| On 2xx | Debug | `"installer token minted"` | `status` (HTTP code only — never the response body) |
| On non-2xx | Debug | `"installer token mint failed"` | `status`, `body` |

The minted token value (`response.token`) is held only in memory. A dedicated unit test captures stderr with `--debug` enabled and asserts the token value never appears in the output.

### 4. Scope name casing

The Dynatrace API accepts `"InstallerDownload"` (PascalCase). This matches the documented scope identifier. If the API rejects it with a scope-not-found error, the error body is surfaced verbatim in the failure message for easy diagnosis.
