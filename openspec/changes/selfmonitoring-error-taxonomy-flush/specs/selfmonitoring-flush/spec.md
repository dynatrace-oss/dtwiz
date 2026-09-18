# Spec: Self-Monitoring Flush

## ADDED Requirements

### Requirement: Self-monitoring events are flushed before process exit

The system SHALL enqueue self-monitoring events and synchronously flush them before process exit, blocking for at most 200ms. This applies on every exit path: successful completion, error, user cancellation, and CTRL+C.

#### Scenario: Events flushed after successful command

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command completes successfully
- **THEN** all pending self-monitoring events are sent before the process exits
- **AND** the flush blocks for at most 200ms
- **AND** no flush status is shown to the user

#### Scenario: Events flushed after command failure

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command fails and returns a non-nil error
- **THEN** all pending self-monitoring events (including the failure event) are sent before the process exits
- **AND** the flush blocks for at most 200ms

#### Scenario: Events flushed after user cancellation

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** a command exits due to user cancellation (e.g. declining the Y/N confirmation)
- **THEN** all pending self-monitoring events are sent before the process exits

#### Scenario: Flush timeout accepted silently

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the pending events cannot be sent within 200ms (e.g. slow network)
- **THEN** the process exits immediately after 200ms
- **AND** no error or warning is shown to the user
- **AND** any unsent events are accepted as lost

#### Scenario: Flush with no pending events is a no-op

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is disabled or no events were enqueued
- **WHEN** the flush runs at process exit
- **THEN** the process exits immediately with no delay

#### Scenario: Feature flag disabled — no events enqueued

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is not enabled
- **WHEN** any dtwiz command runs
- **THEN** no events are enqueued and no flush HTTP call is made

### Requirement: Credentials are resolved at enqueue time

The system SHALL resolve Dynatrace credentials when an event is enqueued, not at flush time. If credentials cannot be resolved at enqueue time, the event is silently discarded.

#### Scenario: Credentials resolved successfully at enqueue time

- **GIVEN** `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN` are set
- **WHEN** `fireSelfMonitoringEvent` is called
- **THEN** credentials are resolved immediately and the event is added to the pending queue

#### Scenario: Missing credentials at enqueue time

- **GIVEN** `DT_ENVIRONMENT` is not set
- **WHEN** `fireSelfMonitoringEvent` is called
- **THEN** the event is silently discarded and does not enter the queue
- **AND** no error is shown to the user

### Requirement: Flush sends all pending events concurrently

When flushing, the system SHALL send all pending events concurrently rather than sequentially, so that multiple events from a single invocation do not compound the flush latency.

#### Scenario: Multiple events flushed concurrently

- **GIVEN** two or more events are pending (e.g. `st=inv` and `st=fai`)
- **WHEN** flush runs
- **THEN** all events are dispatched concurrently
- **AND** the total flush time is bounded by the slowest single event, not the sum of all events
