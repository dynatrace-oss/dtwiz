# Proposal: Platform Token Primary Authentication

## Why

Dynatrace is deprecating access tokens. New tenants will no longer be able to create them. Platform tokens are the replacement and already work for all ingest endpoints. Classic API endpoints are in the process of being updated to accept platform tokens as well.

This PR is a transitional step: platform token becomes the primary required credential, while access token remains as an optional fallback for Classic API calls that do not yet accept platform tokens. Once all Classic API calls accept platform tokens and this is verified, access token support can be removed entirely.

## What Changes

- Platform token (`DT_PLATFORM_TOKEN`) becomes the only required credential
- Access token (`DT_ACCESS_TOKEN`) becomes optional — when set (legacy customers), it takes precedence for Classic API calls; when absent, the platform token is used in its place and `"  Using platform token"` is printed
- At startup, if an explicit access token is provided it is used directly for Classic API calls without probing; if absent, a lightweight probe determines whether the platform token is accepted by the Classic API
- `dtwiz status` shows the access token section only when `DT_ACCESS_TOKEN` is set

## Capabilities

### Modified Capabilities

- **Authentication resolution**: when access token is explicitly set it takes precedence for Classic API calls; otherwise platform token is used for both Platform and Classic APIs
- **Credential validation**: only platform token is validated at startup (via DQL); access token is not required
- **Status output**: access token section shown only when `DT_ACCESS_TOKEN` is set; no "fallback" label

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
