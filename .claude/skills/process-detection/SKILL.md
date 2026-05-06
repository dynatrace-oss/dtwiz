# Running Process Detection

This skill covers how dtwiz discovers, identifies, and correlates running OS processes to project directories. Process detection is the foundation for auto-instrumentation — it answers "what's running and where does it live?".

We want to reliably map instrumented services to proper ports.

## Architecture Overview

```
pkg/installer/
  otel_runtime_scan.go            # Shared types + project-process correlation
  otel_runtime_scan_unix.go       # Unix: ps, lsof
  otel_runtime_scan_windows.go    # Windows: Get-CimInstance Win32_Process, Get-NetTCPConnection
  otel_process.go                 # ManagedProcess lifecycle (launch, port poll, exit tracking)
  otel_process_windows.go         # Windows child-adoption (leaf PID discovery, watchPID)
  otel_process_other.go           # Unix no-op stub for adoptExeclChildren
```

## Core Data Model

```go
type DetectedProcess struct {
    PID              int
    Command          string    // Full command line string
    WorkingDirectory string    // Resolved cwd of the process
    Description      string    // Optional enriched metadata
}
```

A process is identified by three axes: **PID**, **command line**, and **working directory**. Any correlation logic uses at least two of these.

## Platform-Specific Process Enumeration

### Unix (Linux / macOS)

- **List processes**: `ps ax -o pid=,command=` — one line per process, space-separated PID + full command.
- **Resolve working directory**: `lsof -a -d cwd -p <pid> -Fn` — output lines prefixed with `n` contain the path.
- **Detect listening port**: `lsof -a -i TCP -sTCP:LISTEN -p <pid> -Fn -P` — parse `n` lines, extract port after last `:`.
- **Check loaded files**: `lsof -p <pid> -Fn` — verify files loaded by a process even if not on the command line.

### Windows

- **List processes**: `Get-CimInstance Win32_Process` via PowerShell. Fields: `ProcessId`, `CommandLine`, `WorkingDirectory`. Queried with `Where-Object` filter on `CommandLine`.
- **Resolve working directory**: Included in the same CimInstance query (no separate call needed, unlike Unix).
- **Detect listening port**: `Get-NetTCPConnection -State Listen -OwningProcess <pid>` with port exclusion filter.
- **Check loaded modules**: `Get-Process -Id <pid> | ForEach-Object { $_.Modules }` to inspect loaded DLLs/files.

### Key Differences

| Concern | Unix | Windows |
|---------|------|---------|
| Process listing | Single `ps` call, text parsing | PowerShell CimInstance query |
| Working directory | Separate `lsof` call per PID | Included in CimInstance result |
| Port detection | `lsof` with TCP filter | `Get-NetTCPConnection` |
| Performance | Fast (native commands) | Slower (PowerShell startup overhead per call) |
| Parent-child tree | Not directly available from `ps` | `ParentProcessId` field in CimInstance |
| PID of self | `os.Getpid()` filtered out | Same |

## Process Filtering

`detectProcesses(filterTerm, excludeTerms)` is the single entry point on both platforms:

1. **Filter term**: Case-insensitive substring match on the command line (e.g., `"java"`, `"python"`, `"node"`, `"go"`).
2. **Exclude terms**: Command lines containing any exclude term are dropped. Prevents matching utility/tooling processes (package managers, build tools, dtwiz itself).
3. **Self-exclusion**: The current process PID (`os.Getpid()`) is always excluded.

Each runtime defines its own filter term and exclude list. The pattern: include the runtime binary name, exclude known noise processes that aren't the user's application.

## Project-Process Correlation

`matchingProcessIDs(dirPath, processes)` correlates detected processes to a project directory using **two independent signals** (case-insensitive):

1. **Working directory starts with** the project path — the process was launched from within the project tree.
2. **Command line contains** the project path — the project path appears as an argument.

Either condition matching is sufficient. Multiple processes can match a single project.

## Parent-Child PID Resolution

When parent-child relationships between PIDs can't be determined directly (Unix `ps` output doesn't include PPID), the system falls back to **command-line content matching**:

- Match processes to projects by checking if the command line contains the project path.
- Use runtime-specific tooling to enumerate processes by metadata (e.g., `jps -l` lists JVMs by main class).
- On Windows, `ParentProcessId` is available from `Get-CimInstance Win32_Process`, enabling direct tree walks.

### Wrapper/Launcher Problem (General Pattern)

Many runtimes use wrappers that fork a child process to run the actual app. The wrapper PID is detected first, but the *app* process is a separate PID.

**Unix approach**: Use runtime-specific enumeration tools or command-line matching to find the real app process, then probe it for a listening port.

**Windows approach**: Walk the process tree from the wrapper PID using `ParentProcessId`, collect descendants matching the runtime binary, then check for listening ports.

### `os.execl` Child Adoption (Windows-only)

Some launchers use `os.execl()` which on Windows spawns a new process and the launcher exits, breaking PID tracking. On Unix, `execl` replaces the process in-place (same PID) so this isn't an issue.

**Windows solution**: Query `Win32_Process` for processes matching the runtime binary name AND the original entrypoint in CommandLine. Filter to leaf processes (those whose PID doesn't appear as any other match's `ParentProcessId`). Adopt the leaf PID via `windows.OpenProcess(SYNCHRONIZE)` + `WaitForSingleObject`.

The `adoptExeclChildren` function is a no-op on Unix.

## Port Detection

After a process is started or adopted, port detection polls until a listening TCP port appears:

- **Excluded ports**: 4317, 4318 (OTel Collector gRPC/HTTP) — never reported as the app's port.
- **Poll interval**: 500ms.
- **Timeout**: 15 seconds after the settle delay.
- **Settle delay**: 3 seconds before first poll (let the process bind its port).

Custom `portDetector` functions can be set per `ManagedProcess` to handle wrapper/descendant cases where the port is on a child PID rather than the tracked PID.

## Process Lifecycle Management

`stopProcesses(pids)` handles graceful shutdown:
- **Unix**: Send `SIGINT` (graceful) then `process.Wait()`.
- **Windows**: `killAndWaitProcess()` calls `Kill()` and polls until PID is gone (no reliable cross-process signal mechanism on Windows).

## Key Principles

1. **Command line is the universal fallback.** When process tree relationships aren't available, substring matching on the full command line correlates processes to projects.
2. **Platform divergence is isolated in build-tagged files.** Shared logic calls platform-specific functions defined in `_unix.go` / `_windows.go` files.
3. **PowerShell is expensive.** Every Windows detection call pays PowerShell startup overhead. Batch queries where possible (single `Get-CimInstance` call with `Where-Object` instead of per-PID queries).
4. **Exclude noise early.** Filter terms and exclude terms prevent false matches from tooling processes (package managers, build daemons, dtwiz itself).
5. **Two-signal correlation.** Matching uses both working directory and command-line content — either is sufficient, both together increase confidence.
6. **Self-exclusion is mandatory.** Always filter `os.Getpid()` to avoid instrumenting dtwiz itself.
7. **Port 4317/4318 are always excluded.** These are OTel Collector ports and must never be reported as an application's listening port.
8. **Enrichment is opportunistic.** If runtime-specific tools aren't available, detection still works — just with less metadata in `Description`.
9. **Adoption handles launcher patterns.** When a launcher exits and a child continues (Windows `os.execl` pattern), explicit PID adoption re-establishes tracking.
10. **Case-insensitive matching everywhere.** All path and command comparisons use `strings.ToLower()` to handle filesystem and command casing differences across platforms.
