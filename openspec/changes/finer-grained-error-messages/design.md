# Context

`cmd/auth.go` contains two token validation functions: `checkPlatformToken` (validates via DQL) and `checkAccessToken` (validates via `POST /api/v2/apiTokens/lookup`). Both previously mapped both 401 and 403 to the same error string, "authentication failed". The code change splitting these into distinct messages has already landed; what remains is adding the spec scenario and the unit tests that pin the behavior.

## Goals / Non-Goals

**Goals:**

- Add a 403 scenario to the `platform-token-primary-auth` spec covering the platform-token "insufficient permissions" message
- Add a requirement to the `platform-token-primary-auth` spec covering the access-token 401/403 messages surfaced by `dtwiz status`
- Add direct unit tests for `checkPlatformToken` and `checkAccessToken` that assert the exact error string for 200, 401, 403, and unexpected non-2xx responses

**Non-Goals:**

- Changing any runtime behavior (the code fix is already in place)
- Updating the archived copies of the spec under `openspec/changes/archive/`
- Adding tests for `checkPlatformTokenClassicAccess` (already covered by `TestCheckClassicAccess`)

## Decisions

**Test approach: `httptest.NewServer` table-driven tests**

Both functions accept a URL and token string and use the package-level `credentialHTTPClient`. This is the same pattern used in `TestValidateCredentials` and `TestCheckClassicAccess` — swap `credentialHTTPClient` with the test server's client, return the desired status code, assert the error string. No new patterns needed.

**Scope: both `checkPlatformToken` and `checkAccessToken` in the spec update**

The `platform-token-primary-auth` spec covers DQL validation (`checkPlatformToken`), modified to add the 403 scenario. The access-token path (`checkAccessToken`) is a separate, optional flow surfaced by `dtwiz status`; it now gets its own requirement documenting the 401/403 message split. Tests cover both functions.

## Risks / Trade-offs

- [Archived spec copies are stale] → Acceptable: archives are historical snapshots and are not applied during reconciliation.
