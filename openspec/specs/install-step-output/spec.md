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

#### Scenario: Verbose mode disables the spinner

- **GIVEN** `-v` or `--debug` is set
- **WHEN** a step runs
- **THEN** no spinner is drawn, so log lines are not interleaved with an animated line

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
