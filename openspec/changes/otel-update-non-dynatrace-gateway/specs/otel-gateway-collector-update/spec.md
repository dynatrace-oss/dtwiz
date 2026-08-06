# OTel Gateway Collector Update Spec

## ADDED Requirements

### Requirement: Non-Dynatrace collector selection triggers the gateway flow

When a user runs `update otel` and selects a collector that is not the Dynatrace distro, dtwiz SHALL follow the gateway flow described by this spec instead of the existing direct-merge path used for Dynatrace collectors.

#### Scenario: Dynatrace collector selection is unaffected

- **GIVEN** a user runs `update otel` and selects a running `dynatrace-otel-collector` process
- **WHEN** dtwiz classifies the selection
- **THEN** the existing direct-merge update path SHALL be used, unchanged by this spec

#### Scenario: Non-Dynatrace collector selection enters the gateway flow

- **GIVEN** a user runs `update otel` and selects a running collector identified as a non-Dynatrace distro
- **WHEN** dtwiz classifies the selection
- **THEN** dtwiz SHALL proceed with config-source validation as the first step of the gateway flow

### Requirement: Config source must be a single writable file before any change is applied

Before creating a backup, detecting a supervisor, or deploying anything, dtwiz SHALL verify that the selected collector's effective configuration resolves to a single, writable, local YAML file.

#### Scenario: Config source is a single writable file

- **GIVEN** the selected collector was started with exactly one `--config <path>` argument
- **AND** the path is a local file writable by the current user
- **WHEN** dtwiz validates the config source
- **THEN** validation SHALL pass and the flow SHALL continue to backup and supervisor detection

#### Scenario: Config source is not a single writable file

- **GIVEN** the selected collector was started with multiple `--config` arguments, an `env:`/`yaml:` inline config provider, a config baked into a container image with no durable write-back path, or an unwritable config path
- **WHEN** dtwiz validates the config source
- **THEN** validation SHALL fail
- **AND** dtwiz SHALL make no filesystem or process changes of any kind — no backup, no gateway collector deployment, no config write
- **AND** dtwiz SHALL show a docs link explaining how to manually add the Dynatrace exporter and configure host monitoring, then stop

### Requirement: Backup precedes any config write

When config-source validation passes, dtwiz SHALL create a timestamped backup of the current config before writing any changes, and SHALL print the backup's path to the user.

#### Scenario: Backup created and path printed

- **GIVEN** config-source validation has passed
- **WHEN** dtwiz proceeds with the update
- **THEN** a timestamped backup of the current config SHALL be created before any write
- **AND** its path SHALL be printed to the user

### Requirement: A dedicated Dynatrace Gateway Collector is deployed with host monitoring

dtwiz SHALL deploy a new, dedicated Dynatrace OTel Collector — distinct from the selected foreign collector and from dtwiz's own app-monitoring managed collector — configured with host monitoring, listening on a local port.

#### Scenario: Gateway collector deployed before the foreign config is touched

- **GIVEN** config-source validation and backup have completed
- **WHEN** dtwiz proceeds with the gateway flow
- **THEN** dtwiz SHALL install and start a dedicated Dynatrace Gateway Collector, configured with host monitoring, before writing any change to the foreign collector's config
- **AND** dtwiz SHALL confirm the gateway collector's receiver is accepting connections before proceeding

#### Scenario: Gateway deployment failure aborts the flow

- **GIVEN** the gateway collector fails to install or start
- **WHEN** dtwiz detects this failure
- **THEN** dtwiz SHALL abort the flow without writing any change to the foreign collector's config

### Requirement: Foreign collector config receives only an additive forwarding exporter

The patch applied to the foreign collector's config SHALL consist of exactly one new exporter definition, with no authentication secrets, appended to the existing pipelines' exporter lists. No existing receiver, processor, pipeline, or exporter SHALL be modified or removed, and no new receiver, processor, or pipeline SHALL be added.

#### Scenario: Additive-only patch

- **GIVEN** the gateway collector is running and ready
- **WHEN** dtwiz patches the foreign collector's config
- **THEN** exactly one new exporter definition SHALL be added, pointing at the gateway collector's local address, with no authorization header or secret
- **AND** the new exporter's name SHALL be appended to every existing pipeline's `exporters` list
- **AND** every existing receiver, processor, pipeline, and exporter SHALL remain unchanged

#### Scenario: Patch validated as additive before writing

- **WHEN** dtwiz generates the foreign-config patch
- **THEN** dtwiz SHALL verify the resulting diff contains only new nodes and list-appends
- **AND** SHALL NOT write the config if any existing line would be removed or modified

### Requirement: Restart is attempted only when it can be done safely

dtwiz SHALL determine, from the detected process supervisor, whether it can safely restart the foreign collector itself, and SHALL only attempt a direct process kill when it will not conflict with an external supervisor.

#### Scenario: Systemd-supervised collector is restarted via systemctl

- **GIVEN** the foreign collector runs under a systemd unit
- **WHEN** dtwiz restarts it
- **THEN** dtwiz SHALL use `systemctl restart <unit>` rather than killing the process directly

#### Scenario: Container-supervised collector is restarted via the container runtime

- **GIVEN** the foreign collector runs in a docker or podman container
- **WHEN** dtwiz restarts it
- **THEN** dtwiz SHALL use the container runtime's restart command rather than killing the process directly

#### Scenario: Bare process is restarted only with a fully captured launch context

- **GIVEN** the foreign collector is a bare/manual process with no detected supervisor
- **AND** dtwiz successfully captured its full original launch context (arguments, environment, working directory)
- **WHEN** dtwiz restarts it
- **THEN** dtwiz SHALL kill and relaunch the process using the captured launch context

#### Scenario: Kubernetes or undetermined supervisor is never auto-restarted

- **GIVEN** the foreign collector runs as a Kubernetes pod, or its supervisor could not be determined, or its full launch context could not be captured
- **WHEN** dtwiz reaches the restart step
- **THEN** dtwiz SHALL NOT attempt any automatic restart
- **AND** SHALL ask the user to restart the collector manually

### Requirement: Post-restart outcome depends on restart success

Following the restart step, dtwiz SHALL show live ingest confirmation only when it performed a successful automatic restart; in every other case it SHALL provide manual instructions and links to check ingest in the Dynatrace UI instead.

#### Scenario: Successful automatic restart shows live ingest confirmation

- **GIVEN** dtwiz attempted an automatic restart and the collector came back up successfully
- **WHEN** the restart completes
- **THEN** dtwiz SHALL run the ingest-watch flow to confirm data is flowing

#### Scenario: Failed automatic restart shows manual instructions

- **GIVEN** dtwiz attempted an automatic restart and it failed
- **WHEN** dtwiz detects the failure
- **THEN** dtwiz SHALL restore the pre-patch config backup automatically
- **AND** SHALL print manual restart instructions including the config path and the backup path
- **AND** SHALL show links to the Dynatrace Services and Distributed Tracing apps instead of running the ingest-watch flow

#### Scenario: Manual restart path shows the same instructions and links

- **GIVEN** dtwiz determined it cannot safely restart the collector automatically
- **WHEN** dtwiz reaches the end of the flow
- **THEN** dtwiz SHALL print manual restart instructions including the config path and the backup path
- **AND** SHALL show links to the Dynatrace Services and Distributed Tracing apps instead of running the ingest-watch flow

### Requirement: Gateway flow is gated behind the experimental flag

The entire non-Dynatrace gateway flow SHALL only run when `--experimental` or `DTWIZ_EXPERIMENTAL=true` is enabled, consistent with the existing gate on `update otel`.

#### Scenario: Flow inactive without the experimental flag

- **GIVEN** `--experimental` is not set and `DTWIZ_EXPERIMENTAL` is not `true`
- **WHEN** a user attempts to run `update otel`
- **THEN** the command SHALL behave exactly as it does today (hidden/erroring as currently implemented), with no part of this gateway flow executed
