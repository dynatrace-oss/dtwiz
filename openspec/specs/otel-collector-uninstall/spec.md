# Spec: OTel Collector Uninstall

## Overview

`dtwiz uninstall otel` manages the removal of Dynatrace OTel Collector instances installed
by dtwiz. When multiple instances are present the user selects which to remove via an
interactive numbered list before any destructive action is taken.

---

## Requirements

### Requirement: Discover Dynatrace OTel Collector instances before prompting

`UninstallOtelCollector()` SHALL discover all Dynatrace OTel Collector instances on the
host — both currently running processes and binaries present in well-known dtwiz install
directories (`~/opentelemetry`, `./opentelemetry`) that are not currently running. Only
Dynatrace-branded collectors (`dynatrace-otel-collector` in the binary name) are included.
Non-Dynatrace upstream collectors are explicitly excluded.

#### Scenario: One running Dynatrace collector found

- **GIVEN** one `dynatrace-otel-collector` process is running
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** a numbered list with that single collector is shown with its PID and binary path
- **THEN** a "Cancel [0]" option is shown below the list

#### Scenario: Multiple collectors found (running + installed)

- **GIVEN** two Dynatrace collector processes are running and one additional binary exists
  in `~/opentelemetry` that is not running
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** all three instances are listed with their status (`PID N` or `not running`)
- **THEN** an "Uninstall all [N]" option is shown in addition to individual options

#### Scenario: Container-based Dynatrace collector is running

- **GIVEN** a `dynatrace-otel-collector` container is running via docker
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the container appears in the selection list with status `container (docker)`
- **THEN** the display name is the container name (not the image path)
- **THEN** if the config is bind-mounted from the host, it is shown as the config path
- **THEN** if the config is only inside the container, it is shown as `<path> (inside container, not host-mounted)`

#### Scenario: No Dynatrace collectors found

- **GIVEN** no Dynatrace OTel Collector processes are running and no binaries exist in
  the well-known install directories
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the command skips the selection prompt and proceeds to check for runtime
  instrumentation artifacts (Node.js, Python, Java)

#### Scenario: Non-Dynatrace collectors are running alongside a Dynatrace collector

- **GIVEN** an upstream `otelcol` or `otelcol-contrib` process is running
- **AND** at least one Dynatrace-branded collector is also running
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the non-Dynatrace collector is NOT listed in the selection UI
- **THEN** the note "Only Dynatrace OTel Collectors are shown here" is printed as a preamble to the selection list

#### Scenario: Only non-Dynatrace collectors are running

- **GIVEN** an upstream `otelcol` or `otelcol-contrib` process is running
- **AND** no Dynatrace-branded collector is running or installed in well-known directories
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the selection prompt is skipped entirely (no selection UI is shown, no note is printed)
- **THEN** the command proceeds directly to check for runtime instrumentation artifacts

---

### Requirement: Interactive selection of which collector to uninstall

When Dynatrace collectors are found, the user SHALL be prompted to select which instance
to remove before any action is taken.

#### Scenario: User selects a single collector

- **GIVEN** two Dynatrace collectors are listed
- **WHEN** the user enters the number of one collector
- **THEN** only that collector is selected for removal; the other is left untouched

#### Scenario: User selects "Uninstall all" (multiple collectors)

- **GIVEN** multiple Dynatrace collectors are listed and the "Uninstall all" option is shown
- **WHEN** the user enters the "Uninstall all" number
- **THEN** all listed collectors are selected for removal

#### Scenario: User selects Cancel

- **GIVEN** the selection prompt is shown
- **WHEN** the user enters 0
- **THEN** "Uninstall cancelled." is printed and the command exits with code 0 (no error)

#### Scenario: User enters an invalid number

- **GIVEN** the selection prompt is shown with N options
- **WHEN** the user enters a number outside the valid range or non-numeric input
- **THEN** the command exits with `ErrInstallCancelled` — treated as cancellation, exit code 0

#### Scenario: Stdin is closed unexpectedly

- **GIVEN** the selection prompt is waiting for input
- **WHEN** stdin is closed (e.g. piped input exhausted, EOF)
- **THEN** the underlying scanner error is propagated as a real error (not silently swallowed)

---

### Requirement: Uninstall preview before destructive action

After selection, the standard uninstall preview SHALL be shown before prompting for
confirmation, consistent with the existing preview format for processes and directories.

The `collectorInstance` selected by the user is converted to `otelProcessInfo` via
`collectorToProcessInfo`, which propagates `containerRuntime` and `containerName` fields
so that the uninstall execution layer can handle both native processes and containers.

#### Scenario: Selected collector is running

- **GIVEN** the user selects a running collector (PID > 0)
- **WHEN** the preview is rendered
- **THEN** the process is listed under "Processes that will be killed" with its PID
- **THEN** its install directory is listed under "Directories that will be removed"

#### Scenario: Selected collector is installed but not running

- **GIVEN** the user selects an installed-but-not-running collector (PID = 0)
- **WHEN** the preview is rendered
- **THEN** no kill line appears (nothing to kill)
- **THEN** its install directory is listed under "Directories that will be removed"

#### Scenario: Selected collector is a container

- **GIVEN** the user selects a container-based collector
- **WHEN** `collectorToProcessInfo` converts it
- **THEN** the resulting `otelProcessInfo` has `containerRuntime` and `containerName` set
- **THEN** `pid` is 0 (containers have no native PID in dtwiz's process model)

---

### Requirement: Dry-run skips execution but shows full preview

When `--dry-run` is passed, the selection prompt is still shown and the preview is
rendered, but no processes are killed and no files are removed.

#### Scenario: Dry-run with a collector selected

- **GIVEN** `dtwiz uninstall otel --dry-run`
- **WHEN** the user selects a collector
- **THEN** the full preview is printed
- **THEN** "[dry-run] No changes made." is printed
- **THEN** no process is killed and no directory is removed

---

### Requirement: `ErrInstallCancelled` from uninstall is not treated as a CLI error

The `cmd/uninstall.go` handler SHALL treat `ErrInstallCancelled` returned by
`UninstallOtelCollector` as a graceful exit (exit code 0), not as a command error.

#### Scenario: User cancels at the selection prompt

- **GIVEN** `dtwiz uninstall otel` is run interactively
- **WHEN** the user enters 0 at the selection prompt
- **THEN** the command exits with code 0 and prints "Uninstall cancelled."
- **THEN** no error message is printed to stderr
