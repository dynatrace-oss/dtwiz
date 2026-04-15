## ADDED Requirements

### Requirement: Windows child process adoption after execl spawn

On Windows, when a process started by dtwiz exits cleanly within the settle window, the system SHALL attempt to adopt its Python child process for continued liveness tracking.

#### Scenario: Launcher exits cleanly and Python child is found

- **WHEN** a managed process exits with code 0 within the `processSettleDelay` window on Windows
- **AND** a PowerShell `Get-CimInstance` query finds exactly one child process whose name contains `python`
- **THEN** the `ManagedProcess` PID SHALL be replaced with the child's PID
- **AND** a new `watchPID` goroutine SHALL be started on the child's process handle
- **AND** `logger.Debug` SHALL record `"adopted windows child process"` with `original_pid`, `new_pid`, and `service` fields
- **AND** the adopted process SHALL participate in port detection and alive-collection as normal

#### Scenario: Launcher exits cleanly and multiple Python children are found

- **WHEN** a managed process exits with code 0 within the settle window on Windows
- **AND** a PowerShell `Get-CimInstance` query finds more than one child whose name contains `python`
- **THEN** the child with the lowest PID SHALL be adopted
- **AND** `logger.Debug` SHALL record `"windows child adoption: multiple python children found, picking lowest PID"` with `parent_pid`, `candidates`, and `adopted_pid` fields

#### Scenario: Launcher exits cleanly but no Python child is found

- **WHEN** a managed process exits with code 0 within the settle window on Windows
- **AND** a PowerShell `Get-CimInstance` query finds no children whose name contains `python`
- **THEN** the process SHALL remain in the exited state
- **AND** `logger.Debug` SHALL record `"windows child adoption: no python child found"` with `parent_pid` field

#### Scenario: Launcher exits with non-zero code (crash)

- **WHEN** a managed process exits with a non-zero exit code on Windows
- **THEN** adoption SHALL NOT be attempted
- **AND** the process SHALL be reported as crashed with the original exit error

#### Scenario: PowerShell query fails

- **WHEN** the `Get-CimInstance` child query returns an error
- **THEN** adoption SHALL NOT be attempted
- **AND** `logger.Debug` SHALL record `"windows child adoption: PowerShell query failed"` with `parent_pid` and `err` fields

#### Scenario: OpenProcess fails on adopted child PID

- **WHEN** `OpenProcess(SYNCHRONIZE)` fails for the adopted child PID
- **THEN** the process SHALL be reported as dead (nil sent to exit channel)
- **AND** `logger.Debug` SHALL record a message explaining the failure, including: the PID, the error, and the instruction that the user should ensure dtwiz is run as the same user that owns the Python process

#### Scenario: Non-Windows platform

- **WHEN** dtwiz is running on Unix or macOS
- **THEN** `adoptExeclChildren` SHALL be a no-op and SHALL NOT attempt any process queries
- **AND** no Windows-specific imports or symbols SHALL be referenced on non-Windows builds

## Testing

### Cross-platform unit tests (`pkg/installer/otel_runtime_scan_test.go`)

Cover `parseWinProcessOutput` — the line-normalisation helper extracted from `winProcessQuery` into `otel_runtime_scan.go` (no build tag) so it can be exercised on all CI platforms:

- **Empty / whitespace-only input** → returns empty slice
- **CRLF stripping** → trailing `\r` removed from each line (Windows PowerShell output)
- **Blank-line skipping** → consecutive empty lines are dropped
- **Single-line input** → returns one-element slice with CR stripped
- **Pipe-delimited field round-trip** → a typical `ProcessId|CommandLine|WorkingDirectory` line survives `parseWinProcessOutput` → `strings.SplitN(_, "|", 3)` intact

### Windows-only integration tests (`pkg/installer/otel_runtime_scan_windows_test.go`, `//go:build windows`)

Require PowerShell (standard on all supported Windows versions):

- **`TestWinProcessQuery_ReturnsCurrentProcess`** — queries the current process PID; verifies a line is returned and the PID parses correctly
- **`TestWinProcessQuery_NoMatch`** — queries a nonexistent PID; verifies no spurious results
- **`TestWinProcessQuery_PipeDelimitedMultiField`** — three-field query for current PID; verifies `SplitN` yields exactly three fields with the correct PID
- **`TestDetectProcesses_ExcludeTermFilter`** — queries current PID, then re-queries with the PID as an exclude term; verifies the process is absent from the second result

### Windows-only adoption logic tests (`pkg/installer/otel_process_windows_test.go`, `//go:build windows`)

Use the existing `runningManagedProcess`, `crashedManagedProcess`, and `cleanExitedManagedProcess` test helpers:

- **`TestAdoptExeclChildren_NoExitedProcesses`** — all processes still running; counters unchanged after call
- **`TestAdoptExeclChildren_CrashedProcessSkipped`** — process exited non-zero; PID and counters unchanged
- **`TestAdoptExeclChildren_CleanExitNoChildrenSkipped`** — process exited cleanly, nonexistent parent PID returns no Python children; PID and counters unchanged
