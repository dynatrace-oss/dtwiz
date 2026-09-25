# Spec: selfmonitoring-recommendation-menu

## ADDED Requirements

### Requirement: Self-monitoring event per presented recommendation

When the setup menu is displayed, the system SHALL emit one self-monitoring event per actionable recommendation using the "recommendations presented" step. For the OTel recommendation, one event per detected application runtime SHALL be emitted instead of a single event. Each event carries at most one runtime identifier — never a list. Events SHALL only be emitted when self-monitoring is enabled.

#### Scenario: Menu displayed with multiple methods, no OTel runtimes detected

- **GIVEN** self-monitoring is enabled
- **GIVEN** the setup menu contains Kubernetes and OTel as actionable recommendations
- **GIVEN** no application runtimes are detected in the current directory
- **WHEN** the setup menu is displayed
- **THEN** two self-monitoring events are emitted using the "recommendations presented" step: one carrying the Kubernetes option and one carrying the OTel option

#### Scenario: OTel recommendation with detected runtimes

- **GIVEN** self-monitoring is enabled
- **GIVEN** the setup menu contains OTel as an actionable recommendation
- **GIVEN** Node.js and Python are detected in the current directory
- **WHEN** the setup menu is displayed
- **THEN** two "recommendations presented" events are emitted for the OTel option: one carrying the Node.js technology identifier and one carrying the Python technology identifier

#### Scenario: Non-OTel methods do not carry a technology identifier

- **GIVEN** self-monitoring is enabled
- **GIVEN** the setup menu contains Kubernetes and Node.js is detected in the current directory
- **WHEN** the setup menu is displayed
- **THEN** the Kubernetes "recommendations presented" event is emitted without a technology identifier

#### Scenario: Self-monitoring disabled

- **GIVEN** self-monitoring is not enabled
- **WHEN** the setup menu is displayed
- **THEN** no self-monitoring events are emitted

### Requirement: Setup menu option encoded separately from subcommand

The system SHALL encode the ingestion option involved in a setup recommend event using a dedicated field that is distinct from the subcommand field used by other commands. The same abbreviated method identifiers used elsewhere in self-monitoring SHALL be reused.

#### Scenario: Presented and selected events both carry the option identifier

- **GIVEN** self-monitoring is enabled
- **WHEN** a "recommendations presented" or "recommendations selected" event is emitted
- **THEN** the event carries the method identifier in the dedicated option field, not in the subcommand field

### Requirement: Self-monitoring event for selected recommendation

When the user selects an option from the setup menu, the system SHALL emit one self-monitoring event using the "recommendations selected" step, making it distinguishable from presented events.

#### Scenario: User selects a method

- **GIVEN** self-monitoring is enabled
- **WHEN** the user selects Kubernetes from the setup menu
- **THEN** a "recommendations selected" event is emitted carrying the Kubernetes option identifier

#### Scenario: All recommend events for a session share a correlation identifier

- **GIVEN** self-monitoring is enabled
- **GIVEN** the setup menu presents three methods and the user selects one
- **WHEN** all recommend events are emitted
- **THEN** all events carry the same execution identifier in the event body, enabling them to be correlated in queries
