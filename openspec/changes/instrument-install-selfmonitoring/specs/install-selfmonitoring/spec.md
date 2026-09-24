# Spec: Install Self-Monitoring

## Purpose

Instrument each `dtwiz install <method>` subcommand with self-monitoring telemetry events so that install sessions are observable end-to-end: invocation, install completion or failure, and first-data confirmation in the post-install watch session.

## ADDED Requirements

### Requirement: Install commands emit an ist event on completion or failure

When the self-monitoring feature flag is enabled and a `dtwiz install <method>` subcommand is invoked directly (not via `dtwiz setup`), the system SHALL emit an `ist` step event when the installer returns — whether it succeeded or failed — carrying the install method as the subcommand identifier.

The event SHALL be suppressed entirely when the user declines the install confirmation prompt (`ErrInstallCancelled`). Absence of the `ist` event signals that the user never confirmed.

#### Scenario: Successful install emits ist with no error field

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the user runs `dtwiz install <method>` and the installer completes successfully
- **THEN** a self-monitoring event with step `ist`, command `ins`, and subcommand matching the method shortcode is emitted with no error field

#### Scenario: Failed install emits ist with error field

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the installer returns a non-cancellation error
- **THEN** a self-monitoring event with step `ist`, command `ins`, and error field `"err"` is emitted

#### Scenario: Cancelled install emits no ist event

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the user declines the install confirmation prompt
- **THEN** no `ist` event is emitted; only the initial `inv` event is present for the session

#### Scenario: Dry-run emits ist with no duration

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and `--dry-run` is passed
- **WHEN** the dry-run exits without executing anything
- **THEN** an `ist` event is emitted with no error field and no `install.duration_ms` property

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

The following properties are defined:

- `install.host_monitoring_enabled` (`"true"`) — present for methods that activate Dynatrace host monitoring: `oneagent`, `kubernetes`, `otel`, `otel-collector`, `docker`, `demo`
- `install.otel_pipelines` (`"traces,metrics,logs"`) — present for methods that generate an OTel Collector config with all three pipeline types: `otel`, `otel-collector`, `otel-python`, `otel-node`, `otel-java`, `demo`

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

For methods where no data is received before the user exits, no `com` event is emitted.

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

The following install subcommands SHALL be instrumented with `ist` and `com` events:

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
