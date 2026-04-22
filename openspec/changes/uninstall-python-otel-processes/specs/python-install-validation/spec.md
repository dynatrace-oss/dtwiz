## ADDED Requirements

### Requirement: Detect running Python processes on uninstall
`dtwiz uninstall otel` SHALL detect running Python processes using `detectProcesses("python", []string{"pip ", "setup.py", "/bin/dtwiz"})` — the same filter as install-time detection. Filtering on `"opentelemetry-instrument"` is explicitly incorrect: `opentelemetry-instrument` uses `os.execl` on Unix (replacing the wrapper process image with `python`) and spawns a `python` child and exits on Windows. In both cases the surviving process appears as a plain `python` command with no wrapper visible in the process list.

#### Scenario: One instrumented Python process is running
- **WHEN** `dtwiz uninstall otel` is run and one `python` process is found (excluding pip/setup.py/dtwiz)
- **THEN** that process is listed in the preview under "Instrumented Python processes that will be stopped"

#### Scenario: Multiple Python processes are running
- **WHEN** multiple `python` processes match the filter
- **THEN** all are listed in the preview, each showing its PID and command

#### Scenario: No Python processes are running
- **WHEN** no `python` process matches the filter
- **THEN** the Python preview section is omitted and output is identical to previous behaviour

#### Scenario: Process detection fails
- **WHEN** the underlying process scan returns an error (e.g. permission denied)
- **THEN** the Python section is silently skipped; uninstall continues for other artifacts

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
- **THEN** a "Stopped PID <n>" line is printed for each successfully stopped process

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
- **THEN** "[dry-run] No changes made." is printed and no processes are killed

---

### Requirement: Debug logging for Python process detection on uninstall
The uninstall detection step SHALL emit debug log lines (visible with `--debug`).

#### Scenario: Debug mode enabled during uninstall
- **WHEN** `dtwiz uninstall otel` is run with `--debug`
- **THEN** a debug line is emitted for each Python process found, logging PID and command
- **THEN** a summary debug line is emitted with the total matched count
