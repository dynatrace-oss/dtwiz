# Proposal: Access Token Flag-Only

## Why

The Classic API access token (`dt0c01.*`) is resolved from either the `--access-token` flag **or** the `DT_ACCESS_TOKEN` env var. A leftover `DT_ACCESS_TOKEN` exported in a user's shell therefore silently switches API calls off the platform token and onto an access token the user never intended to use for this invocation — with no way to tell an intentional token from a stale one. As Dynatrace deprecates access tokens, the safe default is platform-token auth, and access-token use should require an explicit, per-invocation signal.

## What Changes

- **BREAKING**: Access-token auth is now opt-in and **flag-only**. It activates only when `--access-token` is passed explicitly on the command line. `DT_ACCESS_TOKEN` is no longer read as an activation source.
- When `--access-token` is absent, the platform token is used for API calls (unchanged behavior for the no-access-token case).
- `accessToken()` returns only the `--access-token` flag value; the `os.Getenv("DT_ACCESS_TOKEN")` fallback is removed.
- `dtwiz status` shows the Access Token row only when `--access-token` is provided (it is no longer triggered by the env var).
- Help text, README, AGENTS.md, and the OneAgent download error hint no longer present `DT_ACCESS_TOKEN` as a way to enable access-token auth.
- **Out of scope (explicitly):** no access-token content validation is added to the setup/install path. The token remains trusted-on-use there, exactly as before.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `platform-token-primary-auth`: the "Access token is optional; used as Classic API fallback" requirement and the status-output requirement change — access-token auth is gated on the explicit `--access-token` flag and is no longer sourced from `DT_ACCESS_TOKEN`.

## Impact

- **Code**: `cmd/auth.go` (`accessToken()` drops env fallback), `cmd/root.go` (flag help + root `Long` text), `cmd/status.go` (Access Token row gating, already flag-driven via `accessToken()`), `pkg/installer/oneagent/download.go` (error hint).
- **Docs**: `README.md`, `AGENTS.md`, `CHANGELOG.md`.
- **Behavioral / compat**: users (e.g. CI pipelines) relying on `DT_ACCESS_TOKEN` alone will fall through to platform-token auth for Classic API calls. They must switch to `--access-token "$DT_ACCESS_TOKEN"`. Called out in CHANGELOG.
- **Tests**: add `cmd` unit tests covering `accessToken()` flag-only resolution and `getDtEnvironment()` / `validateCredentials()` behavior with and without the flag (env var present must not activate access-token auth).

## Rollback

Revert is low-risk and self-contained: restore the `os.Getenv("DT_ACCESS_TOKEN")` fallback in `accessToken()` and the prior help/doc strings. No persisted state or migrations are involved.
