# OneAgent Installer Token Configuration

## ADDED Requirements

### Requirement: Installer token resolved from existing credentials

`InstallOneAgentV2` SHALL resolve the token used for the installer download from `InstallOptions.Token`, which is set upstream by `validateCredentials` in `cmd/install.go` (access token preferred, platform token fallback). If the token is empty, the install SHALL abort with a clear error.

#### Scenario: Access token resolved

- **GIVEN** `opts.Token` holds a `dt0c01.*` access token
- **WHEN** `ResolveInstallerToken(opts)` is called
- **THEN** it returns the token value and logs `source=access`

#### Scenario: Platform token resolved as fallback

- **GIVEN** `opts.Token` holds a platform token (non-`dt0c01.*` prefix)
- **WHEN** `ResolveInstallerToken(opts)` is called
- **THEN** it returns the token value and logs `source=platform`

#### Scenario: Empty token aborts install

- **GIVEN** `opts.Token` is empty
- **WHEN** `ResolveInstallerToken(opts)` is called inside `InstallOneAgentV2`
- **THEN** `InstallOneAgentV2` returns an error indicating no credentials are available
- **AND** no further install steps are executed

### Requirement: Token resolution debug logging

`ResolveInstallerToken` SHALL emit a `logger.Debug` line recording which credential source was selected (`access` or `platform`). The token value SHALL NEVER appear in any log line at any level.

#### Scenario: Resolution logged at Debug

- **GIVEN** `--debug` is enabled
- **WHEN** `ResolveInstallerToken(opts)` returns successfully
- **THEN** stderr contains a Debug line with message `"resolved installer token"` and key `source` (value `"access"` or `"platform"`)
- **AND** no log line contains the token value

### Requirement: Token confidentiality

The resolved token SHALL be held only in memory. The token value SHALL NOT be logged at any level (`logger.Debug`, `logger.Info`, stdout, stderr).

#### Scenario: Token not logged

- **GIVEN** `dtwiz install oneagent --verbose --debug` is run
- **WHEN** `ResolveInstallerToken(opts)` returns successfully
- **THEN** no log line contains the token value
