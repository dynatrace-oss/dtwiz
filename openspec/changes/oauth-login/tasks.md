# Tasks

## 1. Scope check

- [ ] 1.1 Run a real `OAuthFlow.Start` against a test tenant with dtctl's prod client and the full dtwiz scope set from the README, plus `platform-token:tokens:write`. Record which scopes are granted. If any are refused, stop and update `design.md` (extend dtctl's registration, or a dtwiz client with its own config) before continuing.

## 2. Feature flag

- [ ] 2.1 Register `OAuthLogin` in `pkg/featureflags/featureflags.go` (`oauth-login`, `DTWIZ_OAUTH_LOGIN`, default `false`, description ending "(experimental)"). Verify with new cases in `pkg/featureflags/featureflags_test.go`: default off, env `true`/`1` on, and the flag appears in `List()`.

## 3. Login and logout

- [ ] 3.1 Add a self-contained package that composes the dtwiz scope set and wraps `session.OAuthFlow`, `TokenManager` and `Config` for login, session lookup and logout. Verify with unit tests against a fake token endpoint and a temp config dir (`HOME` isolated).
- [ ] 3.2 Add `dtwiz auth login` and `dtwiz auth logout`, hidden unless the flag is on, returning the experimental-feature error when it's off. Verify with `cmd` tests for help output and the flag-off error, with no network call.

## 4. Credential source

- [ ] 4.1 Add the OAuth session as the third source in `getDtEnvironment`, only when the flag is on, including the environment from dtctl's current context and the missing-scope check. Verify with `cmd/auth_test.go` cases for each scenario in the spec's "Credential source order" requirement.

## 5. Status

- [ ] 5.1 Show the credential source in `dtwiz status` when the flag is on. Verify with a `cmd/status_test.go` case asserting the line and that no token text appears.

## 6. Long-lived token

- [ ] 6.1 Create a scoped platform token through `sdk/api/platformtoken` when an install writes a token into a config and the credential came from the session, with the pasted-token fallback. Show it in the preview. Verify with tests against a fake token API.

## 7. Integration check

- [ ] 7.1 Run `make test` and `make lint`. On a snapshot build with `DTWIZ_OAUTH_LOGIN=true`, run `dtwiz auth login`, `dtwiz status`, a dry-run install, then `dtwiz auth logout`, on macOS and Linux. Confirm `dtctl` sees the same session.
- [ ] 7.2 Document `auth`, the flag and the credential order in `README.md`, and add an `[Unreleased]` CHANGELOG entry.

## Workflow follow-up

- [ ] Archive the OpenSpec change in the last implementation PR.
