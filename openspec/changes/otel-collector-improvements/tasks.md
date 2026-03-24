## 1. Runtime Detection Integration

- [ ] 1.1 Add a helper function in `otel_collector.go` that calls the analyzer's runtime detection (java, node, python3, go) and returns a list of available runtimes with their installer availability status
- [ ] 1.2 Add a registry/map of language name → installer function availability (e.g., python → implemented, java → implemented, node → not yet, go → not yet) so the menu can mark "coming soon" items

## 2. App-Type Selection Menu

- [ ] 2.1 Implement a numbered selection menu that displays detected runtimes + a "None — collector only" option as the last entry
- [ ] 2.2 Mark runtimes whose installer is not yet implemented with "(coming soon)" and make them non-selectable
- [ ] 2.3 Wire the selected runtime to the corresponding `InstallOtel<Lang>()` function call, or skip instrumentation if "None" is selected

## 3. Overwrite Guard

- [ ] 3.1 Add a pre-plan check in `InstallOtelCollector()` that calls `findRunningOtelCollectors()` and checks for the install directory before `prepareCollectorPlan()`
- [ ] 3.2 If an existing collector is found, display a prompt with three options: overwrite (continue install), switch to `dtwiz update otel`, or abort
- [ ] 3.3 In dry-run mode, report the existing installation without prompting

## 4. Refactor Existing Python Hard-Coding

- [ ] 4.1 Remove the hard-coded `exec.LookPath("python3")` and `DetectPythonPlan()` calls from `InstallOtelCollector()` and replace with the new generic runtime detection + menu flow
- [ ] 4.2 Ensure `InstallOtelCollectorOnly()` remains unchanged (collector-only, no instrumentation menu)

## 5. Testing

- [ ] 5.1 Add unit tests for runtime detection helper (mock available runtimes)
- [ ] 5.2 Add unit tests for menu construction with various runtime combinations
- [ ] 5.3 Test overwrite guard logic with and without existing collector
