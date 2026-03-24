## Why

Go is on the OTel language roadmap after Python, Java, and Node. Go auto-instrumentation differs fundamentally from the other languages — there is no runtime agent injection. Instead, Go uses compile-time instrumentation via `go.opentelemetry.io` SDK packages that must be added to the application source code. This means the install flow must guide the user through adding dependencies and wrapping their code, rather than launching with an agent flag.

## What Changes

- Implement `dtwiz install otel-go` — detect Go projects, add OTel Go SDK dependencies via `go get`, generate an instrumentation bootstrap snippet, configure OTEL_* environment variables for Dynatrace export
- Implement `dtwiz uninstall otel-go` — remove OTel Go dependencies from `go.mod`
- Add pre-flight validation: Go in PATH, `go.mod` presence
- Register CLI subcommands in `cmd/install.go` and `cmd/uninstall.go`
- Wire into collector app-type listing (depends on collector-improvements change)

## Capabilities

### New Capabilities
- `go-project-detection`: Detect Go projects by scanning for `go.mod`, identify the module name and main package
- `go-dependency-injection`: Add OTel Go SDK packages (`go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`, etc.) via `go get`
- `go-bootstrap-snippet`: Generate a Go source snippet that initializes the OTel SDK with Dynatrace-compatible OTLP exporter, and print instructions for where to add it
- `go-uninstall`: Remove OTel Go dependencies from `go.mod` via `go mod edit -droprequire`
- `go-install-validation`: Pre-flight checks — Go in PATH, `go.mod` exists, OS compatibility

### Modified Capabilities

## Impact

- New file: `pkg/installer/otel_go.go` — full Go instrumentation implementation
- New file: `pkg/installer/otel_go_uninstall.go` — uninstall logic
- `cmd/install.go` — register `otel-go` subcommand
- `cmd/uninstall.go` — register `otel-go` subcommand
- `pkg/installer/otel_collector.go` — add Go to app-type detection (handled by collector-improvements change)
