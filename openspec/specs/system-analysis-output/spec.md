# Spec: System Analysis Output

## Purpose

`dtwiz analyze` and `dtwiz status` print a System Analysis block summarising what was
detected on the host. This spec covers which commands print the block, the labels,
ordering, and stability rules for that block so that terminology is consistent across
a single screen and the machine-readable JSON output remains stable regardless of
label changes.

---

## Requirements

### Requirement: The System Analysis block is printed only by `dtwiz analyze` and `dtwiz status`

The System Analysis block (`info.Summary()`) SHALL be printed by `dtwiz analyze` and
`dtwiz status` only. `dtwiz setup` SHALL NOT print the System Analysis block; detection
context is instead surfaced inline on each recommendation menu entry.

#### Scenario: `dtwiz analyze` prints the System Analysis block

- **GIVEN** the user runs `dtwiz analyze`
- **WHEN** analysis completes
- **THEN** the full System Analysis block is printed to stdout

#### Scenario: `dtwiz status` prints the System Analysis block

- **GIVEN** the user runs `dtwiz status`
- **WHEN** analysis completes
- **THEN** the full System Analysis block is printed to stdout

#### Scenario: `dtwiz setup` does NOT print the System Analysis block

- **GIVEN** the user runs `dtwiz setup`
- **WHEN** analysis completes and the recommendation menu is shown
- **THEN** no System Analysis block is printed; detection context appears inline on each menu entry instead

---

### Requirement: The host line is labelled `This host`

The System Analysis block SHALL label the operating-system and hostname line `This host:`.
The word "platform" SHALL NOT be used as a label for the operating system, so that within
a single screen it refers only to the Dynatrace platform.

#### Scenario: Host line rendered

- **GIVEN** a system analysis has been produced
- **WHEN** the System Analysis block is rendered
- **THEN** the first line is labelled `This host:` and carries the operating system, architecture, and hostname

#### Scenario: Host line does not collide with credential labels

- **GIVEN** a command renders both credential status and the System Analysis block in one output
- **WHEN** that output is rendered
- **THEN** the only label containing the word "Platform" is the credential label, and the operating-system line is labelled `This host:`

---

### Requirement: Monitoring-status lines are grouped directly below the host line

The System Analysis block SHALL render the OpenTelemetry line and the OneAgent line
consecutively, immediately after the host line and before any container, orchestrator,
or cloud-provider line. These are the lines reporting whether the host is already monitored,
and they determine which recommendations are offered.

#### Scenario: Both monitoring-status lines present

- **GIVEN** a system analysis has been produced
- **WHEN** the System Analysis block is rendered
- **THEN** the OpenTelemetry line is rendered second and the OneAgent line third
- **THEN** the Docker, Kubernetes, and cloud-provider lines follow them

#### Scenario: Neither monitoring method is present

- **GIVEN** no OpenTelemetry Collector is running and no OneAgent is installed
- **WHEN** the System Analysis block is rendered
- **THEN** both lines are still rendered in the same positions, each reporting that nothing was detected

#### Scenario: OneAgent unavailable for the platform

- **GIVEN** the analysis runs on a platform where OneAgent cannot be installed
- **WHEN** the System Analysis block is rendered
- **THEN** the OneAgent line reports that nothing was detected together with the reason, in position three

---

### Requirement: Detected runtimes are labelled `Runtimes` and rendered last

The System Analysis block SHALL label the detected-runtimes line `Runtimes:` and render it
as the final line. The label SHALL NOT be `Services:`, because the underlying detection
reports runtimes available on the executable search path rather than running workloads.
Rendering it last reflects that no recommendation is derived from it.

#### Scenario: Runtimes detected

- **GIVEN** one or more runtimes are detected on the host
- **WHEN** the System Analysis block is rendered
- **THEN** the final line is labelled `Runtimes:` and lists the detected runtime names

#### Scenario: No runtimes detected

- **GIVEN** no runtimes are detected on the host
- **WHEN** the System Analysis block is rendered
- **THEN** the final line is labelled `Runtimes:` and reports that nothing was detected

---

### Requirement: Analysis labels are presentation-only

Renaming or reordering lines in the System Analysis block SHALL NOT change the
machine-readable analysis output. Field names in the JSON representation SHALL remain
stable regardless of how the block is labelled.

#### Scenario: JSON output unaffected by label changes

- **GIVEN** the System Analysis block labels the operating-system line `This host:`
- **WHEN** the analysis is requested in JSON form
- **THEN** the operating-system field is still emitted under its original key name
