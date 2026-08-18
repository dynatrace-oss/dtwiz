# OTel Collector Port Allocation

## Purpose

`dtwiz install otel` allocates ports for the collector's receivers, health endpoint, and telemetry endpoint. These
ports must avoid conflicts with already-running listeners across both wildcard and local-only bind addresses, and
post-install verification must target the assigned OTLP HTTP port.

## Requirements

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

### Requirement: A collector does not fail to bind a port that was free at the moment it was selected

`install otel` SHALL NOT select a port for the collector if another process was already listening on it at the
moment of selection, so the collector SHALL NOT immediately fail to bind that same port when it starts moments
later.

#### Scenario: A port reported as free can be bound by the collector

- **GIVEN** `install otel` selected a port for the collector because its port-availability check succeeded
- **WHEN** the collector starts and binds that same port on the configured address
- **THEN** it SHALL bind the port successfully

### Requirement: Post-install verification targets the collector's actual assigned port

After starting the collector, `install otel` SHALL check readiness and send its verification log to whichever port
was actually assigned to the collector's OTLP HTTP receiver, not a fixed default.

#### Scenario: Default OTLP HTTP port was occupied, so a different port was assigned

- **GIVEN** the collector's default OTLP HTTP port was occupied, so a different port was selected for it
- **WHEN** `install otel` verifies the collector after starting it
- **THEN** the readiness check and verification log SHALL be sent to the collector's actual assigned port
- **AND** SHALL NOT be sent to the default port
