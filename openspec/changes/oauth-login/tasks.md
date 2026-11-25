# Tasks

## 1. Prerequisites

- [ ] 1.1 Ask dtctl's owners to add `iam:service-users:create` and `iam:bindings:write` to `dt0s12.dtctl-prod`, `-dev` and `-sprint`, and get it approved. If they decline, stop and update `design.md` (a dtwiz client with its own config) before continuing.
- [ ] 1.2 On a sprint tenant, with a dtctl-client OAuth token that has the provisioning scopes, create a service user, bind the DtWiz Ingestion User policy and create a platform token through the tenant endpoints QuickStart uses (`/platform-reserved/iam/v2/...`). If they refuse a CLI client, repeat it through the account API (`sdk/api/platformtoken`). Record the result in `design.md` before task 3.

## 2. Feature flag

- [ ] 2.1 Register `OAuthLogin` in `pkg/featureflags/featureflags.go` (`oauth-login`, `DTWIZ_OAUTH_LOGIN`, default `false`, description ending "(experimental)"). Verify with new cases in `pkg/featureflags/featureflags_test.go`: default off, env `true`/`1` on, and the flag appears in `List()`.

## 3. Login and logout

- [ ] 3.1 Add a self-contained package that wraps `session.OAuthFlow`, `TokenManager` and `Config` for login, session lookup and logout, requesting dtctl's scopes plus the provisioning scopes. Verify with unit tests against a fake token endpoint and a temp config dir (`HOME` isolated), including a run with no dtctl config file.
- [ ] 3.2 Add `dtwiz auth login` and `dtwiz auth logout`, hidden unless the flag is on, returning the experimental-feature error when it's off. Verify with `cmd` tests for help output and the flag-off error, with no network call.

## 4. Credential source

- [ ] 4.1 Add the provisioned dtwiz token as the third source in `getDtEnvironment`, only when the flag is on, including the environment from dtctl's current context and the missing-scope check. Verify with `cmd/auth_test.go` cases for each scenario in the spec's "Credential source order" requirement.

## 5. Status

- [ ] 5.1 Show the credential source in `dtwiz status` when the flag is on. Verify with a `cmd/status_test.go` case asserting the line and that no token text appears.

## 6. Service-user token

- [ ] 6.1 Provision the dtwiz token: create or find the service user, bind the DtWiz Ingestion User policy (treating "binding already exists" as success), create the platform token with the README scope list and a scope-list version, and store it in the keyring with the SDK's file fallback. Include the permission check and the pasted-token fallback. Verify with tests against a fake IAM API for each scenario in the spec's "Service-user platform token" requirement.

## 7. Integration check

- [ ] 7.1 Run `make test` and `make lint`. On a snapshot build with `DTWIZ_OAUTH_LOGIN=true`, run `dtwiz auth login`, `dtwiz status`, a dry-run install, then `dtwiz auth logout`, on macOS and Linux. Confirm `dtctl` sees the same session, the service user and token appear in the UI, and a second install reuses the token.
- [ ] 7.2 Document `auth`, the flag and the credential order in `README.md`, and add an `[Unreleased]` CHANGELOG entry.

## Workflow follow-up

- [ ] Archive the OpenSpec change in the last implementation PR.
