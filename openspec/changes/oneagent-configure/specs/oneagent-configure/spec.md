# OneAgent Installer Token Configuration

## ADDED Requirements

### Requirement: Mandatory installer-token minting

`InstallOneAgentV2` SHALL mint a short-lived `InstallerDownload`-scoped token for the duration of the install. The user-supplied `--access-token` SHALL NOT be passed to the installer binary. There SHALL be no fallback to the user token if minting fails — minting failure SHALL abort the install.

#### Scenario: Token minted successfully

- **GIVEN** the user-supplied token has the `tokens.write` scope
- **WHEN** `MintInstallerToken(c)` is called
- **THEN** it issues `POST /api/v2/tokens` with body `{name: "dtwiz-oneagent-installer", scopes: ["InstallerDownload"], expiresIn: {value: 1, unit: "HOURS"}}`
- **AND** returns the value of the response `token` field
- **AND** the request is signed with the user-supplied access token

#### Scenario: Mint failure aborts install

- **GIVEN** the tenant API returns 403 with body `"Missing scope tokens.write"`
- **WHEN** `MintInstallerToken(c)` is called inside `InstallOneAgentV2`
- **THEN** `InstallOneAgentV2` returns an error including the API status and body
- **AND** `DownloadInstaller`, `BuildInstallCommand`, `ExecuteInstallCommand`, and `WaitForHostRegistration` are not called

#### Scenario: Network failure aborts install

- **GIVEN** the tenant API is unreachable (network error after resty retry exhaustion)
- **WHEN** `MintInstallerToken(c)` is called
- **THEN** it returns a wrapped error whose message includes guidance to verify `--environment` and any proxy settings

### Requirement: Token-minting debug logging

`MintInstallerToken` SHALL emit `logger.Debug` lines describing the request and the response status, and `logger.Debug` lines describing failures including the response body. The minted token value SHALL NEVER appear in any log line at any level.

#### Scenario: Mint request logged at Debug

- **GIVEN** `--debug` is enabled
- **WHEN** `MintInstallerToken(c)` issues `POST /api/v2/tokens`
- **THEN** stderr contains a Debug line with message `"minting installer token"` and keys `url`, `scopes`, `expires_in`

#### Scenario: Mint success logs status only

- **GIVEN** `--debug` is enabled
- **AND** the API returns 200 with a body containing a token
- **WHEN** `MintInstallerToken` returns
- **THEN** stderr contains a Debug line with message `"installer token minted"` and key `status` (value `200`)
- **AND** no log line contains the token value
- **AND** no log line contains the response body (which holds the token)

#### Scenario: Mint failure logs status and body

- **GIVEN** `--debug` is enabled
- **AND** the API returns 403 with body `"Missing scope tokens.write"`
- **WHEN** `MintInstallerToken` returns the error
- **THEN** stderr contains a Debug line with message `"installer token mint failed"` and keys `status` (value `403`), `body` (value containing `"Missing scope tokens.write"`)

### Requirement: Token confidentiality

The minted token SHALL be held only in memory for the duration of the install. The minted token value SHALL NOT be logged at any level (`logger.Debug`, `logger.Info`, stdout, stderr). The user-supplied `--access-token` SHALL NOT be passed to the OneAgent installer subprocess or any of its environment variables.

#### Scenario: Token not logged

- **GIVEN** `dtwiz install oneagent --verbose --debug` is run
- **WHEN** `MintInstallerToken(c)` succeeds and returns a token value
- **THEN** no log line contains the token value
- **AND** debug logs contain only the fact that minting succeeded and the response status

#### Scenario: User token not passed to installer

- **GIVEN** the install proceeds to `BuildInstallCommand`
- **WHEN** the command argv is constructed
- **THEN** no argv element contains the value of the user-supplied `--access-token`

### Requirement: Token used only for the download

The minted token SHALL be used exclusively to authenticate the installer-binary download. Subsequent requests (post-install verification, optional connectivity probe) SHALL continue to use the original client configured by `setupClient()`.

#### Scenario: Post-install verification uses original client

- **GIVEN** `MintInstallerToken` returned `tok-installer-abc`
- **WHEN** `WaitForHostRegistration` polls Grail via `POST /platform/storage/query/v1/query:execute`
- **THEN** the request is signed with the user-supplied platform token (Bearer), not `tok-installer-abc`
- **AND** `tok-installer-abc` is used only for the installer-binary download against the Classic API
