# Azure Monitor: Extension Activation Race Fix

## CHANGED Requirements

### Requirement: Wait for extension to be active before creating monitoring config

After a fresh hub install of the Azure data-acquisition extension, `dtwiz install azure`
must poll until the extension is active before creating the monitoring configuration.
It polls every 5 s for up to 60 s and shows progress to the user.

#### Scenario: Fresh install, extension becomes active in time

- **GIVEN** the da-azure extension is not installed on the tenant
- **WHEN** `dtwiz install azure` runs
- **THEN** step 6 (update connection) completes
- **AND** `installExtension()` installs the extension and returns `freshlyInstalled = true`
- **AND** the install flow prints `"Extension freshly installed — waiting for it to become active..."`
- **AND** the flow polls `isExtensionActive()` until `Active == true`
- **AND** `"✓ Extension is active"` is printed
- **AND** step 7 (create monitoring configuration) succeeds

#### Scenario: Extension already installed, no wait needed

- **GIVEN** the da-azure extension is already installed and active on the tenant
- **WHEN** `dtwiz install azure` runs (e.g. after a partial failure at step 7)
- **THEN** `installExtension()` returns `freshlyInstalled = false`
- **AND** no polling runs
- **AND** step 7 proceeds immediately

#### Scenario: Fresh install, activation poll times out

- **GIVEN** the da-azure extension is freshly installed
- **AND** the extension is not active within 60 s
- **WHEN** the poll exhausts all attempts
- **THEN** a debug log is written
- **AND** step 7 runs and the Extensions API response determines the outcome

### Requirement: `installExtension()` returns whether the extension was freshly installed

`dtclient.installExtension()` returns `(bool, error)`, where `true` means the extension
was just installed from the hub (202 Accepted) and `false` means it was already present.

#### Scenario: Extension already installed

- **GIVEN** `LatestExtensionVersion` succeeds (extension found in the version list)
- **WHEN** `installExtension()` is called
- **THEN** it returns `(false, nil)` without calling `InstallFromHub`

#### Scenario: Extension freshly installed from hub

- **GIVEN** `LatestExtensionVersion` fails (extension not found)
- **WHEN** `installExtension()` is called and `InstallFromHub` succeeds
- **THEN** it returns `(true, nil)`
