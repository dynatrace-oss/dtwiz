# command-selfmonitoring

## ADDED Requirements

### Requirement: Informational commands emit a completed self-monitoring event

When self-monitoring is enabled, the `analyze`, `recommend`, `status`, and `version` commands SHALL each emit a completed self-monitoring event after the command finishes executing. The event SHALL indicate failure when the command exits with an error and indicate success otherwise.

#### Scenario: analyze completes successfully

- **GIVEN** self-monitoring is enabled and the Dynatrace environment is reachable
- **WHEN** `dtwiz analyze` runs and exits without error
- **THEN** a completed self-monitoring event identifying the `analyze` command is sent with a success indicator

#### Scenario: analyze fails during system analysis

- **GIVEN** self-monitoring is enabled and the Dynatrace environment is reachable
- **WHEN** `dtwiz analyze` runs and system analysis fails
- **THEN** a completed self-monitoring event identifying the `analyze` command is sent with a failure indicator before the command exits

#### Scenario: recommend completes successfully

- **GIVEN** self-monitoring is enabled and the Dynatrace environment is reachable
- **WHEN** `dtwiz recommend` runs and exits without error
- **THEN** a completed self-monitoring event identifying the `recommend` command is sent with a success indicator

#### Scenario: recommend fails during system analysis

- **GIVEN** self-monitoring is enabled and the Dynatrace environment is reachable
- **WHEN** `dtwiz recommend` runs and system analysis fails
- **THEN** a completed self-monitoring event identifying the `recommend` command is sent with a failure indicator before the command exits

#### Scenario: status completes successfully

- **GIVEN** self-monitoring is enabled and the Dynatrace environment is reachable
- **WHEN** `dtwiz status` runs and exits without error
- **THEN** a completed self-monitoring event identifying the `status` command is sent with a success indicator

#### Scenario: status fails during system analysis

- **GIVEN** self-monitoring is enabled and the Dynatrace environment is reachable
- **WHEN** `dtwiz status` runs and system analysis fails
- **THEN** a completed self-monitoring event identifying the `status` command is sent with a failure indicator before the command exits

#### Scenario: version completes

- **GIVEN** self-monitoring is enabled and the Dynatrace environment is reachable
- **WHEN** `dtwiz version` runs
- **THEN** a completed self-monitoring event identifying the `version` command is sent with a success indicator

### Requirement: Completed events are distinguishable from setup internal steps

Self-monitoring events emitted by standalone informational commands SHALL be distinguishable from events emitted during the `setup` wizard's internal steps.

#### Scenario: standalone analyze vs setup analyze step

- **GIVEN** self-monitoring is enabled
- **WHEN** `dtwiz analyze` emits a completed event and `dtwiz setup` emits its internal analysis step event
- **THEN** the two events are distinguishable by the originating command without inspecting additional fields

### Requirement: Completed events are not sent when the Dynatrace environment is unreachable

The completed self-monitoring event SHALL NOT be delivered when the Dynatrace environment is not configured or unreachable. This failure SHALL be silent and SHALL NOT affect the command's exit code or user-visible output.

#### Scenario: Dynatrace environment not configured

- **GIVEN** self-monitoring is enabled but no Dynatrace environment is configured
- **WHEN** any informational command runs to completion
- **THEN** no self-monitoring event is delivered and the command exits normally without surfacing any delivery error to the user
