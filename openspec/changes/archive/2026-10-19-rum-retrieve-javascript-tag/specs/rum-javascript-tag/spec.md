# Spec: RUM JavaScript Tag

## NEW Requirements

### Requirement: Retrieve the JavaScript tag for a known frontend entity

The system SHALL retrieve the current OpenTelemetry RUM JavaScript tag for a given Smartscape
FRONTEND entity, given that the entity's identifier is already known.

#### Scenario: Tag retrieved successfully

- **GIVEN** a FRONTEND entity identifier that corresponds to an existing entity
- **WHEN** JavaScript tag retrieval runs for that identifier
- **THEN** the system returns the current JavaScript tag for that entity

#### Scenario: Unknown frontend entity is reported as a dedicated failure

- **GIVEN** a FRONTEND entity identifier that does not correspond to any existing entity
- **WHEN** JavaScript tag retrieval runs for that identifier
- **THEN** the system reports a failure that specifically indicates the entity was not found
- **AND** this failure is distinguishable from other kinds of failures

#### Scenario: Other failures are not misreported as "entity not found"

- **GIVEN** JavaScript tag retrieval fails for any reason other than the entity being unknown
- **WHEN** that failure occurs
- **THEN** the system reports a failure
- **AND** that failure is not reported as the entity-not-found case
