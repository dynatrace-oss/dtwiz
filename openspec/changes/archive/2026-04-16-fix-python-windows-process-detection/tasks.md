# Tasks: Fix Python Windows Process Detection

## 1. Dependency

- [x] 1.1 Promote `golang.org/x/sys` from indirect to direct in `go.mod` and run `go mod tidy`

## 2. Core Bug Fix — Windows Child Process Adoption

- [x] 2.1 Create `pkg/installer/otel_process_windows.go` (`//go:build windows`) with `pythonLeafPID(entrypoint string) (int, error)` — queries all processes whose name contains `python` and whose `CommandLine` contains the entrypoint path via a single PowerShell `Get-CimInstance Win32_Process` command; identifies the leaf by filtering out any PID that appears as a `ParentProcessId` of another matched process; returns 0, nil if no match
- [x] 2.2 Add `adoptExeclChildren(procs []*ManagedProcess, started *int, notStarted *int)` to `otel_process_windows.go` — iterates processes with clean exits where `IsExeclLauncher==true` and `Entrypoint!=""`, calls `pythonLeafPID(p.Entrypoint)`, updates `ManagedProcess.PID`, resets exit state fields, wires `watchPID`, adjusts counters, emits `logger.Debug` for all outcomes (not a launcher, no child, CommandLine query failure, success, `OpenProcess` failure); also add `Entrypoint string` and `IsExeclLauncher bool` fields to `ManagedProcess` and set them in `StartManagedProcess`
- [x] 2.3 Add `watchPID(pid int) chan error` to `otel_process_windows.go` — opens process handle with `SYNCHRONIZE` via `golang.org/x/sys/windows`, calls `WaitForSingleObject(INFINITE)` in goroutine; on `OpenProcess` failure logs actionable debug message directing user to run dtwiz as same user, sends nil to channel
- [x] 2.4 Create `pkg/installer/otel_process_other.go` (`//go:build !windows`) with single no-op stub: `adoptExeclChildren` does nothing
- [x] 2.5 Replace adoption loop in `PrintProcessSummary` (`pkg/installer/otel_process.go`) with a single `adoptExeclChildren(procs, &started, &notStarted)` call; remove `adopted` field from `ManagedProcess` (was set but never read)

## 3. Windows Process Enumeration — Consistent PowerShell

- [x] 3.1 Refactor `detectProcesses` in `pkg/installer/otel_runtime_scan_windows.go` — use `Get-CimInstance Win32_Process | ForEach-Object { "$($_.ProcessId)|$($_.CommandLine)|$($_.WorkingDirectory)" }` (pipe-delimited output); parse with `strings.SplitN(line, "|", 3)`; remove `ConvertTo-Csv`, `Select-Object`, and `parseSimpleCSVRow`
- [x] 3.2 Refactor OTel Collector process detection — extract into `pkg/installer/otel_collector_windows.go` (`//go:build windows`) and `otel_collector_other.go` (`//go:build !windows`); Windows implementation uses `Get-CimInstance Win32_Process` name-pattern query; `findRunningOtelCollectors` in `otel_collector.go` delegates to platform function
- [x] 3.3 Refactor `detectOtelCollector` in `pkg/analyzer/detect_otel_windows.go` — single `Get-CimInstance Win32_Process` command-line query; remove snapshot logic and `otelInfoFromProcessName`
- [x] 3.4 Revert `pkg/analyzer/detect_oneagent_windows.go` to original `Get-Service` PowerShell — OneAgent Windows support is out of scope for this change
- [x] 3.5 Revert `pkg/installer/oneagent_uninstall.go` to original `Get-WmiObject` PowerShell — out of scope for this change
- [x] 3.6 Extract `winProcessQuery(whereClause, fieldsExpr string) []string` helper in `pkg/installer/otel_runtime_scan_windows.go` — owns PowerShell invocation, CRLF trimming, and blank-line filtering; update `detectProcesses` and `findRunningOtelCollectors` (`otel_collector_windows.go`) to use it; `pythonLeafPID` (`otel_process_windows.go`) uses a direct `exec.Command` (different query shape — inline leaf-detection logic); per-PID scalar lookups and `pkg/analyzer` callers are also left as direct `exec.Command` calls (different shape / different package)

## 4. Tests

- [x] 4.1 Extract `parseWinProcessOutput(raw string) []string` from `winProcessQuery` into `pkg/installer/otel_runtime_scan.go` (no build tag) so it can be unit-tested on all platforms
- [x] 4.2 Add cross-platform unit tests for `parseWinProcessOutput` in `pkg/installer/otel_runtime_scan_test.go`: empty input, whitespace-only, CRLF stripping, blank-line skipping, single-line, pipe-delimited field round-trip
- [x] 4.3 Add Windows-only integration tests in `pkg/installer/otel_runtime_scan_windows_test.go` (`//go:build windows`): `winProcessQuery` finds current process by PID, no-match returns empty/nil, multi-field pipe-delimited output, `detectProcesses` exclude-term filter
- [x] 4.4 Add Windows-only adoption logic tests in `pkg/installer/otel_process_windows_test.go` (`//go:build windows`): `adoptExeclChildren` with all-running processes (no change), with crashed process (skipped), with non-execl-launcher clean exit (skipped), with execl launcher and no Python child matched (no adoption); also `watchPID` with nonexistent PID (channel receives nil immediately)

## 5. Verification

- [x] 5.1 Run `make test` — all tests pass
- [x] 5.2 Run `GOOS=windows GOARCH=amd64 go test -c ./pkg/installer/` — Windows test binary compiles
