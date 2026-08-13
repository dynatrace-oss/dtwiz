# OTel Collector Update

## MODIFIED Requirements

### Requirement: Collector restart after patching

After patching the config file, `dtwiz` SHALL restart any running collector that owns the patched config. The
restart mechanism differs between native processes and containers.

For **native processes**: `dtwiz` SHALL kill the old process and re-launch the binary with the updated config, then
SHALL verify the restarted collector against Dynatrace using the OTLP HTTP port read from the patched config,
falling back to the default port only when it cannot be determined.

For **containers**: the patched config is already on the host filesystem (host-mounted case) or has been copied back
into the container (extract/copy-back case). `dtwiz` SHALL restart the container via `<runtime> restart <name>` and
SHALL attempt verification best-effort against the OTLP HTTP port read from the patched config; that port may or
may not be exposed to the host depending on the container's port mapping.

#### Scenario: Native collector restarted after patch

- **GIVEN** a native OTel Collector process was matched and the config was patched
- **WHEN** the user confirms
- **THEN** the old process is killed and the collector is restarted with the updated config
- **THEN** the restart is verified against Dynatrace on the OTLP HTTP port read from the patched config
- **THEN** `✓ Collector restarted and verified.` is printed on success

#### Scenario: Container restarted after patch (host-mounted config)

- **GIVEN** a container collector matched the patched config via a host-mounted path
- **WHEN** the user confirms
- **THEN** `Restarting container <name>...` is printed
- **THEN** `<runtime> restart <name>` is executed
- **THEN** `Container <name> restarted.` is printed
- **THEN** Dynatrace verification is attempted on the OTLP HTTP port read from the patched config, and a warning is
  printed on failure

#### Scenario: Container restarted after patch (config extracted from container)

- **GIVEN** a container collector's config was extracted to a temp file, patched, and copied back
- **WHEN** the user confirms
- **THEN** `Copying updated config into container <name>...` is printed before the restart
- **THEN** `<runtime> cp <tmpPath> <name>:<containerPath>` is executed
- **THEN** `<runtime> restart <name>` is executed
- **THEN** `Container <name> restarted.` is printed

#### Scenario: No running collector found for the patched config

- **GIVEN** the config was patched on disk
- **AND** no running collector (native or container) owns the patched config
- **WHEN** the update completes
- **THEN** "No running collector found — config will be updated on disk only." is printed
- **THEN** no restart is attempted

## REMOVED Requirements

### Requirement: Generated config ports must not conflict with already-running collectors

**Reason**: This requirement described port-conflict detection for generating a new collector config, but that
behavior belongs to `install otel`'s config generation, not to this capability's config-patching flow, which never
generates a new config or allocates ports. The requirement was also incomplete on its own terms: it only covered the
Prometheus metrics port's `localhost` binding and missed that the `otlp`/`health_check` receivers bind `0.0.0.0`,
which does not conflict with a `localhost`-only probe on most systems.

**Migration**: See the `otel-collector-port-allocation` capability, which covers this behavior where it actually
lives, with the missed case corrected.
