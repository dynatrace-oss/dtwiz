# Spec Delta

## Purpose

Defines the shared `gcloud` readiness checks that every GCP command runs before doing any work, and the guidance users see when a check fails, so they can fix their local setup and re-run the command.

## ADDED Requirements

### Requirement: Active account and project checked before any GCP command runs

Before discovering, previewing or mutating anything, `dtwiz install gcp`, `dtwiz update gcp` and `dtwiz uninstall gcp` SHALL check in order: the `gcloud` CLI is on `PATH`, `gcloud` has an active account, and `gcloud` has an active project. Values read from `gcloud` configuration SHALL be parsed the same way as during system detection.

#### Scenario: Account checked before project

- **GIVEN** `gcloud` is on `PATH` with neither an active account nor an active project
- **WHEN** the user runs `dtwiz install gcp`
- **THEN** the not-logged-in guidance is shown
- **AND** the no-project guidance is not shown

#### Scenario: Both present lets the command proceed

- **GIVEN** `gcloud` has an active account and the active project `my-project`
- **WHEN** the user runs `dtwiz install gcp`
- **THEN** no guidance box is shown and the command continues with project `my-project`

#### Scenario: Cloud Shell notice line ignored

- **GIVEN** reading the active project prints `Your active configuration is: [cloudshell-123]` followed by `my-project` on the next line
- **WHEN** the check runs
- **THEN** the active project is `my-project`

### Requirement: Active account means stored credentials, read like system detection

The active account SHALL be the account `gcloud auth list` reports as active, which requires stored credentials; an account that is only configured (the `core/account` setting) SHALL NOT count. Account and project SHALL be read by the same code as during system detection, from `gcloud`'s standard output only.

#### Scenario: Configured account without credentials is not logged in

- **GIVEN** `gcloud config get-value account` returns `jane@example.com`, but `gcloud` has no stored credentials for that account
- **WHEN** the user runs `dtwiz install gcp`
- **THEN** the not-logged-in guidance is shown

#### Scenario: Setup and install agree on the login state

- **GIVEN** any `gcloud` account and project state
- **WHEN** `dtwiz setup` detects GCP and `dtwiz install gcp` runs its checks
- **THEN** both treat the user as logged in, or both treat them as not logged in

### Requirement: No-project guidance box

When no project is active, the system SHALL NOT run the command. It SHALL print an info box that states the command was not run because no `gcloud` project is selected, names the active `gcloud` account, and SHALL list in order: `gcloud projects list` to find the project ID, `gcloud config set project <PROJECT_ID>` to select it, and the GCP command to re-run.

#### Scenario: Install without project

- **GIVEN** `gcloud` has an active account but no active project
- **WHEN** the user runs `dtwiz install gcp`
- **THEN** an info box says the install was not run because no `gcloud` project is selected
- **AND** it shows `gcloud projects list`, `gcloud config set project <PROJECT_ID>` and `dtwiz install gcp` as the steps to follow
- **AND** no Dynatrace or GCP resource is read for discovery, created or changed

#### Scenario: Active account named in the box

- **GIVEN** `gcloud` is logged in as `jane@example.com` with no active project
- **WHEN** any GCP command runs, with or without `--dry-run`
- **THEN** the no-project guidance box names `jane@example.com` as the logged-in account

#### Scenario: Update without project

- **GIVEN** `gcloud` has an active account but no active project
- **WHEN** the user runs `dtwiz update gcp`
- **THEN** the info box names update as the command that was not run and shows `dtwiz update gcp` as the command to re-run
- **AND** no monitoring configuration is changed

#### Scenario: Uninstall without project explains what would be left behind

- **GIVEN** `gcloud` has an active account but no active project
- **WHEN** the user runs `dtwiz uninstall gcp`
- **THEN** the info box names uninstall as the command that was not run and shows `dtwiz uninstall gcp` as the command to re-run
- **AND** the box states that without a project only the Dynatrace resources could be removed, leaving the GCP service account and its project binding in place
- **AND** nothing is deleted

#### Scenario: Project unset reported as "(unset)"

- **GIVEN** `gcloud` reports the active project as `(unset)`
- **WHEN** any GCP command runs
- **THEN** it is treated as no active project and the no-project guidance box is shown

### Requirement: Not-logged-in guidance box

When no account is active, the system SHALL NOT run the command. It SHALL print an info box that states the command was not run because no `gcloud` account is logged in, and SHALL list in order: `gcloud auth login` and the GCP command to re-run.

#### Scenario: Logged out with a project still configured

- **GIVEN** `gcloud` has the active project `my-project` but no active account
- **WHEN** the user runs `dtwiz install gcp`
- **THEN** an info box says the install was not run because no `gcloud` account is logged in
- **AND** it shows `gcloud auth login` and `dtwiz install gcp` as the steps to follow
- **AND** no resource is created or changed

### Requirement: Guidance shown on dry run

The guidance boxes SHALL also be shown with `--dry-run`, in place of the preview, because an accurate preview requires an active account and project.

#### Scenario: Dry run without project

- **GIVEN** `gcloud` has an active account but no active project
- **WHEN** the user runs `dtwiz install gcp --dry-run`
- **THEN** the no-project guidance box is shown
- **AND** no preview and no `[dry-run]` line are printed

### Requirement: Guidance outcome exits as a configuration failure

A command stopped by a guidance box SHALL exit with a non-zero status, SHALL NOT print an additional error line after the box, and SHALL report a configuration-error outcome to self-monitoring naming the missing `gcloud` setting. Errors from all other commands SHALL keep their current output and exit status.

#### Scenario: Exit status and output

- **GIVEN** `gcloud` has an active account but no active project
- **WHEN** the user runs `dtwiz install gcp`
- **THEN** the process exits with a non-zero status
- **AND** the box is the last output, with no `Error: ...` line after it

#### Scenario: Self-monitoring classification

- **GIVEN** `gcloud` has no active project
- **WHEN** `dtwiz install gcp` stops with the guidance box
- **THEN** the terminal event reports a failed outcome in the configuration-error category naming the missing `gcloud` project setting

#### Scenario: Unrelated errors unchanged

- **GIVEN** a command fails for a reason that shows no guidance box
- **WHEN** the command returns
- **THEN** the error is printed as `Error: <message>` and the process exits with status 1, as before
