# Proposal: OAuth Login

## Why

Every dtwiz command that talks to the tenant needs a platform token today. The user has to create one by hand in the Dynatrace UI, with the right scopes, and export it as `DT_PLATFORM_TOKEN`. That is the slowest step of onboarding, and it leaves a long-lived secret in shell history and dotfiles. dtctl already solves this with `dtctl auth login`: a browser OAuth login with tokens kept in the OS keyring and refreshed automatically. That code lives in the dtctl SDK's `sdk/session` package, which dtwiz already depends on.

A login is also the base for later work: remote installs need dtwiz to get tokens from the tenant itself, instead of copying a long-lived token onto every host.

## What Changes

- Add `dtwiz auth login` and `dtwiz auth logout`, built on `sdk/session`. Login opens the browser, runs the OAuth authorization-code flow with PKCE, and stores the tokens the way dtctl does.
- Reuse dtctl's config and OAuth client, as `dtui` does. A user logged in with dtwiz is logged in to dtctl for the same environment, and the other way round. dtwiz adds no config file of its own.
- Add the OAuth session as a third credential source. The order is `--platform-token`, then `DT_PLATFORM_TOKEN`, then the OAuth session. Users who set a token today see no change.
- For configs that outlive the session (the OTel collector config, Kubernetes secrets, CloudFormation parameters), create a platform token at install time through the SDK's `api/platformtoken`, scoped to what that install method needs. If the user can't create platform tokens, ask for a pasted token, as today.
- Show the credential source in `dtwiz status`.
- Gate everything behind a new feature flag, `oauth-login` (`--oauth-login` / `DTWIZ_OAUTH_LOGIN`), off by default. While it is off, `auth` is hidden, the OAuth source is never consulted, and `status` is unchanged.

## Capabilities

### New Capabilities

- `oauth-login`: Log in to a Dynatrace environment through the browser, use the session as a credential source, and create long-lived platform tokens for installed configs, behind a feature flag.

### Modified Capabilities

## Impact

- `cmd/auth.go`: `getDtEnvironment` gains the OAuth session as a third source, only when the flag is on.
- `cmd/`: new `auth login` and `auth logout` commands, hidden until the flag is on.
- `cmd/status.go`: shows which credential source is in use.
- `pkg/featureflags/featureflags.go`: registers `OAuthLogin`.
- A new self-contained package for the login, session lookup and platform-token creation, on top of `sdk/session` and `sdk/api/platformtoken`.
- dtctl's config file and keyring entries are written by `auth login` and removed by `auth logout`, using the SDK's own functions, so the format stays dtctl's.
- Self-monitoring records the credential source type only, never a token, user or environment URL.
- Rollback: turn the flag off. Sessions stay in dtctl's config, where `dtctl auth logout` removes them.
