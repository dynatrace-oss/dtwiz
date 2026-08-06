# GCP Monitor: Extension Activation Race Fix

## CHANGED Requirements

### Requirement: Wait for extension to be active before creating monitoring config

After a fresh hub install of the GCP data-acquisition extension, `dtwiz install gcp`
must poll until the extension is active before creating the monitoring configuration.
It polls every 5 s for up to 60 s and shows progress to the user.

#### Scenario: Fresh install, extension becomes active in time

- **GIVEN** the da-gcp extension is not installed on the tenant
- **WHEN** `dtwiz install gcp` runs
- **THEN** step 6 (update connection) completes
- **AND** `installExtension()` installs the extension and returns `freshlyInstalled = true`
- **AND** the install flow prints `"Extension freshly installed -- waiting for it to become active..."`
- **AND** the flow polls `isExtensionActive()` until `Active == true`
- **AND** `"Extension is active"` is printed
- **AND** step 7 (create monitoring configuration) succeeds

#### Scenario: Extension already installed, no wait needed

- **GIVEN** the da-gcp extension is already installed and active on the tenant
- **WHEN** `dtwiz install gcp` runs
- **THEN** `installExtension()` returns `freshlyInstalled = false`
- **AND** no polling runs
- **AND** step 7 proceeds immediately

#### Scenario: Fresh install, activation poll times out

- **GIVEN** the da-gcp extension is freshly installed
- **AND** the extension is not active within 60 s
- **WHEN** the poll exhausts all attempts
- **THEN** a debug log is written
- **AND** step 7 runs and the Extensions API response determines the outcome
