# Design: OAuth Login

## Context

dtwiz reads credentials in `getDtEnvironment` (`cmd/auth.go`): the environment URL from `--environment` or `DT_ENVIRONMENT`, and a platform token from `--platform-token` or `DT_PLATFORM_TOKEN`. If either is missing, the command fails with a `ConfigError`.

dtwiz already depends on the dtctl SDK (`github.com/dynatrace-oss/dtctl/sdk` v0.45.0) for `auth`, `httpclient`, `urls`, `settings`, `extension` and `query`. The SDK's `session` package has what a login needs:

- `OAuthFlow.Start`: browser authorization-code flow with PKCE and a local callback server.
- `TokenManager`: stores tokens in the OS keyring (or a file, with `DTCTL_TOKEN_STORAGE=file`) and refreshes them.
- `Config`, `Load`, `CurrentContextObj`, `GetTokenForContext`: dtctl's contexts and token lookup.
- The OAuth client IDs `dt0s12.dtctl-prod`, `dt0s12.dtctl-dev` and `dt0s12.dtctl-sprint`, chosen from the environment URL.

The SDK leaves scope selection to the caller: `OAuthConfig.Scopes` is what the login flow requests.

## Goals / Non-Goals

**Goals:**

- A browser login that removes the need to create and export a platform token by hand.
- No change for users who set `DT_PLATFORM_TOKEN` or `--platform-token`.
- One session shared with dtctl, so users don't log in twice.
- Long-lived credentials for installed configs, created without the user pasting anything.

**Non-Goals:**

- Remote installs. They build on this later, in their own change.
- Headless login (device code flow). Headless and CI keep using environment variables.
- A dtwiz config file or a context switcher. dtctl already has `dtctl ctx`.
- Classic access tokens. The existing `--access-token` behaviour is unchanged.

## Decisions

### Reuse dtctl's config and OAuth client

dtwiz reads and writes dtctl's config (`session.DefaultConfigPath()`) and uses dtctl's OAuth clients through the SDK. A dtwiz login creates or updates a dtctl context for the environment, so `dtctl` sees it, and an existing `dtctl auth login` session works in dtwiz with no extra step.

dtctl's maintainers agreed to this on 11-17: an app that reuses dtctl's config, as `dtui` does, may reuse the OAuth client. A standalone app with its own config should register its own client. Reusing the config is also what lets one login cover both tools.

Alternative considered: a dtwiz-only config and a new OAuth client registered for dtwiz. Rejected because it needs a client registration and a second login, and duplicates dtctl's context handling.

### Scopes: dtwiz requests its own set on the shared client

The login requests the scopes dtwiz's installers need, which are already listed in the README (data acquisition, extensions, fleet management, OpenPipeline, RUM, settings, storage), plus `platform-token:tokens:write` for the token creation below and the scope `session` needs to read the user (`app-engine:apps:run`).

A client's allowed scopes are fixed when it is registered, and dtctl doesn't request the fleet-management, data-acquisition or RUM scopes itself. **Open question:** does the `dt0s12.dtctl-*` registration allow those scopes? Task 1.1 checks this with a real login before any other code is written. If it doesn't, the options are to ask dtctl's maintainers to extend the registration, or to register a dtwiz client, which per the decision above also means dtwiz keeps its own config. The answer goes back into this design before implementation continues.

When a context already exists from `dtctl auth login`, dtwiz checks the granted scopes the SDK caches (`TokenManager.CachedScopes`). If any are missing, dtwiz tells the user to run `dtwiz auth login` again instead of failing later with a 403.

### Credential order: flag, then env var, then OAuth session

`getDtEnvironment` keeps its current order and adds the session last:

1. `--platform-token`
2. `DT_PLATFORM_TOKEN`
3. With the flag on: the OAuth token for the environment, from dtctl's config. If no environment is given, dtctl's current context supplies it.

A token set by flag or env var always wins, so CI and existing scripts behave exactly as before. The OAuth token is a Bearer token, and `newCredentialClient` already picks the auth scheme from the token, so the downstream API calls don't change.

### Long-lived token for installed configs

An OAuth access token lives for minutes, and a refresh token needs the keyring on the user's machine. Neither can go into a collector config, a Kubernetes secret or a CloudFormation stack. When an install writes a token into such a config and the credential came from the OAuth session, dtwiz creates a platform token through `sdk/api/platformtoken`:

- scoped to that install method only
- named so it's recognisable in the UI, e.g. `dtwiz <method> <host> <date>`
- with the SDK's default expiry, which the preview shows

If the user lacks `platform-token:tokens:write`, dtwiz asks for a pasted token, as today. When the credential came from a flag or env var, dtwiz keeps writing that token, exactly as today.

### Feature flag `oauth-login`

Registered like the other flags: global on root, off by default, described as "enable dtwiz auth login and the OAuth credential source (experimental)". While it's off:

- `auth` is hidden from help, and running it returns the experimental-feature error naming `--oauth-login` / `DTWIZ_OAUTH_LOGIN`.
- `getDtEnvironment` never reads dtctl's config.
- `status` output is unchanged.

### Logout

`dtwiz auth logout` removes the OAuth tokens for the environment through the SDK, the same as `dtctl auth logout`. Platform tokens already created for installed configs are not revoked, because running collectors depend on them. The command prints that and points to the UI page where they can be revoked.

### Status

`status` adds a "Credentials" line: `--platform-token`, `DT_PLATFORM_TOKEN`, or `OAuth (dtctl context <name>, expires in …)`. It never prints the token.

## Risks / Trade-offs

- **Sandboxed shells can't read the macOS keychain.** This already happens with dtctl inside AI-agent sandboxes. The SDK's `DTCTL_TOKEN_STORAGE=file` works around it, and the error message points to it.
- **Coupling to dtctl's config format.** dtwiz only touches it through SDK functions, so the format stays dtctl's responsibility. The SDK version is pinned in `go.mod`.
- **Scope registration.** This is the open question above, and it could change the client decision. That's why it's the first task.
- **Created platform tokens accumulate.** Each install creates one. The name and expiry make them easy to find, and the preview says a token will be created.

## Migration Plan

None. With the flag off, nothing changes. GA is a later PR that deletes the flag.
