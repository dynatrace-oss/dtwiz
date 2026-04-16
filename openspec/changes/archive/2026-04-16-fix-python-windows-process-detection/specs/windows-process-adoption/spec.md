# Spec: Windows Process Adoption

## Purpose

Enable dtwiz to track and instrument Python child processes on Windows after their launchers exit, by adopting the child process for continued monitoring through liveness and port detection phases.

## ADDED

### Requirement: Windows child process adoption after execl spawn

On Windows, when a process started by dtwiz exits cleanly within the settle window **and is marked as an execl launcher** (`IsExeclLauncher == true`), the system SHALL attempt to adopt its Python child process for continued liveness tracking.

`ManagedProcess` carries two new fields used by the adoption path:

- `Entrypoint string` — the script/entrypoint that was launched (e.g. `app.py`), set by `StartManagedProcess` when `entrypoint != ""`
- `IsExeclLauncher bool` — set to `true` when `entrypoint != ""` (i.e. when the process is expected to exec-spawn a child and exit)

The adoption query is **entrypoint/CommandLine-based**, not parent-PID-based. `pythonLeafPID(entrypoint string) (int, error)` queries all processes whose name contains `python` and whose `CommandLine` contains the entrypoint path, then identifies the leaf — the process whose `ProcessId` does not appear as the `ParentProcessId` of any other matched process. This approach is robust even after the launcher exits (parent-PID queries would find nothing once the launcher is gone).

#### Scenario: Launcher exits cleanly and Python child is found

- **WHEN** a managed process exits with code 0 within the `processSettleDelay` window on Windows
- **AND** `IsExeclLauncher` is `true` and `Entrypoint` is non-empty
- **AND** `pythonLeafPID(entrypoint)` returns a non-zero PID
- **THEN** the `ManagedProcess` PID SHALL be replaced with the child's PID
- **AND** a new `watchPID` goroutine SHALL be started on the child's process handle
- **AND** `logger.Debug` SHALL record `"adoption: adopted windows child process"` with `name`, `launcher_pid`, `child_pid`, and `entrypoint` fields
- **AND** the adopted process SHALL participate in port detection and alive-collection as normal

#### Scenario: Launcher exits cleanly but no Python child is found

- **WHEN** a managed process exits with code 0 within the settle window on Windows
- **AND** `IsExeclLauncher` is `true` and `Entrypoint` is non-empty
- **AND** `pythonLeafPID(entrypoint)` returns 0
- **THEN** the process SHALL remain in the exited state
- **AND** `logger.Debug` SHALL record `"adoption: no running python process matched entrypoint"` with `name` and `entrypoint` fields

#### Scenario: Launcher exits with non-zero code (crash)

- **WHEN** a managed process exits with a non-zero exit code on Windows
- **THEN** adoption SHALL NOT be attempted
- **AND** the process SHALL be reported as crashed with the original exit error

#### Scenario: Process is not an execl launcher

- **WHEN** a managed process exits cleanly on Windows
- **AND** `IsExeclLauncher` is `false` (i.e. `Entrypoint` was empty at start)
- **THEN** adoption SHALL NOT be attempted
- **AND** `logger.Debug` SHALL record `"adoption: not an execl launcher, skipping"` with `name` and `pid` fields

#### Scenario: CommandLine query fails

- **WHEN** the `pythonLeafPID` PowerShell query returns an error
- **THEN** adoption SHALL NOT be attempted
- **AND** `logger.Debug` SHALL record `"adoption: CommandLine query failed"` with `name`, `entrypoint`, and `err` fields

#### Scenario: OpenProcess fails on adopted child PID

- **WHEN** `OpenProcess(SYNCHRONIZE)` fails for the adopted child PID
- **THEN** the process SHALL be reported as dead (nil sent to exit channel)
- **AND** `logger.Debug` SHALL record a message explaining the failure, including: the PID, the error, and the instruction that the user should ensure dtwiz is run as the same user that owns the Python process

#### Scenario: Non-Windows platform

- **WHEN** dtwiz is running on Unix or macOS
- **THEN** `adoptExeclChildren` SHALL be a no-op and SHALL NOT attempt any process queries
- **AND** no Windows-specific imports or symbols SHALL be referenced on non-Windows builds

### Testing

#### Cross-platform unit tests (`pkg/installer/otel_runtime_scan_test.go`)

Cover `parseWinProcessOutput` — the line-normalisation helper extracted from `winProcessQuery` into `otel_runtime_scan.go` (no build tag) so it can be exercised on all CI platforms:

- **Empty / whitespace-only input** → returns empty slice
- **CRLF stripping** → trailing `\r` removed from each line (Windows PowerShell output)
- **Blank-line skipping** → consecutive empty lines are dropped
- **Single-line input** → returns one-element slice with CR stripped
- **Pipe-delimited field round-trip** → a typical `ProcessId|CommandLine|WorkingDirectory` line survives `parseWinProcessOutput` → `strings.SplitN(_, "|", 3)` intact

#### Windows-only integration tests (`pkg/installer/otel_runtime_scan_windows_test.go`, `//go:build windows`)

Require PowerShell (standard on all supported Windows versions):

- **`TestWinProcessQuery_ReturnsCurrentProcess`** — queries the current process PID; verifies a line is returned and the PID parses correctly
- **`TestWinProcessQuery_NoMatch`** — queries a nonexistent PID; verifies no spurious results
- **`TestWinProcessQuery_PipeDelimitedMultiField`** — three-field query for current PID; verifies `SplitN` yields exactly three fields with the correct PID
- **`TestDetectProcesses_ExcludeTermFilter`** — queries current PID, then re-queries with the PID as an exclude term; verifies the process is absent from the second result

#### Windows-only adoption logic tests (`pkg/installer/otel_process_windows_test.go`, `//go:build windows`)

Use the existing `runningManagedProcess`, `crashedManagedProcess`, and `cleanExitedManagedProcess` test helpers:

- **`TestAdoptExeclChildren_NoExitedProcesses`** — all processes still running; counters unchanged after call
- **`TestAdoptExeclChildren_CrashedProcessSkipped`** — process exited non-zero; PID and counters unchanged
- **`TestAdoptExeclChildren_NotExeclLauncher_Skipped`** — process exited cleanly but `IsExeclLauncher` is false; PID and counters unchanged
- **`TestAdoptExeclChildren_NoChildFound_Skipped`** — process exited cleanly with `IsExeclLauncher=true`, entrypoint set to a nonexistent path; no Python child matched; PID and counters unchanged
- **`TestWatchPID_NonexistentPID`** — `watchPID` with a PID that almost certainly does not exist; channel receives nil immediately without blocking
