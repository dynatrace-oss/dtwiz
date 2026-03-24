## Why

Java is a first-priority language in our OTel roadmap (alongside Python). The current `dtwiz install otel-java` is a stub that prints manual instructions and a download URL. We need full automation: download the Java agent JAR, detect running Java processes, offer to restart them with instrumentation, and provide an uninstall path.

## What Changes

- Complete `dtwiz install otel-java` — automatically download the OpenTelemetry Java agent JAR, detect running Java processes, let the user select one, and restart it with the `-javaagent` flag and OTEL environment variables
- Implement `dtwiz uninstall otel-java` — stop instrumented processes and remove the downloaded agent JAR
- Add pre-flight validation: Java in PATH, version check, OS compatibility
- Wire `otel-java` into the collector's app-type listing (depends on collector-improvements change)

## Capabilities

### New Capabilities
- `java-agent-download`: Automatically download the OpenTelemetry Java agent JAR from the official GitHub releases
- `java-process-detection`: Detect running Java processes, display them to the user, and allow selecting one for instrumentation
- `java-instrumentation`: Restart the selected Java process with `-javaagent` flag and OTEL_* environment variables configured for Dynatrace export
- `java-uninstall`: Stop instrumented Java processes and remove the downloaded agent JAR
- `java-install-validation`: Pre-flight checks — Java in PATH, version >= 8, OS compatibility

### Modified Capabilities

## Impact

- `pkg/installer/otel_java.go` — replace stub with full implementation, extend function signature to add `platformToken` parameter, extend `generateOtelJavaEnvVars()` to include missing env vars (`OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`)
- New file: `pkg/installer/otel_java_uninstall.go` — uninstall logic
- `cmd/install.go` — `otel-java` subcommand already exists, no CLI change needed
- `cmd/uninstall.go` — register `otel-java` subcommand
- `pkg/installer/otel_collector.go` — add Java to app-type detection (handled by collector-improvements change)
