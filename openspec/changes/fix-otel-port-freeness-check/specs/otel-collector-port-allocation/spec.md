# OTel Collector Port Allocation

## ADDED Requirements

### Requirement: Collector port selection recognizes a port as occupied regardless of which network address it is bound to

When choosing the ports a newly installed collector will use, `install otel` SHALL treat a candidate port as
unavailable whenever another process is already listening on it, whether that process is listening on all network
interfaces or only on an address reachable from the same machine.

#### Scenario: Port occupied on all network interfaces is not selected

- **GIVEN** another process is already listening on a given port on all network interfaces
- **WHEN** `install otel` selects a port for one of the collector's ingest endpoints
- **THEN** that port SHALL NOT be selected

#### Scenario: Port occupied on the machine's own address is not selected

- **GIVEN** another process is already listening on a given port on an address reachable from the same machine only
- **WHEN** `install otel` selects a port for the collector's own telemetry endpoint
- **THEN** that port SHALL NOT be selected

#### Scenario: Free ports are selected as before

- **GIVEN** no other process is listening on any of the collector's candidate ports
- **WHEN** `install otel` selects ports for the collector
- **THEN** it SHALL select the lowest available port at or above each of the collector's default ports, with each
  chosen port distinct from the others

### Requirement: A collector is never started with a port selected for it that it then fails to bind

`install otel` SHALL NOT select a port for the collector that the collector subsequently fails to bind when it starts.

#### Scenario: Selected ports are usable by the collector on startup

- **GIVEN** `install otel` has selected ports for a new collector
- **WHEN** the collector starts using those ports
- **THEN** it SHALL bind all of them successfully
- **AND** it SHALL NOT exit immediately due to one of those ports already being in use

### Requirement: Post-install verification targets the collector's actual assigned port

After starting the collector, `install otel` SHALL check readiness and send its verification log to whichever port
was actually assigned to the collector's OTLP HTTP receiver, not a fixed default.

#### Scenario: Default OTLP HTTP port was occupied, so a different port was assigned

- **GIVEN** the collector's default OTLP HTTP port was occupied, so a different port was selected for it
- **WHEN** `install otel` verifies the collector after starting it
- **THEN** the readiness check and verification log SHALL be sent to the collector's actual assigned port
- **AND** SHALL NOT be sent to the default port
