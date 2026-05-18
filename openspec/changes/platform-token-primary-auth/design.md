# Design: Platform Token Primary Authentication

## Context

Dynatrace is migrating from access tokens (`dt0c01.*`) to platform tokens (`dt0s16.*`). New tenants can no longer create access tokens. Platform tokens already work for all ingest endpoints, and Classic API endpoints are being updated to accept platform tokens as well.

The two token families:

| Token | Prefix | Auth header | APIs |
|---|---|---|---|
| Platform token | `dt0s16.*` | `Bearer` | Platform API (DQL/Grail), ingest endpoints |
| Access token | `dt0c01.*` | `Api-Token` | Classic API (being phased out) |

This change makes platform token the primary credential. Access token stays as a fallback for Classic API calls that don't yet accept platform tokens.

## Goals / Non-Goals

**Goals:**

- dtwiz works with platform token only — no access token needed
- Access token remains as a silent fallback for Classic API
- Clear debug logging when fallback is applied
- Probe-based fallback resolved once at startup — no per-request retry

**Non-Goals:**

- Removing access token support entirely — that is a follow-up
- Changing which URLs are used for Classic vs Platform API

## Decisions

### 1. Startup probe for Classic API token selection

At credential validation time (`validateCredentials`), a `GET /api/v2/settings/schemas` request is made with the platform token. If this returns 401 or 403, the access token is used instead for Classic API calls. Any other response means authentication was accepted.

**Why not per-request retry**: requires thread-safe token storage and retry logic in the HTTP client — significant complexity for a gap that will close once all Classic API endpoints accept platform tokens.

### 2. `GET /api/v2/settings/schemas` as the probe endpoint

Read-only metadata endpoint, no data-access scope requirements. Any non-401/403 response confirms authentication regardless of the token's data permissions.

**Risk**: if this endpoint requires `settings.read` scope and the token lacks it, the probe returns 403 for the wrong reason (scope, not auth), causing an unnecessary fallback. Acceptable trade-off.

### 3. DQL validation is a hard requirement

`checkPlatformToken` DQL call always runs and failure is always a hard error. No fallback — if the platform token can't authenticate Platform API, dtwiz cannot function.

### 4. `getDtEnvironment` returns raw tokens; `validateCredentials` returns resolved `classicTok`

`getDtEnvironment` resolves tokens from flags/env vars without HTTP calls. `validateCredentials` does the probe and returns `(classicTok string, err error)`. Commands use `classicTok` for Classic API installers and `platformTok` for DQL/Platform API.

### 5. `setupClientFromCreds` avoids double-probing

Commands needing both resolved tokens and a `*client.Client` call `setupClientFromCreds(envURL, classicTok, platformTok)` directly after `validateCredentials`, skipping a second call to `getDtEnvironment` inside `setupClient`.

## Risks / Trade-offs

- **[Probe latency]** → Extra HTTP round-trip at startup. Acceptable for a startup credential check.
- **[False fallback on scope 403]** → Access token used unnecessarily if probe endpoint returns 403 due to missing scope rather than auth failure. Low risk in practice.
- **[Incomplete Classic API coverage]** → Only authentication is probed, not specific endpoint permissions. Some operations may still fail mid-install if the platform token lacks the required Classic API scopes. Users can set `DT_ACCESS_TOKEN` as a workaround in the meantime.
