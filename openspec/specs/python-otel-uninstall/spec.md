# Spec: Python OTel Uninstall

## Purpose

Define how `dtwiz uninstall otel` detects and removes Python OTel instrumentation from managed project directories.

## Requirements

### Requirement: Detect running Python processes associated with OTel-managed project directories on uninstall

`dtwiz uninstall otel` SHALL detect running Python processes associated with a known project directory using a two-pass approach: (1) broad command-line filter via `detectProcesses("python", ...)` (filtering on `"opentelemetry-instrument"` is incorrect as the process appears as plain `python`); (2) cross-reference against project directories identified by marker files, matched via `matchProcessesToProjects()`. This path correlation cannot verify that a matched process is actively instrumented with OpenTelemetry, so an uninstrumented process running from a discovered project directory MAY also be matched.

#### Scenario: One Python process associated with a project directory is running

- **WHEN** `dtwiz uninstall otel` is run and one `python` process is found whose working directory matches a scanned Python project path
- **THEN** that process is listed in the preview under "Instrumented Python processes that will be stopped"

#### Scenario: Multiple Python processes are running, some outside any project directory

- **WHEN** multiple `python` processes match the broad filter but only some have working directories that match scanned Python project paths
- **THEN** only those matching processes are listed in the preview

#### Scenario: No Python processes are running

- **WHEN** no `python` process matches the filter
- **THEN** the Python preview section is omitted and output is identical to previous behaviour

#### Scenario: Process detection fails

- **WHEN** the underlying process scan returns `nil` (e.g. permission denied or scan error)
- **THEN** that runtime's section is silently skipped; uninstall continues for other artifacts

---

### Requirement: Uninstall preview includes Python processes

The uninstall preview SHALL include a Python processes section before the confirmation prompt.

#### Scenario: Collector and Python processes both found

- **WHEN** both a collector process and Python processes are detected
- **THEN** the preview shows the collector section, then the Python section, then the single "Proceed?" prompt

#### Scenario: Only Python processes found

- **WHEN** no collector is running but Python processes are detected
- **THEN** the preview shows "No running collector processes found." followed by the Python section
- **THEN** the confirmation prompt is still shown and stopping proceeds on confirmation

#### Scenario: Nothing found

- **WHEN** no collector, no Python processes, and no install directories are found
- **THEN** the command prints "Nothing to remove" and exits without prompting

---

### Requirement: Stop Python processes on uninstall confirmation

After user confirmation, `dtwiz uninstall otel` SHALL stop all detected Python processes using SIGINT (Unix) or Kill (Windows), consistent with `stopProcesses()`.

#### Scenario: User confirms

- **WHEN** the user confirms the prompt
- **THEN** each detected Python process receives SIGINT (Unix) or is killed (Windows)
- **THEN** a "Stopped PID n" line is printed for each successfully stopped process

#### Scenario: Process has already exited

- **WHEN** a detected Python process is no longer alive at kill time
- **THEN** a warning line is printed and uninstall continues

#### Scenario: User cancels

- **WHEN** the user enters "n" at the confirmation prompt
- **THEN** no processes are stopped and "Uninstall cancelled." is printed

---

### Requirement: Dry-run shows Python processes without stopping them

When `--dry-run` is passed, `dtwiz uninstall otel` SHALL show the full preview including Python processes but SHALL NOT stop any process.

#### Scenario: Dry-run with Python processes detected

- **WHEN** `dtwiz uninstall otel --dry-run` is run and Python processes are detected
- **THEN** the Python processes appear in the preview
- **THEN** "[dry-run] No changes made." is printed and no processes are stopped

---

### Requirement: Debug logging for Python process detection on uninstall

The uninstall detection step SHALL emit debug log lines (visible with `--debug`).

#### Scenario: Debug mode enabled during uninstall

- **WHEN** `dtwiz uninstall otel` is run with `--debug`
- **THEN** a debug line is emitted for each Python process found, logging PID and command
- **THEN** a summary debug line is emitted with the total matched count
