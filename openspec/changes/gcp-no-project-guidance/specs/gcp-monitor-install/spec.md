# Spec Delta

## MODIFIED Requirements

### Requirement: Preflight checks before any mutation

The system SHALL run the shared GCP preflight checks (see `gcp-cli-preflight`) before mutating any resource: the `gcloud` CLI SHALL be present on `PATH`, `gcloud` SHALL have an active account, and `gcloud` SHALL have a non-empty, non-`(unset)` active project. Values read from `gcloud` configuration SHALL be parsed identically to system detection, stripping any Cloud Shell "active configuration" notice line rather than treating it as part of the value. A missing account or project SHALL stop the install with the matching guidance box; a missing CLI SHALL abort with a message pointing to the install page.

#### Scenario: Google Cloud CLI missing

- **GIVEN** `gcloud` is not on `PATH`
- **WHEN** the installer runs preflight
- **THEN** it aborts with a message pointing to the Google Cloud CLI install page

#### Scenario: Not logged in or no active project

- **GIVEN** `gcloud` has no active account, or has an active account but its active project is empty or `(unset)`
- **WHEN** the installer runs preflight
- **THEN** it shows the matching guidance box (not-logged-in or no-project) with `dtwiz install gcp` as the command to re-run
- **AND** nothing is created or changed

#### Scenario: Cloud Shell notice line stripped from project ID

- **GIVEN** `gcloud config get-value project` outputs `Your active configuration is: [cloudshell-123]` followed by the project ID on the next line
- **WHEN** preflight parses the output
- **THEN** the resolved project ID is the value on the last line, not the notice text
