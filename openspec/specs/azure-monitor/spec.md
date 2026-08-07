# Azure Monitor

## Purpose

Define how dtwiz names Azure Monitor resources (Dynatrace connection, monitoring configuration, Azure App Registration) using a Dynatrace environment-scoped identifier, how discovery uses prefix matching to handle both legacy fixed names and new env-scoped names across install, update, and uninstall, and how install handles the extension activation race after a fresh hub install.

## Requirements

### Requirement: Env-scoped resource naming

All resources created by dtwiz (Dynatrace connection, monitoring configuration, Azure App Registration) SHALL use a name derived from the Dynatrace environment URL: `dtwiz-azure-<tenant-id>`, where `<tenant-id>` is the first DNS label of the URL (e.g. `dtwiz-azure-fds1499d` for `https://fds1499d.apps.dynatracelabs.com`).

#### Scenario: Derived name used for all new resources

- **GIVEN** `dtwiz install azure` runs against environment `https://fds1499d.apps.dynatracelabs.com`
- **WHEN** resources are created
- **THEN** the Dynatrace connection, Azure App Registration, and monitoring configuration are all named `dtwiz-azure-fds1499d`

### Requirement: Prefix-based discovery in install and update covers old and new names

Discovery in both `dtwiz install azure` and `dtwiz update azure` SHALL search for connections and monitoring configurations using the `dtwiz-azure` prefix, not the derived name. This ensures that connections created before env-scoped naming (named `dtwiz-azure`) are found and handled correctly alongside connections using the new env-scoped name.

#### Scenario: Pre-env-scoped-naming connection found by install

- **GIVEN** a complete Dynatrace Azure connection named `dtwiz-azure` (created before this change) exists
- **WHEN** `dtwiz install azure` runs
- **THEN** the existing connection is found by the duplicate check
- **AND** install delegates to the update flow instead of creating a new App Registration

#### Scenario: Pre-env-scoped-naming connection found by update

- **GIVEN** a Dynatrace Azure connection named `dtwiz-azure` (created before this change) exists
- **WHEN** `dtwiz update azure` runs
- **THEN** the connection is found by discovery
- **AND** the monitoring configuration is reconciled in place

#### Scenario: Env-scoped connection found by install duplicate check

- **GIVEN** a complete Dynatrace Azure connection named `dtwiz-azure-fds1499d` exists
- **WHEN** `dtwiz install azure` runs against `https://fds1499d.apps.dynatracelabs.com`
- **THEN** the existing connection is found
- **AND** install delegates to the update flow

### Requirement: Uninstall uses prefix to cover both naming generations

Discovery in `dtwiz uninstall azure` SHALL use the `dtwiz-azure` prefix to find and remove connections and monitoring configurations regardless of whether they use the old fixed name or the new env-scoped name.

#### Scenario: Old-style and new-style resources both removed by uninstall

- **GIVEN** both a `dtwiz-azure` connection and a `dtwiz-azure-fds1499d` connection exist
- **WHEN** `dtwiz uninstall azure` runs
- **THEN** both connections and their associated monitoring configurations are removed

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
