# Proposal: Platform Token Primary Authentication

## Why

Dynatrace is deprecating access tokens. New tenants will no longer be able to create them. Platform tokens are the replacement and already work for all ingest endpoints. Classic API endpoints are in the process of being updated to accept platform tokens as well.

This PR is a transitional step: platform token becomes the primary required credential, while access token remains as an optional fallback for Classic API calls that do not yet accept platform tokens. Once all Classic API calls accept platform tokens and this is verified, access token support can be removed entirely.

## What Changes

- Platform token (`DT_PLATFORM_TOKEN`) becomes the only required credential
- Access token (`DT_ACCESS_TOKEN`) becomes optional — used as a fallback for Classic API calls only when the platform token is rejected (401/403)
- At startup, a lightweight probe determines which token to use for Classic API calls — no per-request retry logic needed
- `dtwiz status` labels the access token as "(fallback)" to communicate its optional role

## Capabilities

### Modified Capabilities

- **Authentication resolution**: platform token is tried first for Classic API; access token is fallback
- **Credential validation**: only platform token is validated at startup (via DQL); access token is not required
- **Status output**: access token shown as optional fallback, not a required credential

## Impact

- **Modified files**: `cmd/auth.go`, `cmd/root.go`, and all install/update/uninstall command files
- **Modified files**: `pkg/client/client.go` — two-token client construction restored
- **Modified files**: `test/integration/setup.go`, `test/e2e/otel_test.go` — `TEST_DT_PLATFORM_TOKEN` required, `TEST_DT_ACCESS_TOKEN` optional
- **New files**: `cmd/auth_test.go` — unit tests for the probe and fallback logic
- **Auth**: Classic API probe uses `GET /api/v2/settings/schemas`; DQL validation uses `Bearer` always

## Follow-up

Once all Classic API endpoints accept platform tokens, a follow-up change should:

1. Remove the Classic API probe and access token fallback logic
2. Remove `DT_ACCESS_TOKEN` / `--access-token` flag support entirely
3. Remove the `checkAccessToken` validation path from `dtwiz status`
