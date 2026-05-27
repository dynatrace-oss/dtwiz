# OneAgent Installer Token Configuration

## ADDED Requirements

### Requirement: Installer credential available via ClassicClient

`DownloadInstaller` SHALL authenticate the installer download using `c.Classic.HTTP().R()`. The `ClassicClient` resty instance already carries the correct `Authorization` header — set upstream by `validateCredentials` (access token preferred, platform token fallback) and `setupClientFromCreds` — with no additional token resolution needed at the installer layer.

#### Scenario: Download authenticated automatically

- **GIVEN** `validateCredentials` resolved `classicTok` and passed it to `setupClientFromCreds`
- **WHEN** `DownloadInstaller` calls `c.Classic.HTTP().R().Get(url)`
- **THEN** the request carries the correct `Authorization` header
- **AND** no token value is extracted or passed explicitly in installer code

### Requirement: Token confidentiality

The credential is embedded in the resty HTTP client and never extracted to a variable in installer code. The resty client redacts the `Authorization` header in verbose output via `sensitiveHTTPHeaders` in `pkg/client/client.go`.

#### Scenario: Token not logged

- **GIVEN** `dtwiz install oneagent --verbose --debug` is run
- **THEN** no log line in the installer layer contains the raw token value
