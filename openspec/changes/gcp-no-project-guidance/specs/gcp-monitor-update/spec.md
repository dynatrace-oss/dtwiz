# Spec Delta

## MODIFIED Requirements

### Requirement: Concurrent discovery and account lookup

The system SHALL, in parallel, discover existing monitoring configurations, discover existing connections, and resolve the active `gcloud` project and account. Any of the three failing SHALL abort the update. A missing `gcloud` account or project SHALL show the matching guidance box (see `gcp-cli-preflight`) with `dtwiz update gcp` as the command to re-run, printed once after the parallel lookup finishes. Because an update never creates or modifies IAM bindings, it SHALL NOT run any permission-hint or role-grant logic.

#### Scenario: Project lookup failure aborts update

- **GIVEN** resolving the active `gcloud` project fails during the parallel lookup
- **WHEN** the update runs
- **THEN** it aborts before previewing or mutating anything

#### Scenario: No active project shows guidance

- **GIVEN** `gcloud` has an active account but no active project
- **WHEN** the user runs `dtwiz update gcp`
- **THEN** the no-project guidance box is shown exactly once with `dtwiz update gcp` as the command to re-run
- **AND** no preview is printed and no monitoring configuration is changed

#### Scenario: Not logged in shows guidance

- **GIVEN** `gcloud` has no active account
- **WHEN** the user runs `dtwiz update gcp`
- **THEN** the not-logged-in guidance box is shown exactly once with `dtwiz update gcp` as the command to re-run
- **AND** no monitoring configuration is changed
