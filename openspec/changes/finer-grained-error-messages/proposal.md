# Why

When token validation fails, dtwiz previously returned the same error message for both 401 (bad/expired token) and 403 (valid token, missing scopes), making it impossible for users to distinguish a credential problem from a permissions problem. The fix has already been implemented; this change captures the spec and test coverage that were missing.

## What Changes

- `checkPlatformToken` now returns `✗ Platform token: insufficient permissions` on 403 (previously: `authentication failed`)
- `checkAccessToken` now returns `✗ Access token: insufficient permissions` on 403 (previously: `authentication failed`)
- Unit tests are added to assert the exact error message for each HTTP status code path
- The `platform-token-primary-auth` spec is updated to cover the 403 scenario

## Capabilities

### New Capabilities

- none

### Modified Capabilities

- `platform-token-primary-auth`:
  - Add a 403 scenario to the DQL validation requirement, specifying that `✗ Platform token: insufficient permissions` is the expected exit message when the token is valid but lacks the required DQL scope.
  - Add a requirement covering access-token validation in `dtwiz status`, specifying that 401 yields `✗ Access token: authentication failed` and 403 yields `✗ Access token: insufficient permissions`.

## Impact

- `cmd/auth.go`: already changed (401 and 403 produce distinct messages for both `checkPlatformToken` and `checkAccessToken`)
- `cmd/auth_test.go`: needs new table-driven tests for `checkPlatformToken` and `checkAccessToken` covering 200, 401, 403, and non-2xx status codes with exact message assertions
- `openspec/specs/platform-token-primary-auth/spec.md`: needs a second DQL validation scenario for the 403 case, plus a new requirement covering the access-token 401/403 messages surfaced by `dtwiz status`
