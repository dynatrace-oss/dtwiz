# Spec: Python Install Validation

## Purpose

Enhance process launch and lifecycle tracking on Windows by detecting and adopting Python child processes spawned via `opentelemetry-instrument`, enabling proper instrumentation collection across execl-launchers.

## MODIFIED Requirements

### Requirement: Process crash visibility

When the installer launches instrumented processes, the user SHALL receive explicit feedback if a process exits early rather than silently seeing a missing URL.

#### Scenario: Process crash status is queried more than once

- **WHEN** a process has crashed and its exit status has been read once (e.g. to print the summary line)
- **THEN** the second query SHALL return the same `(exited=true, err)` result as the first
- **AND** SHALL NOT incorrectly report the process as still running
- **NOTE** a Go channel value is consumed on receive; the implementation MUST cache the drained value on the struct to make `WaitResult()` idempotent

#### Scenario: Process crashes within the startup settle window

- **WHEN** one or more processes have been started with `opentelemetry-instrument`
- **AND** a process exits with a non-zero exit code before the settle period ends
- **THEN** the summary line SHALL show `[crashed: <exit error> — check log for details]` and the log filename
- **AND** the URL SHALL NOT be shown for that process
- **AND** if ALL processes have crashed or exited, the installer SHALL print `No services are running — check the logs above for errors.` and SHALL NOT print the traffic-waiting prompt

#### Scenario: Process exits cleanly within the startup settle window on Unix/macOS

- **WHEN** a process was started successfully on Unix or macOS
- **AND** it exits with exit code 0 before the settle period ends (e.g. a one-shot script)
- **THEN** the summary line SHALL show `[exited cleanly]` and the log filename

#### Scenario: Process exits cleanly within the startup settle window on Windows (execl adoption)

- **WHEN** a process was started successfully on Windows
- **AND** it exits with exit code 0 before the settle period ends
- **AND** a Python child process is found via PowerShell `Get-CimInstance`
- **THEN** the process SHALL NOT be reported as exited — it SHALL be adopted and continue through port detection and alive-collection as a running process
- **AND** the summary line SHALL reflect the adopted process's actual state (running with port, or running without port)

#### Scenario: Process exits cleanly on Windows with no child found

- **WHEN** a process was started successfully on Windows
- **AND** it exits with exit code 0 before the settle period ends
- **AND** no Python child process is found
- **THEN** the summary line SHALL show `[exited cleanly]` and the log filename

#### Scenario: Process is running but has not bound a port

- **WHEN** a process is still alive after the settle period (including adopted processes on Windows)
- **AND** no listening TCP port is detected for its PID
- **THEN** the summary line SHALL show `[running, port not detected]` and the log filename

#### Scenario: Process is running and has bound a port

- **WHEN** a process is still alive after the settle period (including adopted processes on Windows)
- **AND** a listening TCP port is detected for its PID
- **THEN** the summary line SHALL show `→ http://localhost:<port>` and the log filename

## ADDED

### Testing

#### Cross-platform unit tests (`pkg/installer/otel_process_test.go`)

Use pre-built `ManagedProcess` helpers with pre-filled exit channels — no real processes spawned:

- **`TestWaitResult_Idempotent`** — reads exit status twice; verifies same `(exited, err)` returned both times (channel value is cached)
- **`TestWaitResult_StillRunning`** — checks an empty channel returns `(false, nil)` immediately
- **`TestPrintProcessSummary_AllCrashed_NoAliveNames`** — two crashed processes; `aliveNames` is empty; output contains `[crashed:`
- **`TestPrintProcessSummary_SomeCrashed_OnlyAliveReturned`** — mixed crashed/running; only the running name is in `aliveNames`
- **`TestPrintProcessSummary_CrashedNonZeroExit_SummaryLabel`** — output contains `[crashed:`
- **`TestPrintProcessSummary_CleanExit_SummaryLabel`** — output contains `[exited cleanly]`
- **`TestPrintSummaryLine_Crashed_IncludesLabel`** — line contains `[crashed:` and service name
- **`TestPrintSummaryLine_CleanExit_IncludesLabel`** — line contains `[exited cleanly]`
- **`TestPrintSummaryLine_Running_IncludesRunningStatus`** — line contains `running` or `localhost`
- **`TestPrintSummaryLine_LogNameIncluded`** — log filename appears in summary line
