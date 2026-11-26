# Spec Delta

## ADDED Requirements

### Requirement: Uninstall requires an active account and project

Unless `--dynatrace-only` is set, before discovering or deleting anything, the system SHALL run the shared GCP preflight checks (see `gcp-cli-preflight`). A missing `gcloud` account or project SHALL stop the uninstall with the matching guidance box and `dtwiz uninstall gcp` as the command to re-run, so Dynatrace and GCP resources are only removed separately when the user asks for it.

#### Scenario: No active project stops uninstall

- **GIVEN** a Dynatrace GCP connection and monitoring configuration exist and `gcloud` has no active project
- **WHEN** the user runs `dtwiz uninstall gcp`
- **THEN** the no-project guidance box is shown, explaining that the GCP service account and its project binding would otherwise stay behind
- **AND** no connection, monitoring configuration, service account or binding is deleted

#### Scenario: Not logged in stops uninstall

- **GIVEN** `gcloud` has no active account
- **WHEN** the user runs `dtwiz uninstall gcp`
- **THEN** the not-logged-in guidance box is shown with `dtwiz uninstall gcp` as the command to re-run
- **AND** nothing is deleted

### Requirement: Dynatrace-only uninstall

`dtwiz uninstall gcp` SHALL accept a `--dynatrace-only` flag for a project that was deleted or is no longer accessible. With it, the system SHALL NOT run or require `gcloud`. It SHALL discover, preview and delete only the Dynatrace monitoring configurations and connections, with the same confirmation, `--dry-run`, step numbering and best-effort semantics as the full uninstall.

#### Scenario: Works without gcloud

- **GIVEN** the `gcloud` CLI is not installed
- **WHEN** the user runs `dtwiz uninstall gcp --dynatrace-only`
- **THEN** no dependency error is shown and the Dynatrace resources are discovered

#### Scenario: Deleted project, Dynatrace side removed

- **GIVEN** a Dynatrace GCP connection bound to `dtwiz-gcp-abc@old-project.iam.gserviceaccount.com` and a monitoring configuration exist, and the GCP project `old-project` was deleted
- **WHEN** the user runs `dtwiz uninstall gcp --dynatrace-only` and confirms
- **THEN** the monitoring configuration and the connection are deleted
- **AND** no `gcloud` command runs

#### Scenario: Nothing to remove

- **GIVEN** no Dynatrace GCP connection or monitoring configuration exists
- **WHEN** the user runs `dtwiz uninstall gcp --dynatrace-only`
- **THEN** it reports that there is nothing to uninstall and deletes nothing

### Requirement: Dynatrace-only preview names the GCP resources left behind

The `--dynatrace-only` preview SHALL warn that GCP resources are left in place and SHALL list the service-account emails bound to the discovered connections, so the user can remove them if the project still exists.

#### Scenario: Preview names what stays behind

- **GIVEN** a discovered connection is bound to `dtwiz-gcp-abc@old-project.iam.gserviceaccount.com`
- **WHEN** the user runs `dtwiz uninstall gcp --dynatrace-only --dry-run`
- **THEN** the preview warns that GCP resources are left in place and names that service account
- **AND** lists only Dynatrace deletion steps
- **AND** nothing is deleted

### Requirement: Service-account discovery uses connections and the active project

The system SHALL gather the service-account emails to clean up from two sources: those bound to a discovered connection, and the deterministic emails for the active `gcloud` project.

#### Scenario: Connection-bound and deterministic emails combined

- **GIVEN** a discovered connection carries a bound service-account email and `gcloud` has the active project `my-project`
- **WHEN** the uninstaller gathers service accounts
- **THEN** the bound email and the deterministic email for `my-project` are both scheduled for cleanup, without duplicates

### Requirement: Deletion step total counts every discovered resource

The system SHALL compute the total number of deletion steps as one per monitoring configuration, two per service account (project Viewer binding removal plus service-account deletion), and one per connection, and SHALL number the printed steps continuously against that total.

#### Scenario: Step total computed

- **GIVEN** 1 monitoring config, 1 service account, 1 connection, and an active project
- **WHEN** the step count is computed
- **THEN** it equals 4 (1 + 2 + 1)

## MODIFIED Requirements

### Requirement: Preview and confirmation before deletion

The system SHALL print a preview listing the environment, the active project, each connection, each service account, and each monitoring configuration found (or noting when one is absent), followed by the numbered deletion steps. It SHALL prompt a single `Apply?` confirmation (default yes). On `--dry-run` it SHALL print the preview and stop; on decline it SHALL cancel without making any changes.

#### Scenario: Dry run previews deletions only

- **GIVEN** resources exist and `--dry-run` is set
- **WHEN** the uninstaller runs
- **THEN** it prints the deletion preview and `[dry-run] No changes were made.` without deleting

### Requirement: Best-effort deletion that continues past failures

The system SHALL delete in order: monitoring configurations, then per service account the Viewer binding removal and service-account deletion, then connections. A failed step SHALL print a warning and continue; all errors SHALL be collected and returned. "Not found" responses SHALL be treated as success.

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

## REMOVED Requirements

### Requirement: Step count reflects discovered resources

**Reason**: Its "only when a project is active" condition and the matching scenario no longer apply, because uninstall now requires an active project.
**Migration**: Replaced by "Deletion step total counts every discovered resource", which keeps the same counting rule without the project condition.

### Requirement: Service-account discovery tolerates a missing active project

**Reason**: A project-less uninstall removed only the Dynatrace resources and silently left the GCP service account and its Viewer binding behind, with nothing in the output saying so.
**Migration**: Select the project the integration was installed into (`gcloud projects list`, then `gcloud config set project <PROJECT_ID>`) and re-run `dtwiz uninstall gcp`. The guidance box shows these steps. If the project was deleted or is no longer accessible, run `dtwiz uninstall gcp --dynatrace-only` (see "Dynatrace-only uninstall"). Service-account discovery is now covered by "Service-account discovery uses connections and the active project".
