# Proposal: OAuth Login

## Why

Every dtwiz command that talks to the tenant needs a platform token today. The user has to create one by hand in the Dynatrace UI, with the right scopes, and export it as `DT_PLATFORM_TOKEN`. That is the slowest step of onboarding, and it leaves a long-lived secret in shell history and dotfiles. dtctl already solves this with `dtctl auth login`: a browser OAuth login with tokens kept in the OS keyring and refreshed automatically. That code lives in the dtctl SDK's `sdk/session` package, which dtwiz already depends on.

A login is also the base for later work: remote installs need dtwiz to get tokens from the tenant itself, instead of copying a long-lived token onto every host.

## What Changes

- Add `dtwiz auth login` and `dtwiz auth logout`, built on `sdk/session`. Login opens the browser, runs the OAuth authorization-code flow with PKCE, and stores the tokens the way dtctl does.
- Reuse dtctl's config file and OAuth client, as `dtui` does. dtwiz does this through the dtctl SDK, which it already depends on. **The `dtctl` binary is not needed.** Users who never installed dtctl just get a dtctl config file written by dtwiz. Users who have dtctl share the login: logged in with one tool means logged in with the other, for the same environment. dtwiz adds no config file of its own.
- Use the login to provision a dtwiz platform token the same way the QuickStart app does: a dedicated service user, bound to the existing **DtWiz Ingestion User** policy, with a platform token created on its behalf. The token carries dtwiz's existing combined scope list (the README's list, which is also QuickStart's), so no per-install-method scope split is needed. The token is created once per user and environment, stored in the OS keyring and reused.
- Add that provisioned token as a third credential source. The order is `--platform-token`, then `DT_PLATFORM_TOKEN`, then the provisioned token. It is a normal platform token, so API calls and written configs (the OTel collector config, Kubernetes secrets, CloudFormation parameters) work exactly as they do with a pasted token. Users who set a token today see no change.
- Show the credential source in `dtwiz status`.
- Gate everything behind a new feature flag, `oauth-login` (`--oauth-login` / `DTWIZ_OAUTH_LOGIN`), off by default. While it is off, `auth` is hidden, the OAuth source is never consulted, and `status` is unchanged.

## Capabilities

### New Capabilities

- `oauth-login`: Log in to a Dynatrace environment through the browser, provision a dtwiz platform token on a service user, and use it as a credential source, behind a feature flag.

### Modified Capabilities

## Impact

- `cmd/auth.go`: `getDtEnvironment` gains the provisioned dtwiz token as a third source, only when the flag is on.
- `cmd/`: new `auth login` and `auth logout` commands, hidden until the flag is on.
- `cmd/status.go`: shows which credential source is in use.
- `pkg/featureflags/featureflags.go`: registers `OAuthLogin`.
- A new self-contained package for the login, service-user provisioning and token lookup, on top of `sdk/session`.
- dtctl's config file and keyring entries are written by `auth login` and removed by `auth logout`, using the SDK's own functions, so the format stays dtctl's.
- dtctl's OAuth clients (`dt0s12.dtctl-*`) need two more scopes, `iam:service-users:create` and `iam:bindings:write`. This is a small registration change per stage that dtctl's owners have to approve.
- Self-monitoring records the credential source type only, never a token, user or environment URL.
- Rollback: turn the flag off. Sessions stay in dtctl's config, where `dtctl auth logout` removes them.
