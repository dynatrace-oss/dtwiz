# Spec: Self-Monitoring Flush

## ADDED Requirements

### Requirement: Self-monitoring events are delivered before process exit

The system SHALL wait for pending self-monitoring event deliveries to complete before the process
exits, on every exit path: successful completion, error, user cancellation, and interruption. The
wait SHALL be bounded so that it does not perceptibly delay the command, and SHALL be invisible to
the user.

#### Scenario: Events delivered after a successful command

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command completes successfully
- **THEN** all pending events are delivered before the process exits
- **AND** no flush status is shown to the user

#### Scenario: Events delivered after a command failure

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command fails
- **THEN** all pending events, including the failure event, are delivered before the process exits

#### Scenario: Events delivered after user cancellation

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command exits because the user declined a confirmation prompt
- **THEN** all pending events are delivered before the process exits

#### Scenario: A fast-failing command still delivers its event

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and the tenant URL is known
- **WHEN** a command fails quickly enough that it would otherwise exit before delivery completes
- **THEN** the event is still delivered

#### Scenario: Exceeding the wait is accepted silently

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** pending events cannot be delivered within the bounded wait
- **THEN** the process exits without further delay
- **AND** no error or warning is shown to the user
- **AND** the undelivered events are accepted as lost

#### Scenario: Nothing pending causes no delay

- **GIVEN** no events are pending
- **WHEN** the process exits
- **THEN** it exits with no added delay

#### Scenario: Feature flag disabled

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is not enabled
- **WHEN** any dtwiz command runs
- **THEN** no events are produced and no network call is made

### Requirement: Events are delivered when they occur

The system SHALL deliver each event at the point it is produced rather than batching events for
delivery at exit, so that each event's ingestion time reflects when its step actually happened.

#### Scenario: Two events from one invocation have distinct ingestion times

- **GIVEN** a command produces an invocation event and later a terminal event
- **WHEN** both are delivered
- **THEN** their ingestion times differ and preserve the order in which the steps occurred
- **AND** the elapsed time between the steps can be derived from them

### Requirement: An event is discarded only when the tenant is unknown

The system SHALL attempt delivery whenever the tenant URL is known, including when the token is
missing or invalid, because a rejected request is still recorded by the tenant. An event SHALL be
discarded only when no tenant URL is configured, as there is no destination.

#### Scenario: Token is missing or rejected

- **GIVEN** a tenant URL is configured but the token is missing or invalid
- **WHEN** an event is produced
- **THEN** delivery is still attempted
- **AND** the resulting rejection is not shown to the user

#### Scenario: Tenant URL is unknown

- **GIVEN** no tenant URL is configured
- **WHEN** an event is produced
- **THEN** the event is discarded and no delivery is attempted
- **AND** no error is shown to the user
