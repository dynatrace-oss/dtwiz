# Spec Delta

## Purpose

Define how dtwiz logs in to a Dynatrace environment through the browser, uses that session as a credential source, and creates long-lived platform tokens for installed configs.

## ADDED Requirements

### Requirement: Browser login shared with dtctl

When the `oauth-login` feature flag is enabled, `dtwiz auth login` SHALL run the OAuth authorization-code flow with PKCE for the given environment, using dtctl's OAuth client for that environment, and SHALL store the session in dtctl's config and token storage through the dtctl SDK.

#### Scenario: Login creates a dtctl context

- **GIVEN** the `oauth-login` feature flag is enabled
- **WHEN** the user runs `dtwiz auth login --environment https://abc12345.apps.dynatrace.com` and completes the browser login
- **THEN** dtctl's config has a context for that environment with an OAuth token
- **AND** `dtctl` commands for that environment work without another login

#### Scenario: Existing dtctl session is reused

- **GIVEN** the `oauth-login` feature flag is enabled and the user ran `dtctl auth login` for the environment, and its granted scopes include those dtwiz needs
- **WHEN** the user runs a dtwiz command for that environment without a platform token
- **THEN** dtwiz uses the existing session and doesn't open the browser

#### Scenario: Existing session lacks scopes

- **GIVEN** the `oauth-login` feature flag is enabled and the cached session for the environment lacks a scope dtwiz needs
- **WHEN** the user runs a dtwiz command that needs the tenant
- **THEN** the command fails before any API call, naming the missing scopes and telling the user to run `dtwiz auth login`

### Requirement: Credential source order

dtwiz SHALL resolve the platform credential in this order: `--platform-token`, then `DT_PLATFORM_TOKEN`, then, only when the `oauth-login` feature flag is enabled, the OAuth session for the environment.

#### Scenario: Env var wins over a session

- **GIVEN** the `oauth-login` feature flag is enabled, `DT_PLATFORM_TOKEN` is set, and an OAuth session exists
- **WHEN** the user runs a dtwiz command
- **THEN** dtwiz uses `DT_PLATFORM_TOKEN`

#### Scenario: Flag off ignores sessions

- **GIVEN** the `oauth-login` feature flag is disabled, no platform token is set, and an OAuth session exists
- **WHEN** the user runs a dtwiz command that needs the tenant
- **THEN** dtwiz fails with the existing "no Dynatrace platform token configured" error
- **AND** dtctl's config is not read

#### Scenario: Environment from dtctl's current context

- **GIVEN** the `oauth-login` feature flag is enabled, neither `--environment` nor `DT_ENVIRONMENT` is set, and dtctl's current context has an OAuth session
- **WHEN** the user runs a dtwiz command
- **THEN** dtwiz uses that context's environment and session

### Requirement: Long-lived token for installed configs

When an install writes a token into a config that outlives the session and the credential came from the OAuth session, dtwiz SHALL create a platform token scoped to that install method and write that token, not the OAuth token.

#### Scenario: Install creates a platform token

- **GIVEN** the `oauth-login` feature flag is enabled and the credential came from the OAuth session
- **WHEN** the user runs an install that writes a token into a config file
- **THEN** the preview says a platform token will be created, with its name, scopes and expiry
- **AND** the written config contains the created platform token

#### Scenario: Token creation not allowed

- **GIVEN** the session lacks `platform-token:tokens:write`
- **WHEN** the install needs a long-lived token
- **THEN** dtwiz asks the user for a platform token, as it does today

#### Scenario: Pasted token is kept

- **GIVEN** the credential came from `DT_PLATFORM_TOKEN`
- **WHEN** the user runs an install
- **THEN** dtwiz writes that token, as it does today, and creates no platform token

### Requirement: Logout

When the `oauth-login` feature flag is enabled, `dtwiz auth logout` SHALL remove the OAuth tokens for the environment and SHALL NOT revoke platform tokens created for installed configs.

#### Scenario: Logout keeps installed tokens

- **GIVEN** the user is logged in and an earlier install created a platform token
- **WHEN** the user runs `dtwiz auth logout`
- **THEN** the OAuth tokens for the environment are removed
- **AND** the created platform token still works
- **AND** the output says where to revoke created tokens

### Requirement: Feature flag gating

The `auth` command SHALL be hidden from help and SHALL return an error naming `--oauth-login` and `DTWIZ_OAUTH_LOGIN` while the `oauth-login` feature flag is disabled.

#### Scenario: Auth command with the flag off

- **GIVEN** the `oauth-login` feature flag is disabled
- **WHEN** the user runs `dtwiz auth login`
- **THEN** the command fails with an error naming `--oauth-login` and `DTWIZ_OAUTH_LOGIN=true`
- **AND** `dtwiz --help` does not list `auth`

### Requirement: Status shows the credential source

When the `oauth-login` feature flag is enabled, `dtwiz status` SHALL show which credential source is in use, and SHALL NOT print any token.

#### Scenario: Status with a session

- **GIVEN** the `oauth-login` feature flag is enabled and the credential comes from an OAuth session
- **WHEN** the user runs `dtwiz status`
- **THEN** the output shows `OAuth`, the dtctl context name, and the session expiry
