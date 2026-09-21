# Spec: Self-Monitoring Error Taxonomy

## Purpose

Classify every dtwiz command failure or cancellation into a single structured taxonomy category and
report it, with its defining attributes, on a terminal self-monitoring event. This makes the
drop-off funnel visible: which commands fail, at what step, and why.

## Requirements

### Requirement: Every command outcome emits a terminal self-monitoring event

The system SHALL emit exactly one terminal event per command invocation before the command
returns: a cancelled event when the user cancels, a failed event for any other error, and a
completed event on success.

#### Scenario: Command fails with an error

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command fails for any reason other than user cancellation
- **THEN** one failed event is emitted before the command returns
- **AND** the event reports the classified error category

#### Scenario: Command is cancelled by the user

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the user declines a confirmation prompt or selects the cancel entry at the recommendation menu
- **THEN** one cancelled event is emitted before the command returns
- **AND** the event reports the user-cancelled category

#### Scenario: Command completes successfully

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command succeeds
- **THEN** one completed event is emitted
- **AND** no error category is reported

#### Scenario: Feature flag disabled

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is not enabled
- **WHEN** a command fails or is cancelled
- **THEN** no terminal event is emitted

### Requirement: Errors are classified using the structured taxonomy

The system SHALL classify every error into exactly one taxonomy category and report that category,
together with any attributes defined for it, on the terminal event. The categories and their
attributes are defined in `design.md`.

#### Scenario: Missing environment variable

- **GIVEN** `DT_ENVIRONMENT` is not set and the user runs a command that requires credentials
- **WHEN** credential validation fails
- **THEN** the terminal event reports the config-error category
- **AND** the missing field is named in the event

#### Scenario: Multiple missing fields captured in a single event

- **GIVEN** both `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN` are not set
- **WHEN** credential validation fails
- **THEN** one terminal event reports the config-error category
- **AND** both missing fields are named in that single event

#### Scenario: Missing dependency

- **GIVEN** a required external binary such as `helm` is not installed
- **WHEN** the binary is not found on the system
- **THEN** the terminal event reports the dependency-missing category
- **AND** the name of the missing binary is included

#### Scenario: Platform not supported

- **GIVEN** the user runs an install method on an operating system it does not support
- **WHEN** the installer checks the platform
- **THEN** the terminal event reports the platform-unsupported category
- **AND** no additional attributes are included

#### Scenario: Unrecognised error

- **GIVEN** a command fails with an error matching no specific category
- **WHEN** the error is classified
- **THEN** the terminal event reports the install-failed category
- **AND** the failing step is included when the error identifies one

### Requirement: Classification is deterministic

The system SHALL classify errors in a fixed priority order, so that an error matching more than one
category always produces the same result.

#### Scenario: An error matches more than one category

- **GIVEN** an error that matches several categories
- **WHEN** it is classified
- **THEN** the highest-priority matching category is used
- **AND** exactly one category is reported

### Requirement: The event body is the query surface

The event body SHALL carry the full, unabbreviated value of every field it reports, and every field
encoded in the `User-Agent` SHALL also be present in the body. The `User-Agent` MAY abbreviate
values to stay within its capture limit.

#### Scenario: Header abbreviation does not affect the body

- **GIVEN** a command fails with a platform-unsupported error
- **WHEN** the event is sent
- **THEN** the event body reports the full category name
- **AND** the `User-Agent` carries an abbreviated form of it

### Requirement: An error category is reported only when an error occurred

The system SHALL report an error category only on terminal events where a failure or cancellation
was classified. No error category SHALL appear on invocation events or on successful completion
events.

#### Scenario: Invocation event has no error category

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** a command is invoked
- **THEN** the invocation event reports no error category

#### Scenario: Failure event reports its category

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and a command fails
- **WHEN** the failed event is sent
- **THEN** it reports the classified error category
