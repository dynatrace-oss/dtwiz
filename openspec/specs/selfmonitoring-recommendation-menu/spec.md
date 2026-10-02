# Spec: Self-Monitoring Recommendation Menu

## Purpose

Emit self-monitoring events for the recommendations presented in the setup menu and for the option the user selects, so that recommendation exposure and selection can be tracked and correlated per session.

## Requirements

### Requirement: Self-monitoring event per presented recommendation

When the setup menu is displayed, the system SHALL emit one self-monitoring event per actionable recommendation using the "recommendations presented" step.

#### Scenario: Menu displayed with multiple methods

- **GIVEN** the setup menu contains Kubernetes and OTel as actionable recommendations
- **WHEN** the setup menu is displayed
- **THEN** two self-monitoring events are emitted using the "recommendations presented" step: one carrying the Kubernetes option and one carrying the OTel option

#### Scenario: No actionable recommendations

- **GIVEN** the setup menu contains no actionable recommendations
- **WHEN** the setup flow runs
- **THEN** no "recommendations presented" events are emitted

#### Scenario: Emitting events never blocks or interrupts the menu

- **GIVEN** the setup menu contains actionable recommendations
- **WHEN** the events are emitted
- **THEN** the menu is displayed and accepts input regardless of whether the events can be delivered, and no delivery failure is surfaced to the user

### Requirement: Setup menu option encoded separately from subcommand

The system SHALL encode the ingestion option involved in a setup recommend event using a dedicated field that is distinct from the subcommand field used by other commands. The same abbreviated method identifiers used elsewhere in self-monitoring SHALL be reused.

#### Scenario: Presented and selected events both carry the option identifier

- **GIVEN** the setup menu presents Kubernetes as an actionable recommendation and the user selects it
- **WHEN** the resulting "recommendations presented" and "recommendations selected" events are emitted
- **THEN** both events carry the Kubernetes method identifier in the dedicated option field, not in the subcommand field

### Requirement: Self-monitoring event for selected recommendation

When the user selects an option from the setup menu, the system SHALL emit one self-monitoring event using the "recommendations selected" step, making it distinguishable from presented events.

#### Scenario: User selects a method

- **GIVEN** the setup menu presents Kubernetes as an actionable recommendation
- **WHEN** the user selects Kubernetes from the setup menu
- **THEN** a "recommendations selected" event is emitted carrying the Kubernetes option identifier

#### Scenario: All recommend events for a session share a correlation identifier

- **GIVEN** the setup menu presents three methods and the user selects one
- **WHEN** all recommend events are emitted
- **THEN** all events carry the same execution identifier in the event body, enabling them to be correlated in queries
