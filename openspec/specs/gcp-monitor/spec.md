# GCP Monitor

## Purpose

Define how dtwiz names GCP Monitor resources (Dynatrace connection, monitoring configuration, GCP service account) using a Dynatrace environment-scoped identifier, how discovery uses prefix matching to handle both legacy fixed names and new env-scoped names across install, update, and uninstall, and how install handles the extension activation race after a fresh hub install.

## Requirements

### Requirement: Env-scoped resource naming

All resources created by dtwiz (Dynatrace connection, monitoring configuration, GCP service account) SHALL use a name derived from the Dynatrace environment URL: `dtwiz-gcp-<tenant-id>`, where `<tenant-id>` is the first DNS label of the URL (e.g. `dtwiz-gcp-fds1499d` for `https://fds1499d.apps.dynatracelabs.com`).

#### Scenario: Derived name used for all new resources

- **GIVEN** `dtwiz install gcp` runs against environment `https://fds1499d.apps.dynatracelabs.com`
- **WHEN** resources are created
- **THEN** the Dynatrace connection, GCP service account, and monitoring configuration are all named `dtwiz-gcp-fds1499d`

### Requirement: Prefix-based discovery in install and update covers old and new names

Discovery in both `dtwiz install gcp` and `dtwiz update gcp` SHALL search for connections and monitoring configurations using the `dtwiz-gcp` prefix, not the derived name. This ensures that connections created before env-scoped naming (named `dtwiz-gcp`) are found and handled correctly alongside connections using the new env-scoped name.

#### Scenario: Pre-env-scoped-naming connection found by install

- **GIVEN** a complete Dynatrace GCP connection named `dtwiz-gcp` (created before this change) exists
- **WHEN** `dtwiz install gcp` runs
- **THEN** the existing connection is found by the duplicate check
- **AND** install delegates to the update flow instead of creating a new service account

#### Scenario: Pre-env-scoped-naming connection found by update

- **GIVEN** a Dynatrace GCP connection named `dtwiz-gcp` (created before this change) carries a bound service-account email
- **WHEN** `dtwiz update gcp` runs
- **THEN** the connection is found by discovery
- **AND** the monitoring configuration is reconciled in place

#### Scenario: Env-scoped connection found by install duplicate check

- **GIVEN** a complete Dynatrace GCP connection named `dtwiz-gcp-fds1499d` exists
- **WHEN** `dtwiz install gcp` runs against `https://fds1499d.apps.dynatracelabs.com`
- **THEN** the existing connection is found
- **AND** install delegates to the update flow

### Requirement: Uninstall uses prefix to cover both naming generations

Discovery in `dtwiz uninstall gcp` SHALL use the `dtwiz-gcp` prefix to find and remove connections and monitoring configurations regardless of whether they use the old fixed name or the new env-scoped name. Legacy service accounts (named after the old fixed prefix) SHALL be cleaned up with warn-only failure semantics.

#### Scenario: Old-style and new-style resources both removed by uninstall

- **GIVEN** both a `dtwiz-gcp` connection and a `dtwiz-gcp-fds1499d` connection exist
- **WHEN** `dtwiz uninstall gcp` runs
- **THEN** both connections and their associated monitoring configurations are removed

#### Scenario: Legacy service account cleanup failure is non-fatal

- **GIVEN** the legacy `dtwiz-gcp`-named service account cannot be deleted (e.g. already removed or insufficient permissions)
- **WHEN** `dtwiz uninstall gcp` runs
- **THEN** a warning is printed but the uninstall continues and succeeds

### Requirement: Wait for extension to be active before creating monitoring config

After a fresh hub install of the GCP data-acquisition extension, `dtwiz install gcp`
SHALL poll until the extension is active before creating the monitoring configuration.
It SHALL poll every 5 s for up to 60 s and show progress to the user.

#### Scenario: Fresh install, extension becomes active in time

- **GIVEN** the da-gcp extension is not installed on the tenant
- **WHEN** `dtwiz install gcp` runs
- **THEN** step 6 (update connection) completes
- **AND** `installExtension()` installs the extension and returns `freshlyInstalled = true`
- **AND** the install flow prints `"Extension freshly installed — waiting for it to become active..."`
- **AND** the flow polls `isExtensionActive()` until `Active == true`
- **AND** `"✓ Extension is active"` is printed
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
