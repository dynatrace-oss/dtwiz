# Design

## Context

The existing `InstallOneAgent` flow in `pkg/installer/oneagent.go` passes the user's `--access-token` directly as the `Authorization` header for the installer download request. The v2 flow needs to explicitly resolve which credential to use for the download without requiring any additional API call or external dependency.

The `pkg/client` package already carries both the Classic (`--access-token`) and Platform (`--platform-token`) credentials. No new infrastructure is needed.

## Goals / Non-Goals

**Goals:**

- Implement `ResolveInstallerToken(c *client.Client) (string, error)` in `pkg/installer/oneagent_v2.go`.
- Prefer the access token; fall back to the platform token if the access token is not set.
- Abort the install with a clear error if neither credential is available.
- Enforce token confidentiality: the token value must never appear in any log line at any level.

**Non-Goals:**

- Minting or exchanging tokens via any external API.
- Caching or persisting the resolved token beyond the install operation.

## Decisions

### 1. Token resolution logic

`InstallOptions` carries a `Token string` field — the already-resolved `classicTok` from `validateCredentials` — set in `cmd/install.go` before calling `InstallOneAgentV2`. This matches the pattern used by all other installers (e.g. `InstallOtelCollectorWithProject(envURL, classicTok, platformTok, ...)`): the token is resolved once at the command layer and passed down explicitly.

`ResolveInstallerToken(opts InstallOptions) (string, error)` then applies a simple check:

1. If `opts.Token` is empty → error: `"no credentials available: supply --access-token or --platform-token"`.
2. Else infer source from token prefix using `AuthHeader()`: `"Api-Token"` scheme → `source = "access"`, `"Bearer"` scheme → `source = "platform"`.

No network call is made.

### 2. Token confidentiality in logs

Logging follows the project-wide `pkg/logger` convention:

| Event | Level | Message | Keys |
|---|---|---|---|
| Token resolved | Debug | `"resolved installer token"` | `source` (`"access"` or `"platform"`) |

The token value itself is never logged. A dedicated unit test captures stderr with `--debug` enabled and asserts the token value never appears in the output.

### 3. Token scope

The resolved token string is returned to `InstallOneAgentV2` for use in the download step (implemented in a subsequent task). All other requests use the original `*client.Client` credentials unchanged.
