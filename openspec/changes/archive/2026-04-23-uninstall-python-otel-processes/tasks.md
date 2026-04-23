# Tasks: uninstall-python-otel-processes

- [x] Add `RuntimeCleaner` interface and `runtimeCleaners` registry to `pkg/installer/otel_uninstall_runtime.go`
- [x] Add `pythonCleaner` struct implementing `RuntimeCleaner` to `pkg/installer/otel_uninstall_python.go`
- [x] Update `UninstallOtelCollector` to iterate `runtimeCleaners` for preview and stop
- [x] Add tests for `pythonCleaner` and uninstall preview in `pkg/installer/otel_uninstall_python_test.go`
- [x] Add `processHasOtelEnvVars(pid int) bool` to `otel_runtime_scan_unix.go` (macOS: `ps eww`, Linux: `/proc/<pid>/environ`) and `otel_runtime_scan_windows.go` (fallback: command-line check)
- [x] Filter `detectPythonProcesses()` in `otel_python_project.go` to only return processes where `processHasOtelEnvVars` returns true
- [x] Add/update tests to cover the env var filter
