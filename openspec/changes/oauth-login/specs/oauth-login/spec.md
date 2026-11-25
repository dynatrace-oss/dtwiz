# Spec Delta

## Purpose

Define how dtwiz logs in to a Dynatrace environment through the browser, uses that session to provision a platform token on a dedicated service user, and uses that token as a credential source.

## ADDED Requirements

### Requirement: Browser login shared with dtctl

When the `oauth-login` feature flag is enabled, `dtwiz auth login` SHALL run the OAuth authorization-code flow with PKCE for the given environment, using dtctl's OAuth client for that environment, and SHALL store the session in dtctl's config and token storage through the dtctl SDK. dtwiz SHALL NOT require or call the `dtctl` binary.

#### Scenario: Login creates a dtctl context

- **GIVEN** the `oauth-login` feature flag is enabled
- **WHEN** the user runs `dtwiz auth login --environment https://abc12345.apps.dynatrace.com` and completes the browser login
- **THEN** dtctl's config has a context for that environment with an OAuth token
- **AND** `dtctl` commands for that environment work without another login

#### Scenario: Login without dtctl installed

- **GIVEN** the `oauth-login` feature flag is enabled, `dtctl` is not installed, and no dtctl config file exists
- **WHEN** the user runs `dtwiz auth login` and completes the browser login
- **THEN** the login succeeds and dtwiz creates the dtctl config file

#### Scenario: Existing dtctl session is reused

- **GIVEN** the `oauth-login` feature flag is enabled and the user ran `dtctl auth login` for the environment, and its granted scopes include the provisioning scopes dtwiz needs
- **WHEN** the user runs a dtwiz command for that environment without a platform token
- **THEN** dtwiz provisions its token with the existing session and doesn't open the browser

#### Scenario: Existing session lacks scopes

- **GIVEN** the `oauth-login` feature flag is enabled, no dtwiz token is stored, and the cached session for the environment lacks a provisioning scope
- **WHEN** the user runs a dtwiz command that needs the tenant
- **THEN** the command fails before any API call, naming the missing scopes and telling the user to run `dtwiz auth login`

### Requirement: Credential source order

dtwiz SHALL resolve the platform credential in this order: `--platform-token`, then `DT_PLATFORM_TOKEN`, then, only when the `oauth-login` feature flag is enabled, the dtwiz platform token provisioned for the environment.

#### Scenario: Env var wins over a session

- **GIVEN** the `oauth-login` feature flag is enabled, `DT_PLATFORM_TOKEN` is set, and a dtwiz token is stored
- **WHEN** the user runs a dtwiz command
- **THEN** dtwiz uses `DT_PLATFORM_TOKEN`

#### Scenario: Flag off ignores sessions

- **GIVEN** the `oauth-login` feature flag is disabled, no platform token is set, and an OAuth session and a stored dtwiz token exist
- **WHEN** the user runs a dtwiz command that needs the tenant
- **THEN** dtwiz fails with the existing "no Dynatrace platform token configured" error
- **AND** neither dtctl's config nor the keyring is read

#### Scenario: Environment from dtctl's current context

- **GIVEN** the `oauth-login` feature flag is enabled, neither `--environment` nor `DT_ENVIRONMENT` is set, and dtctl's current context has an OAuth session
- **WHEN** the user runs a dtwiz command
- **THEN** dtwiz uses that context's environment and the dtwiz token for it

### Requirement: Service-user platform token

When the `oauth-login` feature flag is enabled and dtwiz has an OAuth session but no stored token for the environment, dtwiz SHALL create a service user, bind the DtWiz Ingestion User policy to it, create a platform token on its behalf with dtwiz's combined scope list, and store that token in the OS keyring. dtwiz SHALL use that token, never the OAuth access token, for API calls and for every config it writes.

#### Scenario: First use provisions a token

- **GIVEN** the `oauth-login` feature flag is enabled, the user is logged in, and no dtwiz token is stored for the environment
- **WHEN** the user runs a dtwiz command that needs the tenant
- **THEN** dtwiz creates the service user, the policy binding and the platform token, and stores the token
- **AND** the command runs with that token

#### Scenario: Stored token is reused

- **GIVEN** a dtwiz token is stored for the environment and its scope-list version is current
- **WHEN** the user runs several installs
- **THEN** no new service user or platform token is created
- **AND** every written config contains the stored token

#### Scenario: Scope list changed

- **GIVEN** the stored token's scope-list version is older than dtwiz's and the session is valid
- **WHEN** the user runs a dtwiz command that needs the tenant
- **THEN** dtwiz creates a new token on the same service user and replaces the stored one

#### Scenario: Binding already exists

- **GIVEN** the DtWiz Ingestion User policy is already bound to the service user
- **WHEN** dtwiz provisions a token
- **THEN** the "binding already exists" response counts as success

#### Scenario: Provisioning not allowed

- **GIVEN** the user lacks the permission to create service users, bindings or platform tokens
- **WHEN** dtwiz tries to provision a token
- **THEN** dtwiz names the missing permissions and asks the user for a platform token, as it does today

#### Scenario: Pasted token is kept

- **GIVEN** the credential came from `DT_PLATFORM_TOKEN`
- **WHEN** the user runs an install
- **THEN** dtwiz writes that token, as it does today, and provisions nothing

### Requirement: Logout

When the `oauth-login` feature flag is enabled, `dtwiz auth logout` SHALL remove the OAuth tokens and the stored dtwiz token for the environment, and SHALL NOT revoke that platform token or delete the service user.

#### Scenario: Logout keeps installed tokens

- **GIVEN** the user is logged in and a dtwiz token is stored and used in an installed config
- **WHEN** the user runs `dtwiz auth logout`
- **THEN** the OAuth tokens and the stored dtwiz token for the environment are removed from the machine
- **AND** the platform token still works in the installed config
- **AND** the output says where to revoke it

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

- **GIVEN** the `oauth-login` feature flag is enabled and the credential is the provisioned dtwiz token
- **WHEN** the user runs `dtwiz status`
- **THEN** the output shows `dtwiz login`, the service user name, and the token expiry
