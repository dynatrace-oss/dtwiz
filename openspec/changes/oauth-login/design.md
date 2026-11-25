# Design: OAuth Login

## Context

dtwiz reads credentials in `getDtEnvironment` (`cmd/auth.go`): the environment URL from `--environment` or `DT_ENVIRONMENT`, and a platform token from `--platform-token` or `DT_PLATFORM_TOKEN`. If either is missing, the command fails with a `ConfigError`.

Most users get that token from the QuickStart app on their tenant. QuickStart creates a dedicated service user for the person, binds the **DtWiz Ingestion User** policy to it (provisioned by default on customer tenants), and creates a platform token on the service user's behalf with dtwiz's combined scope list. It stores one token per user and regenerates it when the scope list changes.

dtwiz already depends on the dtctl SDK (`github.com/dynatrace-oss/dtctl/sdk` v0.45.0) for `auth`, `httpclient`, `urls`, `settings`, `extension` and `query`. The SDK's `session` package has what a login needs:

- `OAuthFlow.Start`: browser authorization-code flow with PKCE and a local callback server.
- `TokenManager`: stores tokens in the OS keyring (or a file, with `DTCTL_TOKEN_STORAGE=file`) and refreshes them.
- `Config`, `Load`, `CurrentContextObj`, `GetTokenForContext`: dtctl's contexts and token lookup.
- The OAuth client IDs `dt0s12.dtctl-prod`, `dt0s12.dtctl-dev` and `dt0s12.dtctl-sprint`, chosen from the environment URL.

The SDK leaves scope selection to the caller: `OAuthConfig.Scopes` is what the login flow requests. The scopes a client may be granted are fixed when the client is registered.

## Goals / Non-Goals

**Goals:**

- A browser login that removes the need to create and export a platform token by hand.
- No change for users who set `DT_PLATFORM_TOKEN` or `--platform-token`.
- One session shared with dtctl, so users who have both tools don't log in twice.
- The same kind of token QuickStart gives users today, so installs, written configs and permissions behave the same.

**Non-Goals:**

- Remote installs. They build on this later, in their own change.
- Headless login (device code flow). Headless and CI keep using environment variables.
- A dtwiz config file or a context switcher. dtctl already has `dtctl ctx`.
- Minimal scopes per install method. dtwiz keeps its one combined scope list; splitting it is separate work.
- Classic access tokens. The existing `--access-token` behaviour is unchanged.

## Decisions

### Reuse dtctl's config and OAuth client, not the dtctl binary

dtwiz reads and writes dtctl's config file (`session.DefaultConfigPath()`) and uses dtctl's OAuth clients, all through the dtctl SDK it already imports. **dtwiz never calls the `dtctl` binary, and users don't need dtctl installed.** For a user without dtctl, `dtwiz auth login` simply creates the dtctl config file. For a user with dtctl, a dtwiz login creates or updates the dtctl context for the environment, so `dtctl` sees it, and an existing `dtctl auth login` session works in dtwiz with no extra step.

dtctl's maintainers agreed to this on 11-17: an app that reuses dtctl's config, as `dtui` does, may reuse the OAuth client. A standalone app with its own config should register its own client. Reusing the config is also what lets one login cover both tools.

Alternative considered: a dtwiz-only config and a new OAuth client registered for dtwiz. Rejected because it needs a client registration and a second login, and duplicates dtctl's context handling.

### The session provisions a token; it is not the credential

The OAuth access token is not used for dtwiz's API calls. After login, dtwiz uses it once to provision a dtwiz platform token, the same way QuickStart does:

1. Create a service user for the person (`iam:service-users:create`), or find the one dtwiz created earlier.
2. Bind the DtWiz Ingestion User policy to it (`iam:bindings:write`). A "binding already exists" answer counts as success, as in QuickStart.
3. Create a platform token on the service user's behalf (`platform-token:tokens:write`, `iam:service-users:use`) with dtwiz's combined scope list, the one in the README. That list is identical to QuickStart's `PLATFORM_TOKEN_SCOPES`.
4. Store the token in the OS keyring, keyed by environment, with the same file fallback as the SDK (`DTCTL_TOKEN_STORAGE=file`).

Later runs reuse the stored token, so there's one token per user and environment instead of one per install. dtwiz records a scope-list version next to it, like QuickStart's `CURRENT_PLATFORM_TOKEN_VERSION`. When dtwiz's scope list grows, it creates a new token, provided the session is still valid. If the session has expired, it asks the user to run `dtwiz auth login`.

This answers three review points at once:

- **Scopes.** The OAuth session only needs the four provisioning scopes, not the 46 scopes dtwiz's API calls need. The ingest scopes are on the platform token, and the policy governs them.
- **Installed configs.** The OTel collector config, Kubernetes secrets and CloudFormation parameters get a normal platform token, exactly as with a pasted one. No install-time token creation and no per-method scope split.
- **Permissions.** A service user backed by a dedicated policy is the model dtwiz users already have through QuickStart. The token keeps working after the person leaves, and it carries only what the policy allows, not the person's own permissions.

Alternative considered: a platform token owned by the logged-in user. It needs no new client scopes, because the dtctl client already has `platform-token:tokens:write`. Rejected because the token carries the person's own permissions and dies with their account, which is not how QuickStart tokens behave.

Alternative considered: the OAuth access token as the credential, with a platform token created per install for written configs (this proposal's first draft). Rejected because the dtctl client would need 30 more scopes, and per-install scoped tokens need a scope split dtwiz doesn't have.

### Scopes on dtctl's OAuth client

Checked against dtctl's client registrations on 2026-11-25. All three stages, `dt0s12.dtctl-prod`, `-dev` and `-sprint`, already allow `openid`, `offline_access`, `app-engine:apps:run`, `platform-token:tokens:write` and `iam:service-users:use`. They don't allow `iam:service-users:create` or `iam:bindings:write`.

The registrations also lack the 30 data-acquisition, fleet-management, OpenPipeline and RUM scopes. That's why the session can't be the credential, and why the design above needs only provisioning scopes.

So the change needs `iam:service-users:create` and `iam:bindings:write` added to the three dtctl clients, which dtctl's owners approve. Task 1.1 requests it before any dtwiz code is written. If dtctl's owners decline, dtwiz registers its own client and, per the decision above, keeps its own config.

When a context already exists from `dtctl auth login`, its cached scopes (`TokenManager.CachedScopes`) won't include the provisioning scopes. dtwiz then runs its own login for that context, requesting dtctl's scopes plus its own, so dtctl keeps working.

### Credential order: flag, then env var, then provisioned token

`getDtEnvironment` keeps its current order and adds the provisioned token last:

1. `--platform-token`
2. `DT_PLATFORM_TOKEN`
3. With the flag on: the dtwiz platform token stored for the environment. If no environment is given, dtctl's current context supplies it. If no token is stored but an OAuth session exists, dtwiz provisions one first.

A token set by flag or env var always wins, so CI and existing scripts behave exactly as before. The provisioned token is a platform token, so `newCredentialClient` and every downstream call work unchanged.

### Tenant API for provisioning

QuickStart calls the IAM endpoints through `@dynatrace-sdk/client-iam-reserved-v2` (`/platform-reserved/iam/v2/...`). The dtctl SDK's `api/platformtoken` uses the account API (`/iam/v1/accounts/{uuid}/platform-tokens`), which needs the account UUID and an account login. **Open question for task 1.2:** do the tenant endpoints QuickStart uses accept a token from a CLI OAuth client? If they don't, dtwiz falls back to the account API through `sdk/api/platformtoken`, whose create request already takes a `userUuid` for the service user. The answer goes into this design before task 3 starts.

### Feature flag `oauth-login`

Registered like the other flags: global on root, off by default, described as "enable dtwiz auth login and the OAuth credential source (experimental)". While it's off:

- `auth` is hidden from help, and running it returns the experimental-feature error naming `--oauth-login` / `DTWIZ_OAUTH_LOGIN`.
- `getDtEnvironment` never reads dtctl's config or the keyring.
- `status` output is unchanged.

### Logout

`dtwiz auth logout` removes the OAuth tokens for the environment through the SDK, the same as `dtctl auth logout`, and removes the stored dtwiz platform token from the keyring. It doesn't revoke that platform token or delete the service user, because installed collectors and secrets use the token. The command says so and points to the UI page where it can be revoked.

### Status

`status` adds a "Credentials" line: `--platform-token`, `DT_PLATFORM_TOKEN`, or `dtwiz login (service user <name>, token expires …)`. It never prints the token.

## Risks / Trade-offs

- **Needs a change to dtctl's clients.** Two more scopes. It's a small change, but it's in another team's files. It's the first task, so a "no" surfaces before any code.
- **Reserved tenant API.** QuickStart uses `platform-reserved` endpoints. If a CLI client can't call them, the account API fallback needs the account UUID, which is one more thing for the user to supply. That's task 1.2.
- **Provisioning needs IAM rights.** Creating service users and bindings requires permissions not every user has. QuickStart has the same requirement and checks first. dtwiz does the same, names the missing permissions, and falls back to asking for a pasted token.
- **Sandboxed shells can't read the macOS keychain.** This already happens with dtctl inside AI-agent sandboxes. The SDK's `DTCTL_TOKEN_STORAGE=file` works around it, and the error message points to it.
- **Coupling to dtctl's config format.** dtwiz only touches it through SDK functions, so the format stays dtctl's responsibility. The SDK version is pinned in `go.mod`.
- **Policy UUID per stage.** QuickStart hard-codes the DtWiz Ingestion User policy UUID for dev, sprint and prod. dtwiz needs the same three values, or a lookup by name.

## Migration Plan

None. With the flag off, nothing changes. GA is a later PR that deletes the flag.
