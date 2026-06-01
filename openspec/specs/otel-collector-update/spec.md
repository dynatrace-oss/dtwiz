# Spec: OTel Collector Update (Config Patching)

## Overview

`dtwiz update otel` patches an existing OTel Collector config file to add the Dynatrace
OTLP exporter, then restarts the affected collector. When `--config` is omitted, a
running-collector picker is shown so the user can select which instance to patch.

---

## Requirements

### Requirement: When `--config` is omitted, show running collector picker

When `UpdateOtelConfig` is called with an empty `configPath`, it SHALL discover all
running OTel Collector processes on the host (both Dynatrace and upstream distributions)
and present them in a numbered selection list. The user picks one; its detected config
path is used as the config to patch.

The `--config` flag default is empty — the picker is the primary interaction path.

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

#### Scenario: No collectors are running and `--config` is omitted

- **GIVEN** no OTel Collector processes are running
- **WHEN** `dtwiz update otel` is run without `--config`
- **THEN** "No running OTel Collectors found." is printed
- **THEN** the command exits with an error: "no running OTel Collectors found — use --config to specify the config file path"

#### Scenario: Collector is running but config path cannot be detected

- **GIVEN** a collector process is running without a `--config` flag in its command line
- **WHEN** the user selects that collector
- **THEN** the command exits with an error: "could not determine config path for the selected collector — use --config to specify it"

#### Scenario: User cancels at the picker

- **GIVEN** the picker is shown
- **WHEN** the user enters 0
- **THEN** the command exits with `ErrInstallCancelled` (exit code 0, no error printed)

---

### Requirement: When `--config` is provided, validate file and find matching running collector

When `UpdateOtelConfig` is called with a non-empty `configPath`, it SHALL validate that
the file exists and then search for a running collector process whose detected config path
resolves to the same absolute path. The matching collector is used for the restart step.

#### Scenario: Config file exists and a matching running collector is found

- **GIVEN** `--config /etc/otel/config.yaml` is passed
- **AND** a running collector process has `--config /etc/otel/config.yaml` in its command line
- **WHEN** `dtwiz update otel --config /etc/otel/config.yaml` is run
- **THEN** the picker is skipped
- **THEN** the config is patched and the matched collector is restarted

#### Scenario: Config file exists but no matching running collector

- **GIVEN** `--config /etc/otel/config.yaml` is passed
- **AND** no running collector has that config path
- **WHEN** `dtwiz update otel --config /etc/otel/config.yaml` is run
- **THEN** the config is patched on disk
- **THEN** "No running collector found — config will be updated on disk only." is printed
- **THEN** no restart is attempted

#### Scenario: Config file does not exist

- **GIVEN** `--config /nonexistent/config.yaml` is passed
- **WHEN** `dtwiz update otel --config /nonexistent/config.yaml` is run
- **THEN** the command exits with error: "config file not found: /nonexistent/config.yaml"

---

### Requirement: Relative config paths from running processes are resolved against the process CWD

When a running collector's config path (parsed from its command line) is relative, it
SHALL be resolved against that process's working directory — not dtwiz's working directory.

On Linux, the process CWD is read from `/proc/<pid>/cwd`. On macOS, `lsof` is used as a
fallback. On Windows, relative config paths remain unresolved (WMI does not expose CWD;
this is an acceptable limitation because Windows collectors typically use absolute paths).

#### Scenario: Collector started with relative config path on Linux/macOS

- **GIVEN** a collector was launched as `./otelcol --config config.yaml` from `/opt/otel/`
- **WHEN** `findAllRunningOtelCollectors()` discovers the process
- **THEN** the detected config path is `/opt/otel/config.yaml` (resolved against the process CWD)

---

### Requirement: All running OTel Collector distributions are shown in the picker

The picker SHALL include both Dynatrace and upstream OTel Collector distributions.
The binary name patterns used are: `dynatrace-otel-collector`, `otelcorecol`, `otelcol`,
`opentelemetry-collector`. These are substring matches against the binary base name
(case-insensitive). `otelcol-contrib` is implicitly matched by the `otelcol` pattern
and does not require a separate entry.

The `otelcorecol` pattern SHALL be listed separately from `otelcol` because
`"otelcol"` is not a substring of `"otelcorecol"` — without the explicit entry,
`otelcorecol` binaries would be missed.

#### Scenario: `otelcorecol` binary is running

- **GIVEN** a process named `otelcorecol_darwin_arm64` is running
- **WHEN** `findAllRunningOtelCollectors()` scans
- **THEN** the process is included in the result list (matched by the explicit `otelcorecol` pattern, not by `otelcol`)
