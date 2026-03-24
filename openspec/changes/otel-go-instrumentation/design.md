## Context

No OTel Go code exists in the project. Go auto-instrumentation is fundamentally different from Python, Java, and Node: there is no runtime agent or `--require` hook. Go instrumentation requires adding SDK packages to the application source code and calling initialization functions. The OTel Go SDK provides `go.opentelemetry.io/otel` packages that must be imported and initialized in `main()`.

This means `dtwiz install otel-go` cannot fully automate instrumentation the way other languages can. Instead, it can: add Go module dependencies, generate a bootstrap code snippet, print instructions for where to paste it, and set up OTEL_* environment variables.

## Goals / Non-Goals

**Goals:**
- Detect Go projects (`go.mod`)
- Add OTel Go SDK dependencies via `go get`
- Generate and print an initialization code snippet for the user to add to `main()`
- Set up OTEL_* environment variables
- Implement uninstall (remove dependencies)
- Add pre-flight validation

**Non-Goals:**
- Auto-modifying Go source code (too risky and brittle)
- Adding library-specific instrumentation wrappers (net/http, gRPC, etc.)
- Supporting Go versions older than 1.21

## Decisions

**1. Dependency injection via `go get`**
Run `go get` for the core OTel packages:
- `go.opentelemetry.io/otel`
- `go.opentelemetry.io/otel/sdk`
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp`

This is safe and reversible.

**2. Generate bootstrap snippet instead of modifying source**
Print a Go code snippet that initializes TracerProvider, MeterProvider, and LoggerProvider with OTLP HTTP exporters. The user copies it into their `main()`. This is the only safe approach for Go — AST manipulation is fragile and error-prone.

Alternative: Use `go generate` or AST rewriting — rejected because Go source modification is too risky and the variety of `main()` patterns makes automation unreliable.

**3. OTEL_* env vars for exporter configuration**
Use environment variables (same as other languages) so the bootstrap snippet stays generic. The snippet reads endpoint and auth from `OTEL_EXPORTER_OTLP_ENDPOINT` and `OTEL_EXPORTER_OTLP_HEADERS`.

**4. Uninstall via `go mod edit -droprequire`**
Remove OTel packages from `go.mod` using `go mod edit -droprequire`. Then run `go mod tidy` to clean up.

## Risks / Trade-offs

- [User must manually add code] → This is unavoidable for Go. Make the snippet as copy-paste-ready as possible, with clear comments showing where to place it.
- [Snippet may not fit all project structures] → Provide a minimal, standalone snippet. Users with complex setups can adapt it.
- [Dependency versions may conflict] → `go get` handles version resolution. Accept that edge cases with replace directives may need manual intervention.
