# install-step-output Specification

## Purpose

Define how install flows present progress, verbose detail and failures: one line per step with a spinner, wordy output behind `-v`, and failures always visible.

## Requirements

### Requirement: Steps render as one line

Install flows SHALL present each unit of work as a single step line that collapses to `✓ <label>` on success or `✗ <label>` on failure.

#### Scenario: Step runs on a terminal

- **GIVEN** stdout is a terminal and neither `-v` nor `--debug` is set
- **WHEN** a step is running
- **THEN** a spinner with the step label and optional detail is shown on one line
- **AND** the line is replaced by `✓ <label>` when the step succeeds

#### Scenario: Step runs without a terminal

- **GIVEN** stdout is not a terminal
- **WHEN** a step completes
- **THEN** only the final `✓` or `✗` line is printed, without spinner control sequences

#### Scenario: Verbose mode keeps the spinner

- **GIVEN** `-v` or `--debug` is set and stdout is a terminal
- **WHEN** a step runs
- **THEN** the spinner is still shown so the current step is visible
- **AND** log lines clear the spinner line before they are written

### Requirement: Wordy output requires verbose mode

Commands, download URLs, file paths, generated manifests and configuration dumps SHALL only be printed with `-v` or `--debug`, or in the `--dry-run` plan.

#### Scenario: Default preview

- **GIVEN** `dtwiz install kubernetes` is run without `-v`
- **WHEN** the preview is shown before confirmation
- **THEN** it contains a short summary and no manifest or command list

#### Scenario: Verbose preview

- **GIVEN** `dtwiz install kubernetes -v` is run
- **WHEN** the preview is shown
- **THEN** it also lists the rendered manifest and the commands to run

### Requirement: Failures are always visible

A failed step SHALL print its error, limited to the last 10 lines, under the `✗` line without requiring any flag, and SHALL NOT be printed a second time by the top-level error handler.

#### Scenario: Command fails

- **GIVEN** a step runs a command that exits non-zero
- **WHEN** the step fails
- **THEN** the last lines of the command output are shown under the failed step
- **AND** a hint to re-run with `-v` or `--debug` is shown when neither is set

### Requirement: Output written during a step does not corrupt the spinner

Warnings and completed-step lines printed while a step is running SHALL clear the spinner line first and let it redraw below.

#### Scenario: Warning inside a step

- **GIVEN** a step is running with a spinner
- **WHEN** a warning line is printed
- **THEN** the warning appears on its own line and the spinner continues underneath

### Requirement: Uninstall and update commands use the same step style

The `uninstall` and `update` commands SHALL present their work as steps, with a compact summary before the confirmation prompt, the same as install flows.

#### Scenario: Kubernetes uninstall

- **GIVEN** `dtwiz uninstall kubernetes` is run
- **WHEN** the user confirms
- **THEN** each of deleting the custom resources, waiting for pods, helm uninstall and deleting the namespace is shown as one step line
- **AND** a failing step shows `✗ <label>  failed` with its error while the remaining steps still run

### Requirement: Failed steps are marked as failed

A failed step SHALL be printed as `✗ <label>  failed` followed by the error tail, because step labels are in past tense.

#### Scenario: Step fails

- **GIVEN** a step labelled "Uninstalled Dynatrace Operator" returns an error
- **WHEN** the step line is printed
- **THEN** it reads `✗ Uninstalled Dynatrace Operator  failed`

### Requirement: The install header is printed once

An installer SHALL NOT print its own section header when the calling command has already printed one for the same install.

#### Scenario: Run through setup

- **GIVEN** `dtwiz setup` printed "Installing: Kubernetes cluster"
- **WHEN** the Kubernetes installer starts
- **THEN** no second "dtwiz · Kubernetes" header is printed

#### Scenario: Direct install command

- **GIVEN** `dtwiz install kubernetes` is run directly
- **WHEN** the installer starts
- **THEN** it prints its own "dtwiz · Kubernetes" header

### Requirement: Long-running phases show progress

Any phase that can take noticeable time (tenant setup, lookups, scans, downloads, installs) SHALL show a spinner while it runs, including phases whose helpers print their own result lines.

#### Scenario: Tenant setup before the collector install

- **GIVEN** the extension is installed and OpenPipeline routes are configured before the collector is downloaded
- **WHEN** these phases run
- **THEN** a spinner with a present-tense label is shown while each runs
- **AND** the result lines printed by the helpers appear above the spinner as they complete
