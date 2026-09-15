# Spec: Self-Monitoring Error Taxonomy

## NEW Requirements

### Requirement: Every command failure emits a terminal self-monitoring event

The system SHALL emit exactly one terminal self-monitoring event per command invocation — `StepFailed` for errors or `StepCancelled` for user cancellations — before the command returns.

#### Scenario: Command fails with an error

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command's `RunE` returns a non-nil error (other than `ErrInstallCancelled`)
- **THEN** one `st=fai` event is enqueued before the error is returned
- **AND** the event carries the classified `error.type` in the `er=` User-Agent field and `error` body property

#### Scenario: Command is cancelled by the user

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the user declines a Y/N confirmation prompt or selects `0` at the recommendation menu
- **THEN** one `st=can` event is enqueued before the command returns nil
- **AND** the event carries `er=user_cancelled`

#### Scenario: Command completes successfully

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command's `RunE` returns nil
- **THEN** one `st=com` event is enqueued
- **AND** the `er=` field is absent

#### Scenario: Feature flag disabled — no terminal event

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is not enabled
- **WHEN** a command fails or is cancelled
- **THEN** no terminal event is enqueued

### Requirement: Errors are classified using the structured taxonomy

The system SHALL classify every error into exactly one of the following taxonomy categories and include the classification in the terminal event. Additional attributes are included where specified.

| `error.type` | Additional attributes | When |
|---|---|---|
| `user_cancelled` | — | User declined Y/N prompt or typed `0` at recommendation menu |
| `auth_error` | `auth.failure_reason`: `invalid_token`, `environment_not_reachable`, or `authentication_failed` | Authentication failed, invalid or missing token, environment unreachable |
| `config_error` | `config.missing_fields`: array of missing field names (`DT_ENVIRONMENT`, `DT_PLATFORM_TOKEN`, `project_path`) | Missing environment URL, missing platform token, or project path not found before the install could start |
| `dependency_missing` | `dependency.name`: name of the missing binary | A required external tool was not found |
| `network_error` | `network.failure_reason`, `network.url` (if available) | Connection timeout, environment not reachable, or unexpected HTTP response |
| `install_failed` | `install.step`: free-text name of the step that failed | Install execution failed; catch-all for failures not matching a more specific category |
| `platform_unsupported` | — | Current OS or architecture is not supported by the selected install method |

#### Scenario: Missing environment variable

- **GIVEN** `DT_ENVIRONMENT` is not set and the user runs a command that requires credentials
- **WHEN** credential validation fails
- **THEN** the terminal event carries `er=config_error` and a body property `config.missing_fields` listing `DT_ENVIRONMENT`
- **AND** `tenant.id` is absent because the environment URL is unknown

#### Scenario: Multiple missing fields captured in a single event

- **GIVEN** both `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN` are not set
- **WHEN** credential validation fails
- **THEN** the terminal event carries `er=config_error` and `config.missing_fields` contains both field names in a single event

#### Scenario: Invalid token

- **GIVEN** `DT_ENVIRONMENT` is set and `DT_PLATFORM_TOKEN` is set to an invalid value
- **WHEN** the token validation call returns HTTP 401
- **THEN** the terminal event carries `er=auth_error` and `auth.failure_reason=authentication_failed`

#### Scenario: Missing dependency

- **GIVEN** a required external binary (e.g. `kubectl`, `helm`, `aws-cli`) is not installed
- **WHEN** the installer checks for the binary using `exec.LookPath`
- **THEN** the terminal event carries `er=dependency_missing` and `dependency.name` set to the name of the missing binary

#### Scenario: Platform not supported

- **GIVEN** the user runs `dtwiz install oneagent` on macOS
- **WHEN** the installer checks the OS
- **THEN** the terminal event carries `er=platform_unsupported`
- **AND** no additional attributes are included

#### Scenario: Unrecognised error falls through to install_failed

- **GIVEN** a command fails with an error that does not match any specific taxonomy type
- **WHEN** the error is classified
- **THEN** the terminal event carries `er=install_failed`
- **AND** `install.step` is included if the error carries a step identifier; otherwise the attribute is absent

### Requirement: Classification priority is deterministic

The system SHALL classify errors in a fixed priority order so that a single error with multiple matching types always produces the same classification.

#### Scenario: Error chain contains multiple classifiable types

- **GIVEN** an error wraps both a `DependencyMissingError` and a generic message
- **WHEN** the error is classified
- **THEN** the most specific matching type in the priority order is used
- **AND** only one `error.type` is present in the event

## MODIFIED Requirements

### Requirement: Self-monitoring events include `er=` only when an error occurred

The `er=` field in the User-Agent and the `error` body property SHALL be present only on terminal events where a failure or cancellation was classified. They SHALL be absent on `st=inv` and successful `st=com` events.

#### Scenario: Invocation event has no error field

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** a command is invoked
- **THEN** the `st=inv` event User-Agent does not contain `er=`
- **AND** the event body does not contain an `error` property

#### Scenario: Failure event carries error type

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and a command fails
- **WHEN** the `st=fai` event is sent
- **THEN** the User-Agent contains `er=<type>` and the event body contains `error: "<type>"`
