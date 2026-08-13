# Spec: OTel Collector Update (Config Patching)

## Purpose

`dtwiz update otel` patches an existing OTel Collector config file to add the Dynatrace
OTLP exporter, then restarts the affected collector. When `--config` is omitted, a
running-collector picker is shown so the user can select which instance to patch.

---

## Requirements

### Requirement: When `--config` is omitted, show running collector picker

When `dtwiz update otel` is run without `--config`, `UpdateOtelConfigInteractive` SHALL be called. It SHALL discover all running OTel Collector processes (Dynatrace and upstream, including containers) and present them in a numbered selection list. `UpdateOtelConfig` SHALL return an error when called with an empty `configPath`; callers without a known path SHALL use `UpdateOtelConfigInteractive`.

#### Scenario: One collector is running with a detectable config

- **GIVEN** one OTel Collector process is running with `--config /etc/otel/config.yaml`
- **WHEN** `dtwiz update otel` is run without `--config`
- **THEN** the picker shows one entry with the collector binary name, PID, and config path
- **THEN** after selection, `/etc/otel/config.yaml` is used as the config file to patch

#### Scenario: Multiple collectors are running

- **GIVEN** two OTel Collector processes are running
- **WHEN** `dtwiz update otel` is run without `--config`
- **THEN** both collectors are listed with their binary name, PID, and config path (if known)
- **THEN** the user selects one and patching proceeds for that collector's config

#### Scenario: A container-based collector is running

- **GIVEN** a container named `my-otel-collector` is running via docker with image `otel/opentelemetry-collector-contrib`
- **WHEN** `dtwiz update otel` is run without `--config`
- **THEN** the picker includes an entry for that container
- **THEN** its status is shown as `container (docker)` (not a PID)
- **THEN** its display name is the container name (`my-otel-collector`)
- **THEN** if the config is bind-mounted from the host, the host path is shown as the config path
- **THEN** if the config is inside the container only, it is shown as `<path> (inside container, not host-mounted)`

#### Scenario: No collectors are running and `--config` is omitted

- **GIVEN** no OTel Collector processes are running and no OTel Collector containers are running
- **WHEN** `dtwiz update otel` is run without `--config`
- **THEN** "No running OTel Collectors found." is printed
- **THEN** the command exits with an error: "no running OTel Collectors found — use --config to specify the config file path"

#### Scenario: Native collector is running but config path cannot be detected

- **GIVEN** a native collector process is running without a `--config` flag in its command line
- **AND** it is not a container-based collector
- **WHEN** the user selects that collector
- **THEN** the command exits with an error: "could not determine config path for the selected collector — use --config to specify it"

#### Scenario: Container collector has config inside the container (not host-mounted)

- **GIVEN** a container collector is running with a config file accessible only inside the container
- **AND** no `--config` flag is passed
- **WHEN** the user selects that collector
- **THEN** the config is extracted from the container to a temporary host file
- **THEN** `Extracted config from container (<container-path> → temp file)` is printed
- **THEN** the temporary file is used for patching
- **THEN** after patching, the file is copied back into the container and the container is restarted
- **THEN** the temp file is deleted after the operation

#### Scenario: User cancels at the picker

- **GIVEN** the picker is shown
- **WHEN** the user enters 0
- **THEN** the command exits with `ErrInstallCancelled` (exit code 0, no error printed)

---

### Requirement: When `--config` is provided, validate file and find matching running collector

When `UpdateOtelConfig` is called with a non-empty `configPath`, it SHALL validate that the file exists and then search for a running collector whose detected host-accessible config path resolves to the same absolute path. Container collectors SHALL be included when their config is bind-mounted from the host. Containers whose config is inside the container SHALL only be matched via the picker, not via `--config`.

#### Scenario: Config file exists and a matching running native collector is found

- **GIVEN** `--config /etc/otel/config.yaml` is passed
- **AND** a running collector process has `--config /etc/otel/config.yaml` in its command line
- **WHEN** `dtwiz update otel --config /etc/otel/config.yaml` is run
- **THEN** the picker is skipped
- **THEN** the config is patched and the matched collector is restarted

#### Scenario: Config file exists and a matching container with host-mounted config is found

- **GIVEN** `--config /host/otel/config.yaml` is passed
- **AND** a running container has `/host/otel/config.yaml` bind-mounted as its collector config
- **WHEN** `dtwiz update otel --config /host/otel/config.yaml` is run
- **THEN** the picker is skipped
- **THEN** the config is patched on disk
- **THEN** the matched container is restarted via `<runtime> restart <name>`
- **THEN** the container name and runtime are shown in the restart preview: `• <name>  (<runtime> restart)`

#### Scenario: Config file exists but no matching running collector

- **GIVEN** `--config /etc/otel/config.yaml` is passed
- **AND** no running collector has that config path
- **WHEN** `dtwiz update otel --config /etc/otel/config.yaml` is run
- **THEN** the config is patched on disk
- **THEN** "No running collector found — config will be updated on disk only." is printed
- **THEN** no restart is attempted

#### Scenario: `UpdateOtelConfig` called with empty path

- **GIVEN** `UpdateOtelConfig` is called programmatically with an empty `configPath`
- **THEN** the function returns immediately with error: "config path must not be empty — use --config or UpdateOtelConfigInteractive"

#### Scenario: Config file does not exist

- **GIVEN** `--config /nonexistent/config.yaml` is passed
- **WHEN** `dtwiz update otel --config /nonexistent/config.yaml` is run
- **THEN** the command exits with error: "config file not found: /nonexistent/config.yaml"

#### Scenario: Config path cannot be resolved to an absolute path

- **GIVEN** `--config` is provided with a relative path and the working directory cannot be determined
- **WHEN** `dtwiz update otel --config <path>` is run
- **THEN** the command exits with error: "failed to resolve config path: ..."

---

### Requirement: Relative config paths from running processes are resolved against the process CWD

When a running collector's config path is relative, it SHALL be resolved against that process's working directory, not dtwiz's working directory. On Linux the process CWD SHALL be read from `/proc/<pid>/cwd`; on macOS `lsof` SHALL be used as a fallback. On Windows relative config paths remain unresolved.

#### Scenario: Collector started with relative config path on Linux/macOS

- **GIVEN** a collector was launched as `./otelcol --config config.yaml` from `/opt/otel/`
- **WHEN** `findAllRunningOtelCollectors()` discovers the process
- **THEN** the detected config path is `/opt/otel/config.yaml` (resolved against the process CWD)

#### Scenario: Collector started with a quoted config path containing spaces

- **GIVEN** a collector was launched as `otelcol.exe --config "C:\Program Files\otelcol\config.yaml"`
- **WHEN** `findAllRunningOtelCollectors()` discovers the process
- **THEN** the detected config path is `C:\Program Files\otelcol\config.yaml` (quotes stripped, path preserved intact)

---

### Requirement: Generated config ports must not conflict with already-running collectors

When generating a new collector config, `dtwiz` SHALL probe `localhost:<port>` to find a
free port for the Prometheus metrics endpoint (which also binds on `localhost`). On macOS,
probing on `0.0.0.0` does not detect a conflict when `localhost:<port>` is already taken
by another process — the probe would succeed yet the collector would fail to start.

#### Scenario: Another collector already occupies the default Prometheus metrics port

- **GIVEN** a collector process is running and has bound `localhost:8888` for its Prometheus metrics endpoint
- **WHEN** a new collector config is generated
- **THEN** `findFreePort(8888)` probes `localhost:8888`, detects the conflict, and selects the next available port (e.g. 8889)
- **THEN** the generated config contains the conflict-free port
- **THEN** the new collector starts successfully

---

### Requirement: Collector restart after patching

After patching the config file, any running collector that owns the patched config SHALL be
restarted. The restart mechanism differs between native processes and containers.

For **native processes**: the old process is killed and the binary is re-launched with
the updated config. The restarted collector is verified against Dynatrace.

For **containers**: the patched config is already on the host filesystem (host-mounted
case) or has been copied back into the container (extract/copy-back case). The container
is then restarted via `<runtime> restart <name>`. Verification is attempted best-effort —
port 4318 may or may not be exposed to the host depending on the container's port mapping.

#### Scenario: Native collector restarted after patch

- **GIVEN** a native OTel Collector process was matched and the config was patched
- **WHEN** the user confirms
- **THEN** the old process is killed and the collector is restarted with the updated config
- **THEN** the restart is verified against Dynatrace
- **THEN** `✓ Collector restarted and verified.` is printed on success

#### Scenario: Container restarted after patch (host-mounted config)

- **GIVEN** a container collector matched the patched config via a host-mounted path
- **WHEN** the user confirms
- **THEN** `Restarting container <name>...` is printed
- **THEN** `<runtime> restart <name>` is executed
- **THEN** `Container <name> restarted.` is printed
- **THEN** Dynatrace verification is attempted and a warning is printed on failure

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

---

### Requirement: All running OTel Collector distributions are shown in the picker

The picker SHALL include both Dynatrace and upstream OTel Collector distributions,
including container-based collectors detected via docker/podman/nerdctl.

For native processes, the binary name patterns are defined in the shared
`otelCollectorNames` slice: `otelcorecol`, `otel-collector`, `otelcol`, `otelcol-contrib`,
`opentelemetry-collector`, `dynatrace-otel-collector`. This single list is used for both
exact process name matching (`pgrep -x`, `Get-Process`) and command-line substring
searches (`pgrep -f`, WMI `CommandLine`).

`otelcorecol` and `otelcol-contrib` are listed explicitly because neither is a substring
of the other patterns — without separate entries they would be missed by substring search.

For containers, any running container whose image name or container name matches the
pattern `otel.+collector` or `opentelemetry.+collector` (case-insensitive regex) is
included. Container runtimes probed (in order): `docker`, `podman`, `nerdctl`.

#### Scenario: `otelcorecol` binary is running

- **GIVEN** a process named `otelcorecol_darwin_arm64` is running
- **WHEN** `findAllRunningOtelCollectors()` scans
- **THEN** the process is included in the result list (matched by the explicit `otelcorecol` entry in `otelCollectorNames`)

#### Scenario: Container collector found via podman

- **GIVEN** a podman container with image `ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib:latest` is running
- **WHEN** `findAllRunningOtelCollectors()` scans
- **THEN** the container is included in the result with `containerRuntime = "podman"`
- **THEN** its `binaryPath` is the image name and `containerName` is the container name

---

### Requirement: Dynatrace OTel Collector config is regenerated from the install template

When the selected or matched running collector is identified as a Dynatrace OTel Collector
(its binary path contains `dynatrace-otel-collector`), the update SHALL regenerate the
full collector config from the install template with the current tenant credentials and
receiver ports, rather than patching the exporter in as an additional entry.

Existing OTLP receiver port assignments are preserved from the current config so that
connected app services keep their OTLP endpoint after the restart. The ports read are:

- `receivers.otlp.protocols.grpc.endpoint` (default 4317)
- `receivers.otlp.protocols.http.endpoint` (default 4318)
- `service.telemetry.metrics.readers[0].pull.exporter.prometheus.port` (default 8888)
- `extensions.health_check.endpoint` (default 13133; included only when `--experimental`)

If the freshly rendered config is byte-identical to the existing file, the update prints
`Collector configuration is up to date.` and returns `ErrUpToDate`. The `cmd/update.go`
handler treats `ErrUpToDate` the same as `ErrInstallCancelled` — clean exit, no error
printed, and `WatchIngest` is not called.

After a successful restart of a Dynatrace collector, `WatchIngest()` is called to poll
until new telemetry data appears in Dynatrace.

#### Scenario: Dynatrace collector with current credentials selected

- **GIVEN** a running `dynatrace-otel-collector` process is selected
- **AND** the existing config already reflects the current tenant and token
- **WHEN** `dtwiz update otel` is run
- **THEN** `Collector configuration is up to date.` is printed
- **THEN** the command exits cleanly (`ErrUpToDate`), no error is printed, and `WatchIngest` is not called

#### Scenario: Dynatrace collector with outdated credentials selected

- **GIVEN** a running `dynatrace-otel-collector` process is selected
- **AND** the existing config has a stale tenant URL or token
- **WHEN** the user confirms the preview
- **THEN** the config is regenerated from the template with the current tenant credentials
- **THEN** the existing OTLP receiver ports are preserved in the new config
- **THEN** the collector is restarted and verified
- **THEN** `WatchIngest()` is called to confirm data arrives in Dynatrace

---

### Requirement: Connected app services are detected, shown in the preview, and restarted

Before prompting for confirmation, the update flow SHALL detect app processes associated
with the collector being updated and display them in the preview. After the collector is
restarted, the detected services are restarted with a reconciled or retargeted environment.

**Detection methods** (both applied; results deduplicated by PID):

1. **TCP connection to OTLP ports**: processes with an active TCP connection to any of the
   collector's OTLP receiver ports (default 4317 and 4318; actual ports read from the
   collector config). Implemented via `lsof(1)` on Unix. The OTLP HTTP exporter closes
   the connection after each export batch, so short-lived connections are included (no TCP
   state filter applied).

2. **OTLP env-var tenant match**: OTel-instrumented processes with
   `OTEL_EXPORTER_OTLP_ENDPOINT` (or any per-signal variant) pointing to the same
   Dynatrace tenant as any configured collector exporter. Implemented via `ps -eww` on
   Unix. On Windows this method is a no-op because reading process environment variables
   requires elevated `ReadProcessMemory` privileges.

The collector's own PID(s) and dtwiz's own PID are excluded from results. `DT_ENVIRONMENT`
is intentionally excluded from the OTLP env-var search to avoid false matches against
dtwiz and its children, which inherit that variable.

**Preview output** for each connected service: PID, binary name, listen ports (if any),
and the OTLP export endpoint from the process environment (if matched by tenant).

**Restart behavior** differs by update path:

- **Upstream OTel Collector update** (`updateOtelConfig`): services are restarted with
  their environment reconciled — `OTEL_EXPORTER_OTLP_ENDPOINT` and the Authorization
  header are updated to match `DT_ENVIRONMENT` + `DT_PLATFORM_TOKEN` (correcting stale
  tenant routing). Per-signal endpoint overrides (`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`,
  etc.) are removed. Services already routing to a loopback endpoint are left as-is.

- **Dynatrace Collector update** (`updateDynatraceCollector`): services that currently
  export directly to a Dynatrace tenant (non-loopback endpoint) are retargeted to route
  through the local collector HTTP endpoint (`http://localhost:<httpPort>`), so all data
  reaches every configured export destination. Per-signal endpoint overrides are removed.
  Services already routing via the collector (loopback) are left as-is.

**Stop/restart mechanism**: each service is stopped via SIGTERM (with SIGKILL fallback)
and relaunched detached with its original command line, working directory, and environment.
If any service fails to restart, a message is printed and the user is advised to start it
manually. A service restart failure does not abort the overall update.

#### Scenario: App service connected to collector via OTLP port

- **GIVEN** a Node.js app is running with an active TCP connection to `localhost:4318`
- **AND** the collector's config uses OTLP HTTP on port 4318
- **WHEN** the update preview is shown
- **THEN** the service is listed under `Connected services` with its PID and binary name
- **THEN** after the collector is restarted, the service is also restarted

#### Scenario: Dynatrace Collector update retargets a direct-to-tenant service

- **GIVEN** a running app has `OTEL_EXPORTER_OTLP_ENDPOINT=https://<tenant>.live.dynatrace.com/api/v2/otlp` in its environment
- **AND** the collector's exporter targets the same Dynatrace tenant
- **WHEN** the Dynatrace collector config is regenerated and the collector is restarted
- **THEN** the app service is restarted with `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:<httpPort>` (routed via local collector)

#### Scenario: Upstream OTel update reconciles stale export environment

- **GIVEN** a running app has `OTEL_EXPORTER_OTLP_ENDPOINT` pointing to an old Dynatrace tenant
- **AND** `DT_ENVIRONMENT` in the app's environment points to the current tenant
- **WHEN** the upstream OTel Collector is patched and restarted
- **THEN** the app service is restarted with `OTEL_EXPORTER_OTLP_ENDPOINT` updated to the current tenant's OTLP endpoint
- **THEN** per-signal endpoint overrides are removed from the restarted service's environment

#### Scenario: No connected services found

- **GIVEN** no app processes have active OTLP connections to the collector
- **AND** no app processes export to the same Dynatrace tenant as the collector
- **WHEN** the update flow runs
- **THEN** the connected services section is omitted from the preview
- **THEN** no service restarts are attempted after the collector is restarted
