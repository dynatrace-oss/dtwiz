## Why

On Windows, Python's `os.execl()` (used by `opentelemetry-instrument` to launch the instrumented app) is implemented as `subprocess.Popen` + `sys.exit(0)` — unlike Unix where it replaces the process in-place. This causes Go's `cmd.Wait()` to return immediately when the launcher exits, making dtwiz incorrectly declare all instrumented services as dead even though Flask (and other frameworks) are running correctly in orphaned child processes.

## What Changes

- **New**: Windows child-process adoption — after a launched process exits cleanly, scan for its Python child process via PowerShell `Get-CimInstance` and take over tracking it using `OpenProcess`/`WaitForSingleObject` (`golang.org/x/sys/windows`)
- **New**: Windows-only `watchPID` helper that waits on an arbitrary PID via process handle, with explicit user-facing debug messages when access is denied
- **Modified**: `PrintProcessSummary` in `otel_process.go` — inserts adoption pass between settle delay and port detection; Unix/macOS path is completely unchanged
- **Modified**: `otel_runtime_scan_windows.go` — `detectProcesses` uses `Get-CimInstance Win32_Process` with pipe-delimited `ForEach-Object` output and `strings.SplitN` parsing; removes `ConvertTo-Csv`, `Select-Object`, and `parseSimpleCSVRow`
- **Modified**: `otel_collector_windows.go` — `findRunningOtelCollectors` uses `Get-CimInstance Win32_Process` name-pattern search
- **Modified**: `detect_otel_windows.go` — `detectOtelCollector` uses `Get-CimInstance Win32_Process` command-line search
- **Modified**: `go.mod` — promotes `golang.org/x/sys` from indirect to direct dependency (`watchPID` requires `OpenProcess`/`WaitForSingleObject`)

## Capabilities

### New Capabilities

- `windows-process-adoption`: Tracking instrumented service processes on Windows by adopting the child Python process spawned by `opentelemetry-instrument`'s `os.execl` call, using native Win32 APIs with full debug logging and user-facing error messages on access failures

### Modified Capabilities

- `python-install-validation`: Process liveness detection after instrumented service launch now correctly reports running services on Windows (previously all services were reported as dead due to the `execl` child-spawn behaviour)

## Impact

- `pkg/installer/otel_process.go` — `ManagedProcess` struct (`adopted` field removed), `PrintProcessSummary`
- `pkg/installer/otel_process_windows.go` — new file (`//go:build windows`): `pythonChildPIDs`, `adoptExeclChildren`, `watchPID`
- `pkg/installer/otel_process_other.go` — new file (`//go:build !windows`): no-op `adoptExeclChildren` stub
- `pkg/installer/otel_runtime_scan_windows.go` — `detectProcesses`: pipe-delimited PowerShell output, `parseSimpleCSVRow` removed
- `pkg/installer/otel_collector_windows.go` — new file (`//go:build windows`): `findRunningOtelCollectors` via `Get-CimInstance`
- `pkg/installer/otel_collector_other.go` — new file (`//go:build !windows`): Unix `findRunningOtelCollectors` via `pgrep`
- `pkg/analyzer/detect_otel_windows.go` — `detectOtelCollector` via `Get-CimInstance`
- `go.mod` — `golang.org/x/sys` promoted to direct
- No changes to Unix/macOS code paths, no CLI interface changes, no breaking changes
