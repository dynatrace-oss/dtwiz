# GCP Monitor Install

## ADDED Requirements

### Requirement: Install command and entry point

The system SHALL expose `dtwiz install gcp` as the final runnable command for GCP installation, accepting no extra positional arguments. The command SHALL set up the Dynatrace Google Cloud integration, resolve the Dynatrace environment URL and platform token from the standard sources (`--environment`/`DT_ENVIRONMENT`, `--platform-token`/`DT_PLATFORM_TOKEN`), and honor the shared `--dry-run` flag.

#### Scenario: Install command registered

- **GIVEN** the CLI is built
- **WHEN** the user runs `dtwiz install gcp`
- **THEN** the GCP installer runs against the resolved environment and platform token
- **AND** `--dry-run` previews the workflow without making changes

### Requirement: Service-account impersonation authentication

The system SHALL authenticate Dynatrace to GCP using service-account impersonation and SHALL NOT create, download, or store any service-account key file. The Dynatrace principal SHALL be granted `roles/iam.serviceAccountTokenCreator` on a service account dtwiz creates, and the Dynatrace connection SHALL reference that service account's email.

#### Scenario: No key file created

- **GIVEN** the installer creates the GCP service account and grants impersonation
- **WHEN** the workflow completes
- **THEN** no service-account key is generated, printed, or persisted to disk

### Requirement: Preflight checks before any mutation

The system SHALL run preflight checks before mutating any resource: the `gcloud` CLI SHALL be present on `PATH`, and `gcloud config get-value project` SHALL resolve to a non-empty, non-`(unset)` active project. Values read from `gcloud config get-value` SHALL be parsed identically to system detection, stripping any Cloud Shell "active configuration" notice line rather than treating it as part of the value. Failure of either check SHALL abort the install with a user-actionable message.

#### Scenario: Google Cloud CLI missing

- **GIVEN** `gcloud` is not on `PATH`
- **WHEN** the installer runs preflight
- **THEN** it aborts with a message pointing to the Google Cloud CLI install page

#### Scenario: Not logged in or no active project

- **GIVEN** `gcloud config get-value project` fails or returns `(unset)`
- **WHEN** the installer runs preflight
- **THEN** it aborts with a message instructing the user to log in and set an active project

#### Scenario: Cloud Shell notice line stripped from project ID

- **GIVEN** `gcloud config get-value project` outputs `Your active configuration is: [cloudshell-123]` followed by the project ID on the next line
- **WHEN** preflight parses the output
- **THEN** the resolved project ID is the value on the last line, not the notice text

### Requirement: Redirect to update, resume, or abort based on existing connection completeness

Before installing, the system SHALL discover every Dynatrace connection named `dtwiz-gcp` and classify each as complete (carries a bound service-account email) or incomplete (does not). If exactly one complete connection is found, the system SHALL print a note ("prerequisites already exist — running update instead of a fresh install") and transparently redirect to the update flow without making any new gcloud mutations. If more than one complete connection is found, the system SHALL fall through to the ambiguity check below. If more than one incomplete connection is found, the system SHALL abort with guidance to uninstall and reinstall for a clean slate. If exactly one incomplete connection is found, the system SHALL resume it: its object ID SHALL be reused in step 2 instead of creating a new connection, and the remaining steps SHALL proceed normally.

#### Scenario: Existing complete connection redirects to update

- **GIVEN** exactly one connection named `dtwiz-gcp` already carries a bound service-account email
- **WHEN** `dtwiz install gcp` runs
- **THEN** it prints a note that prerequisites already exist and runs the update flow instead of a fresh install
- **AND** no new gcloud mutations are made
- **AND** no new Dynatrace connection is created

#### Scenario: Single incomplete connection is resumed

- **GIVEN** a connection named `dtwiz-gcp` exists with no bound service-account email (left by an install interrupted between steps 2 and 6)
- **WHEN** `dtwiz install gcp` runs
- **THEN** step 2 reuses that connection's object ID instead of creating a new connection
- **AND** steps 3 through 7 proceed using that object ID

#### Scenario: Multiple incomplete connections are ambiguous

- **GIVEN** two connections named `dtwiz-gcp` exist, neither carrying a bound service-account email
- **WHEN** `dtwiz install gcp` runs
- **THEN** it aborts without mutating anything and tells the user to uninstall then reinstall for a clean single integration

### Requirement: Preview and confirmation before applying

The system SHALL print a preview showing the environment, project, service account, Dynatrace principal, connection name, and configuration name, followed by the numbered list of commands to be executed, with the platform token masked. It SHALL then prompt a single `Apply?` confirmation (default yes). On `--dry-run` it SHALL print the preview and stop without prompting or mutating; on decline it SHALL cancel without making any changes.

#### Scenario: Dry run shows preview only

- **GIVEN** `--dry-run` is set
- **WHEN** the installer runs
- **THEN** it prints the preview and `[dry-run] No changes were made.` and makes no changes

#### Scenario: Token masked in preview

- **GIVEN** the preview lists steps that reference the platform token
- **WHEN** the steps are printed
- **THEN** every occurrence of the token is replaced with `***`

#### Scenario: User declines

- **GIVEN** the preview has been shown and `--dry-run` is not set
- **WHEN** the user answers no to `Apply?`
- **THEN** the install is cancelled without mutating anything

### Requirement: Seven-step installation workflow in order

The system SHALL execute the installation as seven ordered steps: (1) enable the required Google Cloud APIs on the active project; (2) create the Dynatrace GCP connection, or reuse a resumed incomplete one; (3) create the Google Cloud service account, reusing the deterministic email if one already exists; (4) grant the service account `roles/viewer` on the project; (5) grant the Dynatrace principal `roles/iam.serviceAccountTokenCreator` on the service account; (6) finalize the Dynatrace connection with the service-account email; (7) create the `da-gcp` monitoring configuration. The connection object ID from step 2 and the service-account email from step 3 SHALL be threaded forward into the later steps.

#### Scenario: Steps run in order and thread identifiers forward

- **GIVEN** the user confirmed the install
- **WHEN** the workflow runs
- **THEN** the connection object ID from step 2 is used as the connection-update target in step 6
- **AND** the service-account email from step 3 is used as the IAM member in steps 4 and 5 and as the connection identity in steps 6 and 7

#### Scenario: Service account creation is idempotent

- **GIVEN** step 3 reports the service account already exists
- **WHEN** the installer handles the response
- **THEN** it reuses the deterministic email instead of failing

### Requirement: Monitoring configuration defaults derived from the live extension schema

The system SHALL determine the extension version to use by selecting the highest semantic version available for `com.dynatrace.extension.da-gcp`. It SHALL fetch that version's monitoring-configuration schema and populate the configuration's feature sets from the `FeatureSetsType` enum, keeping only values ending in `_essential`. Project filtering SHALL be set to the active `gcloud` project, and the credential entry SHALL reference the connection object ID and service-account email. If no `_essential` feature sets are found, the system SHALL fail with a descriptive error rather than create a partial configuration.

#### Scenario: Defaults populated from schema enum

- **GIVEN** the latest `da-gcp` schema exposes a `FeatureSetsType` enum
- **WHEN** the monitoring configuration is created
- **THEN** feature sets contains exactly the `*_essential` values from the schema

#### Scenario: Highest extension version selected

- **GIVEN** the extension lists multiple versions
- **WHEN** the installer chooses a version
- **THEN** it selects the highest by semantic-version comparison

#### Scenario: Empty feature-set enum fails fast

- **GIVEN** the schema yields no `_essential` feature sets
- **WHEN** the monitoring configuration would be created
- **THEN** the install fails with an error naming the missing enum and creates no configuration

### Requirement: Tolerate GCP IAM propagation delays on gcloud steps

The system SHALL retry each `gcloud`-driven step that can race a just-created resource (enabling APIs, granting the project Viewer binding, granting the impersonation binding) up to 12 times with a jittered ~5-second base delay while the error indicates the target resource is not yet found or not yet propagated. Any other error SHALL fail immediately without further retries.

#### Scenario: Resource not yet propagated then succeeds

- **GIVEN** a `gcloud` step first returns a not-found error for a resource created moments earlier
- **WHEN** the step retries
- **THEN** it waits a jittered delay between attempts and succeeds once the resource is visible

#### Scenario: Permanent error fails fast

- **GIVEN** a `gcloud` step returns an error unrelated to propagation
- **WHEN** the step runs
- **THEN** it returns the error immediately without retrying

### Requirement: Retry connection finalization only on the verified propagation signal

The system SHALL retry the Dynatrace connection finalization (step 6) up to 30 times, with a jittered ~30-second initial delay followed by a jittered ~5-second delay between subsequent attempts, while the error contains the verified constraint-violation signal for an unpropagated impersonation binding. It SHALL exclude a permanent schema-mismatch error (`Unknown property`) from retrying even though it is also reported as a constraint violation, and it SHALL NOT treat every error mentioning "permission" as retryable, so a permanent Dynatrace-side authorization failure fails immediately instead of exhausting the retry budget.

#### Scenario: Propagation error retried

- **GIVEN** step 6 returns the verified constraint-violation signal for an unpropagated impersonation binding
- **WHEN** the update retries
- **THEN** it waits the jittered initial and subsequent delays between attempts, up to 30 times, until the update succeeds or a different error occurs

#### Scenario: Permanent schema mismatch stops retrying

- **GIVEN** step 6 returns a constraint violation whose detail is `Unknown property`
- **WHEN** the update runs
- **THEN** the retry loop stops immediately and the error is returned

#### Scenario: Permanent permission error stops retrying

- **GIVEN** step 6 returns a permanent Dynatrace-side authorization error that is not the verified constraint-violation signal
- **WHEN** the update runs
- **THEN** the retry loop makes exactly one attempt and returns the error without sleeping

### Requirement: Role-specific permission hints on gcloud step failures

The system SHALL inspect the error from steps 1, 3, 4, and 5 for a permission-denied signal and, when found, append a hint naming the specific IAM role most likely missing for that step. This check SHALL be purely reactive: it SHALL NOT run before a step or block/delay it, and SHALL only append text to an error the step already returned.

#### Scenario: Step 3 permission error includes role hint

- **GIVEN** step 3 (create service account) fails with a permission-denied error
- **WHEN** the installer returns the error
- **THEN** the error names `roles/iam.serviceAccountAdmin` as the likely missing role

### Requirement: Connection name conflict hint when a connection is hidden from the token's view

When step 2 fails because the connection name is already taken but discovery found no matching connection, the system SHALL append a hint explaining that the object is likely owned by a different app/user context and hidden from this token's view, and SHALL point to the Dynatrace UI location to find and remove it. Retrying SHALL NOT be attempted for this condition.

#### Scenario: Hidden connection conflict explained

- **GIVEN** discovery found no `dtwiz-gcp` connection but creation fails with a name-already-taken error
- **WHEN** the installer returns the error
- **THEN** it explains the connection is likely hidden from this token's view and points to the Dynatrace UI

### Requirement: Partial-failure cleanup guidance

On failure after one or more mutating steps have completed, the system SHALL print the resources that were already created (Dynatrace connection, GCP service account, project IAM binding) with explicit commands to remove them, and SHALL note that re-running `dtwiz uninstall gcp` removes them all automatically.

#### Scenario: Hint after connection and service account created

- **GIVEN** steps 2 and 3 succeeded but a later step failed
- **WHEN** the installer returns the error
- **THEN** it lists the created DT connection and GCP service account with their cleanup commands and points to `dtwiz uninstall gcp`

### Requirement: Watch ingested data after a successful install

After a successful install, the system SHALL tail newly ingested GCP data starting from the recorded install start time. When the start time is zero (the unit-test path), the watch SHALL be skipped.

#### Scenario: Watch skipped in tests

- **GIVEN** the install start time is the zero value
- **WHEN** the install completes
- **THEN** no ingest watch is started
