# Spec: Install Self-Monitoring

## Purpose

Instrument each `dtwiz install <method>` subcommand with self-monitoring telemetry events so that install sessions are observable end-to-end: invocation, install completion or failure (including user cancellation), and first-data confirmation in the post-install watch session.

## Event flow per session

For every `dtwiz install <method>` invocation (not via `dtwiz setup`), exactly two self-monitoring events are emitted in sequence:

1. `inv` (invoked) — fired immediately when the subcommand's `RunE` begins, before any install logic. Ensures the invocation is recorded even if the process is killed mid-install.
2. `ist` (install) — fired when the installer returns, regardless of outcome. Carries `install.duration_ms`, per-method feature flags, and a classified error type when applicable.

A third event, `com` (completed), is fired asynchronously by the post-install WatchIngest session when signals are first detected or when the session times out. It is owned entirely by WatchIngest and is absent when the user exits before data arrives.

Pre-install failures (credential lookup, token validation, client setup) emit `fai` (failed) instead of `ist` — the installer was never reached.

## ADDED Requirements

### Requirement: Install commands emit an inv event on invocation

When the self-monitoring feature flag is enabled and a `dtwiz install <method>` subcommand is invoked directly, the system SHALL emit an `inv` step event immediately when the command starts, before any install logic executes.

#### Scenario: inv fires before install logic

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the user runs `dtwiz install <method>`
- **THEN** an `inv` event is emitted immediately in `PersistentPreRun`, before credential resolution or install execution

### Requirement: Install commands emit an ist event on completion, failure, or cancellation

When the self-monitoring feature flag is enabled and a `dtwiz install <method>` subcommand is invoked directly (not via `dtwiz setup`), the system SHALL emit an `ist` step event when the installer returns — whether it succeeded, failed, or was cancelled by the user.

#### Scenario: Successful install emits ist with no error field

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the user runs `dtwiz install <method>` and the installer completes successfully
- **THEN** a self-monitoring event with step `ist`, command `ins`, and subcommand matching the method shortcode is emitted with no error field

#### Scenario: Failed install emits ist with classified error type

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the installer returns a non-cancellation error
- **THEN** a self-monitoring event with step `ist`, command `ins`, and error field set to the classified error type from the taxonomy (e.g. `install_failed`, `network_error`, `dependency_missing`) is emitted

#### Scenario: Cancelled install emits ist with error type user_cancelled

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the user declines the install confirmation prompt
- **THEN** an `ist` event is emitted with `error = "user_cancelled"`; `install.duration_ms` is absent because `ExecutionStart` was never set

#### Scenario: Dry-run emits ist with no duration

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and `--dry-run` is passed
- **WHEN** the dry-run exits without executing anything
- **THEN** an `ist` event is emitted with no error field and no `install.duration_ms` property

#### Scenario: Pre-install failure emits fai instead of ist

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** credential resolution (`getDtEnvironment`) or token validation (`validateCredentials`) fails before the installer is called
- **THEN** a `fai` event is emitted with a classified error type; no `ist` event is emitted

### Requirement: The ist event carries install.duration_ms excluding user think time

The `ist` event SHALL include an `install.duration_ms` property encoding the elapsed time in milliseconds from the moment the user confirmed the install prompt to the moment the installer returned. User think time (the time between prompt display and keystroke) SHALL be excluded from this value.

For install methods that have no interactive confirmation prompt (currently `oneagent`), the duration starts when the installer function is called.

The property SHALL be absent when the install was not executed (dry-run or cancellation).

#### Scenario: Duration is present on successful install with confirmation

- **GIVEN** the installer ran with an interactive confirmation prompt
- **WHEN** the `ist` event is assembled after the installer returns
- **THEN** `install.duration_ms` is present and reflects the time from user confirmation to install completion, not from command invocation

#### Scenario: Duration is absent on dry-run

- **GIVEN** `--dry-run` is passed and the installer did not execute
- **WHEN** the `ist` event is assembled
- **THEN** `install.duration_ms` is absent from the event properties

### Requirement: The ist event carries per-method feature flags

The `ist` event SHALL include feature flag properties that identify which Dynatrace capabilities were activated during the install. Only properties applicable to the install method SHALL be present; inapplicable fields SHALL be absent rather than set to false.

The following properties are defined for current install methods:

- `install.host_monitoring_enabled` (`"true"`) — present for methods that activate Dynatrace host monitoring: `oneagent`, `kubernetes`, `otel`, `otel-collector`, `docker`, `demo`
- `install.otel_pipelines` (`"traces,metrics,logs"`) — present for methods that generate an OTel Collector config. Currently a static value: all OTel installers always configure all three pipelines.

The following properties are defined but not yet applicable to any current install method (absent for all current methods):

- `install.rum_enabled` — whether the RUM snippet was generated and injected. No current install method activates RUM.
- `install.synthetic_enabled` — whether a Synthetic monitor was created. No current install method activates Synthetic.
- `install.rds_extension_enabled` — whether the AWS RDS extension was auto-enabled. The `aws` installer deploys a generic CloudFormation data-acquisition stack; it does not detect or return RDS extension status. This field is absent for all current methods.

#### Scenario: oneagent ist event includes host_monitoring_enabled

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** `dtwiz install oneagent` completes
- **THEN** the `ist` event properties include `install.host_monitoring_enabled: "true"` and no `install.otel_pipelines` field

#### Scenario: otel ist event includes both feature flags

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** `dtwiz install otel` completes
- **THEN** the `ist` event properties include both `install.host_monitoring_enabled: "true"` and `install.otel_pipelines: "traces,metrics,logs"`

#### Scenario: otel-python ist event includes otel_pipelines only

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** `dtwiz install otel-python` completes
- **THEN** the `ist` event properties include `install.otel_pipelines: "traces,metrics,logs"` and no `install.host_monitoring_enabled` field

#### Scenario: aws ist event includes no feature flags

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** `dtwiz install aws` completes
- **THEN** the `ist` event properties include no `install.*` feature flag fields

### Requirement: Install commands wire up the com event from WatchIngest

When the self-monitoring feature flag is enabled and a `dtwiz install <method>` subcommand runs a post-install WatchIngest session, the system SHALL fire a `com` step event when the first signal data is received OR when the session times out without data — using the same signal timing encoding as `dtwiz setup` and `dtwiz watch`.

When the user exits WatchIngest early (Enter key) before data arrives, no `com` event is emitted.

The `com` event is fired asynchronously by WatchIngest via `buildWatchEventCallback`. It is not fired by the install command handler itself.

#### Scenario: com event fires on first data after install

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and `dtwiz install <method>` ran successfully
- **WHEN** the post-install WatchIngest session detects its first signal data
- **THEN** a `com` event is emitted asynchronously with `t=<signals>` encoding time-to-first-data per signal, without interrupting the watch display

#### Scenario: com event fires on watch timeout with no data

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and the post-install WatchIngest session reaches 10 minutes with no signal data
- **THEN** a `com` event is emitted with no `t=` field

#### Scenario: no com event when user exits watch before data

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and `dtwiz install <method>` ran successfully
- **WHEN** the user presses Enter to exit WatchIngest before any signal data arrives
- **THEN** no `com` event is emitted; only the `inv` and `ist` events are present for the session

### Requirement: Install methods cover all production and experimental subcommands

The following install subcommands SHALL be instrumented with `inv`, `ist`, and (when applicable) `com` events:

- Production: `oneagent`, `kubernetes`, `otel`, `otel-collector`, `otel-python`, `otel-node`, `otel-java`, `aws`, `aws-lambda`, `azure`, `gcp`
- Experimental (gated by `DTWIZ_EXPERIMENTAL`): `docker`, `demo`

#### Scenario: All production methods emit ist

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** any production install subcommand completes
- **THEN** an `ist` event with the correct `ins` command and method subcommand shortcode is emitted

#### Scenario: Experimental methods emit ist when experimental flag is enabled

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` and `DTWIZ_EXPERIMENTAL` are both enabled
- **WHEN** `dtwiz install docker` or `dtwiz install demo` completes
- **THEN** an `ist` event with command `ins` and the appropriate subcommand shortcode (`dock` or `demo`) is emitted
