# Design: Fix Python Windows Process Detection

## Context

`opentelemetry-instrument` is the OTel-recommended way to launch a Python application with automatic instrumentation. Internally its `run()` function calls `os.execl()` at the end to hand off to the user's Python app. On Unix/macOS, `os.execl` is a true process replacement — the launcher's PID stays alive as the app. On Windows, Python's `os.execl` is implemented as `subprocess.Popen` + `sys.exit(0)`: the launcher spawns a new child process and then exits cleanly.

dtwiz starts instrumented processes via `exec.Cmd.Start()` and tracks liveness via a goroutine that calls `cmd.Wait()`. On Windows this means `cmd.Wait()` returns the moment the launcher exits (exit code 0), `WaitResult()` reports `exited=true`, and `PrintProcessSummary` declares all services dead — even though Flask is running correctly in an orphaned child process.

The existing PowerShell-based Windows helpers (`otel_runtime_scan_windows.go`, `detect_otel_windows.go`, `detect_oneagent_windows.go`) all spawn a PowerShell subprocess for every query. With `golang.org/x/sys/windows` already a transitive dependency (v0.39.0), direct Win32 API calls are available where needed (process handle waiting).

## Goals / Non-Goals

**Goals:**

- Correctly detect and track running instrumented Python services on Windows after `opentelemetry-instrument` performs its `os.execl` child-spawn
- Use consistent `Get-CimInstance Win32_Process` PowerShell queries for all Windows process enumeration, with pipe-delimited output format for multi-field queries
- Provide actionable debug output when Windows process adoption fails so the user can self-diagnose
- Leave Unix/macOS code paths completely untouched
- Promote `golang.org/x/sys` to a direct dependency (used only in `watchPID` for `OpenProcess`/`WaitForSingleObject`)

**Non-Goals:**

- Changing the `opentelemetry-instrument` invocation — the OTel-docs-recommended approach is correct and must stay
- Replacing PowerShell for operations with no clean Win32 equivalent (working directory lookup, TCP port detection via `GetExtendedTcpTable`, MSI uninstall)
- Supporting Windows process adoption for non-Python launchers
- Any changes to the CLI interface or user-facing command structure

## Decisions

### Decision 1: Child adoption via PowerShell `Get-CimInstance` after settle delay

**Chosen**: After `processSettleDelay` (3 seconds), if a process exited cleanly (exit code 0), query child processes via `Get-CimInstance Win32_Process` filtering by `ParentProcessId` and look for a child whose name contains `python`. Replace the `ManagedProcess` PID with the child's PID and wire up a new `watchPID` goroutine.

**Why not Job Objects**: Job Objects would need to be assigned before the child spawns. The window between `cmd.Start()` returning and the launcher calling `subprocess.Popen` is too narrow to reliably assign a Job Object before the child is created. Windows 8+ supports nested Jobs, but the child might still not inherit the Job object depending on how Python's subprocess module creates it.

**Why not polling during the settle window**: The launcher exits in milliseconds; the child starts within the same Python process call. By the time the 3-second settle delay completes, the child is either fully running or has already crashed. Polling during the settle window adds complexity for zero practical benefit.

**Why clean exit only (not crashes)**: If the launcher exits with a non-zero code, the instrumentation itself failed. There is no child to adopt and the user should see the crash. Only exit code 0 triggers adoption — this matches the exact `os.execl` → `sys.exit(0)` sequence.

### Decision 2: `watchPID` via `OpenProcess(SYNCHRONIZE)` + `WaitForSingleObject`

**Chosen**: Open the adopted child's process handle with `SYNCHRONIZE` access right, then block on `WaitForSingleObject(INFINITE)` in a goroutine. Send result to a buffered `chan error` (capacity 1) — identical pattern to the existing `cmd.Wait()` goroutine in `StartManagedProcess`.

**Why this chain is necessary**: On Windows you cannot wait on a bare PID. `WaitForSingleObject` requires a kernel *handle*. `OpenProcess` is the only way to obtain a handle to a process you didn't start yourself. `SYNCHRONIZE` is the single access right that grants the ability to call `WaitForSingleObject` on the handle — nothing more. Requesting any broader right (e.g. `PROCESS_QUERY_INFORMATION`) would require elevation for processes owned by other users. With `SYNCHRONIZE` alone the call succeeds for any process the current user owns, no elevation needed.

**On OpenProcess failure**: If `OpenProcess` fails (process already gone, or owned by a different user), log an actionable debug message and send `nil` to the channel. The service is reported as dead — same as current behaviour, no regression.

**Why not get the exit code**: `WaitForSingleObject` signals when the process exits but does not return the exit code. Retrieving it would require `PROCESS_QUERY_INFORMATION` on the handle and a `GetExitCodeProcess` call. For the adopted-child case we don't need the exit code — we only need to know when the process is gone so `WaitResult()` can flip `hasExited`. The `nil` error sent to the channel is consistent with how the code already treats a clean exit.

### Decision 3: PowerShell `Get-CimInstance` for all Windows process enumeration

**Chosen**: Use `Get-CimInstance Win32_Process` for all process enumeration on Windows — `detectProcesses`, `findRunningOtelCollectors`, `detectOtelCollector`, and child adoption. For multi-field queries, use pipe-delimited output (`ForEach-Object { "$($_.ProcessId)|$($_.CommandLine)|$($_.WorkingDirectory)" }`) parsed with `strings.SplitN(line, "|", 3)`, eliminating the need for a CSV parser. Keep `lookupProcessWorkingDirectory` and `detectProcessListeningPort` as PowerShell (no clean Win32 replacements without undocumented APIs or manual DLL binding).

All "enumerate via `Where-Object` and return fields" queries within `pkg/installer` are routed through a single `winProcessQuery(whereClause, fieldsExpr string) []string` helper in `otel_runtime_scan_windows.go`. The helper owns the PowerShell invocation, CRLF trimming, and blank-line filtering. Callers (`pythonChildPIDs`, `findRunningOtelCollectors`, `detectProcesses`) pass only the filter expression and the `ForEach-Object` body. The per-PID scalar lookups (`lookupProcessWorkingDirectory`, `binaryPathFromPID`) use `-Filter "ProcessId=N" | Select-Object -ExpandProperty …` and are a different shape — they are left as direct `exec.Command` calls. `detectOtelCollector` in `pkg/analyzer` is also a direct call since it is in a separate package.

**Why not Win32 snapshot API**: `CreateToolhelp32Snapshot` was initially considered for the hot path. However, this added complexity, brought in `golang.org/x/sys/windows` as a dependency across multiple files, and provided no meaningful benefit since these calls happen at install time — not in a hot loop. A single consistent PowerShell approach is simpler and equally correct.

**Why pipe-delimited over CSV**: `ConvertTo-Csv -NoTypeInformation` requires a 25-line custom CSV parser (`parseSimpleCSVRow`) to handle quoted fields with embedded commas. Switching to `ForEach-Object { "$($_.F1)|$($_.F2)|$($_.F3)" }` reduces the parsing to a single `strings.SplitN` call and eliminates the parser entirely. A `|` character does not appear in Windows process command lines in practice.

## Risks / Trade-offs

**[Risk] Child Python process starts after the adoption window** → Extremely unlikely. The 3-second settle delay dwarfs the time between launcher exit and child start (sub-millisecond). Mitigation: debug logging captures the snapshot result so users can report edge cases.

**[Risk] Multiple Python children found** → Edge case if the instrumented app itself spawns Python subprocesses within 3 seconds. Mitigation: take the child with the lowest PID (earliest spawned); log all PIDs found at debug level so the user can investigate if the wrong one is picked.

**[Limitation] Child filter is exe-name only, not entrypoint-verified** → `adoptExeclChildren` identifies candidates by checking whether the exe name contains `python` (e.g. `python.exe`, `python3.12.exe`). This is applied only to direct children of the dead launcher PID — a narrow scope that makes system-wide false positives extremely unlikely. However, there is no check that the child's command line contains the specific entrypoint path (e.g. `app.py`) that dtwiz launched. In the normal case `opentelemetry-instrument.exe` spawns exactly one Python child (the instrumented app) within milliseconds, so the filter is sufficient. The gap is that if the instrumented app itself spawns a Python subprocess before the 3-second snapshot is taken, the lowest-PID tie-breaker will coincidentally pick the correct process — but this is not structurally guaranteed. Future hardening: pass the entrypoint path into `pythonChildPIDs` and verify it appears in the candidate's command line (already retrievable via the existing `winProcessQuery` mechanism). `ManagedProcess` would need a small struct extension to carry the entrypoint path alongside `Name`.

**[Risk] `OpenProcess` fails due to user mismatch** → Only if something outside dtwiz re-spawned the child under a different account. Mitigation: explicit, actionable debug message directing the user to run dtwiz as the same user.

**[Risk] `golang.org/x/sys` API changes** → It is a first-party Go team package with strong stability guarantees. Already a transitive dependency at v0.39.0.

**[Trade-off] PowerShell retained for working directory, port detection, and MSI uninstall** → Acceptable. These are infrequent, non-hot-path operations with no clean Win32 equivalent in `golang.org/x/sys`.

## Migration Plan

No migration needed. All changes are internal implementation details. No CLI flags, no config files, no data formats change. On Unix/macOS nothing changes. On Windows the behaviour changes from "services always reported dead" to "services correctly tracked" — a pure bug fix.

## Open Questions

None. Root cause is confirmed, Win32 APIs are verified in `golang.org/x/sys` v0.39.0, approach is settled.
