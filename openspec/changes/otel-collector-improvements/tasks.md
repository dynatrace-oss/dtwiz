<!-- Parallelism: Groups 1+2 (menu) and 3 (overwrite guard) are independent — can be developed in parallel. Group 4 depends on 1+2. -->

## 1. Runtime Detection Integration

- [ ] 1.1 Add a helper function `detectAvailableRuntimes()` in `otel_collector.go` that uses `exec.LookPath` (same pattern as existing `detectPython()`) for each supported language: `python3`/`python`, `java`, `node`, `go` — return a slice of detected runtimes
- [ ] 1.2 Add a registry mapping language name → installer function pointer and implemented status (e.g., `map[string]runtimeInfo` where `runtimeInfo` has `name`, `available bool`, `installer func(...)`) so the menu can mark "coming soon" items. Initially: python=implemented, java=implemented, node=not yet, go=not yet

## 2. App-Type Selection Menu

- [ ] 2.1 Implement a numbered selection menu using `bufio.Scanner` (same input pattern as Python project selection) that displays detected runtimes + "None — collector only" as the last entry
- [ ] 2.2 Mark runtimes whose installer is not yet implemented with "(coming soon)" and skip them when validating user input
- [ ] 2.3 Wire the selected runtime to the corresponding `InstallOtel<Lang>()` function call via the registry, or skip instrumentation if "None" is selected

## 3. Overwrite Guard (independent of groups 1+2)

- [ ] 3.1 Add a pre-plan check in `InstallOtelCollector()` before `prepareCollectorPlan()` — call `findRunningOtelCollectors()` and check if the install directory (`./opentelemetry`) exists via `os.Stat`
- [ ] 3.2 If existing collector found: show what was detected (PIDs + directory path), then prompt with three options using numbered menu: (1) overwrite, (2) switch to `UpdateOtelConfig()`, (3) abort — use `bufio.Scanner` for input
- [ ] 3.3 In dry-run mode: print `[dry-run] Existing collector detected` with details, continue with dry-run plan output without prompting

## 4. Refactor Existing Python Hard-Coding (depends on 1+2)

- [ ] 4.1 Remove the hard-coded `exec.LookPath("python3")` and `DetectPythonPlan()` calls from `InstallOtelCollector()` and replace with the new `detectAvailableRuntimes()` + menu flow
- [ ] 4.2 Ensure `InstallOtelCollectorOnly()` remains unchanged (collector-only, no instrumentation menu) — verify no regressions

## 5. Testing

- [ ] 5.1 Add unit tests for `detectAvailableRuntimes()` (mock `exec.LookPath` results)
- [ ] 5.2 Add unit tests for menu construction with various runtime combinations (all detected, none detected, some "coming soon")
- [ ] 5.3 Test overwrite guard logic with and without existing collector
