# GCP Monitor Uninstall

## ADDED Requirements

### Requirement: Uninstall command and entry point

The system SHALL expose `dtwiz uninstall gcp` as the CLI command for GCP removal, accepting no positional arguments. It SHALL remove the Dynatrace GCP integration created by the installer, resolve the environment URL and platform token from the standard sources, and honor the shared `--dry-run` flag. The package SHALL also expose a helper for callers (such as `dtwiz setup`) to detect whether a complete integration is present.

#### Scenario: Uninstall command registered

- **GIVEN** the CLI is built
- **WHEN** the user runs `dtwiz uninstall gcp`
- **THEN** the uninstaller discovers and removes dtwiz-created GCP integration resources

### Requirement: Discover all dtwiz-named resources concurrently

The system SHALL discover every Dynatrace connection and every `da-gcp` monitoring configuration whose name equals `dtwiz-gcp`, returning all matches (not just the first) so leftovers from interrupted runs are also removed. Connection and monitoring-configuration lookups SHALL run concurrently, and if both fail the resulting error SHALL include both failures rather than only the first.

#### Scenario: Duplicate leftovers all discovered

- **GIVEN** two connections named `dtwiz-gcp` exist from an interrupted prior run
- **WHEN** the uninstaller discovers resources
- **THEN** both connection object IDs are returned for deletion

#### Scenario: Concurrent discovery failures are both reported

- **GIVEN** the monitoring-configuration lookup and the connection lookup both fail
- **WHEN** discovery runs
- **THEN** the returned error contains both underlying failures

### Requirement: Service-account discovery tolerates a missing active project

The system SHALL gather the service-account emails to clean up from two sources: those bound to a discovered connection, and the deterministic email for the active `gcloud` project (when one is available). Resolving the active `gcloud` project SHALL be best-effort during uninstall: if it fails, the system SHALL continue without service-account cleanup rather than aborting, since the Dynatrace-side resources can still be removed.

#### Scenario: No active project skips service-account cleanup only

- **GIVEN** `gcloud` reports no active project during uninstall
- **WHEN** the uninstaller runs
- **THEN** it still discovers and removes connections and monitoring configurations
- **AND** it performs no service-account or project-IAM-binding deletions

### Requirement: Nothing-to-do short-circuit

When no monitoring configurations and no connections are discovered, the system SHALL print that there is nothing to uninstall and return without prompting or mutating.

#### Scenario: No resources found

- **GIVEN** the environment has no dtwiz-named GCP resources
- **WHEN** the uninstaller runs
- **THEN** it prints a "nothing to uninstall" message and exits successfully

### Requirement: Preview and confirmation before deletion

The system SHALL print a preview listing the environment, the active project (when known), each connection, each service account, and each monitoring configuration found (or noting when one is absent), followed by the numbered deletion steps. It SHALL prompt a single `Apply?` confirmation (default yes). On `--dry-run` it SHALL print the preview and stop; on decline it SHALL cancel without making any changes.

#### Scenario: Dry run previews deletions only

- **GIVEN** resources exist and `--dry-run` is set
- **WHEN** the uninstaller runs
- **THEN** it prints the deletion preview and `[dry-run] No changes were made.` without deleting

### Requirement: Best-effort deletion that continues past failures

The system SHALL delete resources in order: monitoring configurations, then (only when a project is active) per service account the project Viewer binding removal and the service-account deletion, then connections. A failed step SHALL print a warning and the run SHALL continue with remaining steps; all step errors SHALL be collected and returned together. Service-account deletes and project-binding removals that report "not found", and monitoring-configuration and connection deletes that report the resource already gone, SHALL be treated as success.

#### Scenario: One failure does not strand the rest

- **GIVEN** three deletion steps where the second fails
- **WHEN** the uninstaller runs
- **THEN** steps one and three still execute and the second step's error is returned

#### Scenario: Already-gone resource treated as success

- **GIVEN** a connection delete reports the object already gone
- **WHEN** that step runs
- **THEN** it is treated as success with no error

#### Scenario: Service account deletion removes the impersonation binding

- **GIVEN** a GCP service account is deleted
- **WHEN** the step completes
- **THEN** the Dynatrace principal's impersonation binding on that service account is removed along with it

### Requirement: Step count reflects discovered resources

The system SHALL compute the total number of deletion steps as one per monitoring configuration, two per service account (project Viewer binding removal plus service-account deletion, only when a project is active), and one per connection, and SHALL number the printed steps continuously against that total.

#### Scenario: Step total computed

- **GIVEN** 1 monitoring config, 1 service account, 1 connection, and an active project
- **WHEN** the step count is computed
- **THEN** it equals 4 (1 + 2 + 1)

#### Scenario: Step total excludes service-account steps without a project

- **GIVEN** 1 monitoring config and 1 connection are discovered but no `gcloud` project is active
- **WHEN** the step count is computed
- **THEN** it equals 2 (1 + 0 + 1)
