# Spec Delta

## ADDED Requirements

### Requirement: Go projects are listed as auto-instrumentable when the flag is enabled

When the `GoInstrumentation` flag is enabled, detected Go projects SHALL appear in the detected-projects list of `dtwiz install otel` like Python, Java, and Node.js projects, and Go SHALL NOT be offered in the list of languages without automatic project detection (the manual-languages list). When the flag is not enabled, Go SHALL remain in the manual-languages list and behavior is unchanged.

#### Scenario: Flag enabled

- **GIVEN** the `GoInstrumentation` flag is enabled
- **WHEN** the interactive project list is printed
- **THEN** detected Go projects are listed together with the other runtimes' projects
- **AND** Go is not shown in the manual-languages list

#### Scenario: Flag not enabled

- **GIVEN** the `GoInstrumentation` flag is not enabled
- **WHEN** the interactive project list is printed
- **THEN** Go is shown in the manual-languages list as before
- **AND** selecting it shows the manual OpenTelemetry guide after install
