# Tasks: uninstall-python-otel-processes

- [x] Add `RuntimeCleaner` interface and `runtimeCleaners` registry to `pkg/installer/otel_uninstall_runtime.go`
- [x] Add `pythonCleaner` struct implementing `RuntimeCleaner` to `pkg/installer/otel_uninstall_python.go`
- [x] Update `UninstallOtelCollector` to iterate `runtimeCleaners` for preview and stop
- [x] Add tests for `pythonCleaner` and uninstall preview in `pkg/installer/otel_uninstall_python_test.go`
