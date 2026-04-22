## ADDED Requirements

### Requirement: Detect instrumented Python processes
`dtwiz uninstall otel` SHALL scan all running processes and identify those with `opentelemetry-instrument` in their command line as OTel-instrumented Python processes managed by dtwiz.

#### Scenario: One instrumented Python process is running
- **WHEN** `dtwiz uninstall otel` is run and one process has `opentelemetry-instrument` in its command line
- **THEN** that process is listed in the preview under "Instrumented Python processes that will be stopped"

#### Scenario: Multiple instrumented Python processes are running
- **WHEN** `dtwiz uninstall otel` is run and multiple processes match the marker
- **THEN** all matching processes are listed in the preview, each showing its PID and command

#### Scenario: No instrumented Python processes are running
- **WHEN** `dtwiz uninstall otel` is run and no process contains `opentelemetry-instrument`
- **THEN** no Python section appears in the preview and output is identical to the current behaviour

#### Scenario: Process detection fails (e.g. permission error)
- **WHEN** the underlying process scan returns an error
- **THEN** the Python section is silently skipped (no error shown to user, uninstall continues for other artifacts)

---

### Requirement: Preview includes Python processes
The uninstall preview SHALL include a "Instrumented Python processes" section listing all detected processes before any confirmation is requested.

#### Scenario: Collector and Python processes both found
- **WHEN** both a collector process and instrumented Python processes are detected
- **THEN** the preview shows the collector section followed by the Python section, then the single "Proceed?" prompt

#### Scenario: Only Python processes found (no collector running)
- **WHEN** no collector process is running but instrumented Python processes are detected
- **THEN** the preview shows "No running collector processes found." and then the Python processes section
- **THEN** the confirmation prompt is still shown and stopping proceeds on confirmation

#### Scenario: Nothing found
- **WHEN** no collector, no Python processes, and no install directories are found
- **THEN** the command prints "Nothing to remove" and exits without prompting

---

### Requirement: Stop instrumented Python processes on confirmation
After user confirmation, `dtwiz uninstall otel` SHALL stop all detected instrumented Python processes using SIGINT (Unix) or Kill (Windows), consistent with how `stopProcesses()` already works.

#### Scenario: User confirms
- **WHEN** the user confirms the prompt
- **THEN** each detected Python process receives SIGINT (Unix) or is killed (Windows)
- **THEN** a "Stopped PID <n>" line is printed for each successfully stopped process

#### Scenario: A process has already exited before kill
- **WHEN** a detected Python process is no longer running at the time of the kill attempt
- **THEN** a warning line is printed ("Warning: could not stop PID <n>: ...") and uninstall continues

#### Scenario: User cancels
- **WHEN** the user enters "n" at the confirmation prompt
- **THEN** no processes are stopped and "Uninstall cancelled." is printed

---

### Requirement: Dry-run skips process stopping
When `--dry-run` is passed, `dtwiz uninstall otel` SHALL show the full preview including Python processes but SHALL NOT stop any process.

#### Scenario: Dry-run with instrumented Python processes detected
- **WHEN** `dtwiz uninstall otel --dry-run` is run and Python processes are detected
- **THEN** the Python processes appear in the preview
- **THEN** "[dry-run] No changes made." is printed and no processes are killed

---

### Requirement: Debug logging for Python process detection
The detection step SHALL emit debug log lines (visible with `--debug`) so that the detection logic can be traced without modifying source code.

#### Scenario: Debug mode enabled
- **WHEN** `dtwiz uninstall otel` is run with `--debug`
- **THEN** a debug line is emitted for each process scanned, indicating whether it matched the `opentelemetry-instrument` marker
