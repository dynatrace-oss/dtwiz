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

### 1. Access token takes precedence when set; platform token fills in when absent

At credential validation time (`validateCredentials`), if an explicit access token is set (different from the platform token), it is returned directly as `classicTok` for Classic API calls — no probe needed. When no explicit access token is configured, `getDtEnvironment` sets `accessTok = platformTok` and prints `"  Using platform token"`. `validateCredentials` then probes `GET /api/v2/settings/schemas` with the platform token; regardless of whether the probe passes or fails, the platform token is used for Classic API calls.

**Why explicit access token wins without probing**: legacy customers who set `DT_ACCESS_TOKEN` know they need it — there is no value in probing the platform token first and potentially falling through to the access token anyway.

**Why platform token is used even when the probe fails**: the probe failure may be transient or due to scope rather than auth. Using the platform token as best-effort is better than failing hard when no access token is available.

### 2. `GET /api/v2/settings/schemas` as the probe endpoint (platform-token-only path)

When no explicit access token is set, a read-only metadata probe is made with the platform token. Any non-401/403 response confirms Classic API authentication. Used only when `accessTok == platformTok` (i.e. no explicit access token was configured).

### 3. DQL validation is a hard requirement

`checkPlatformToken` DQL call always runs and failure is always a hard error. No fallback — if the platform token can't authenticate Platform API, dtwiz cannot function.

### 4. `getDtEnvironment` returns raw tokens; `validateCredentials` returns resolved `classicTok`

`getDtEnvironment` resolves tokens from flags/env vars without HTTP calls. `validateCredentials` does the probe and returns `(classicTok string, err error)`. Commands use `classicTok` for Classic API installers and `platformTok` for DQL/Platform API.

### 5. `setupClientFromCreds` avoids double-probing

Commands needing both resolved tokens and a `*client.Client` call `setupClientFromCreds(envURL, classicTok, platformTok)` directly after `validateCredentials`, skipping a second call to `getDtEnvironment` inside `setupClient`.

## Risks / Trade-offs

- **[Probe latency]** → Extra HTTP round-trip at startup when no explicit access token is set. Acceptable for a startup credential check.
- **[Incomplete Classic API coverage]** → Only authentication is probed, not specific endpoint permissions. Some operations may still fail mid-install if the platform token lacks the required Classic API scopes. Users can set `DT_ACCESS_TOKEN` as a workaround in the meantime.
