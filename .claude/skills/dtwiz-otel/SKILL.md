# OTel Instrumentation Developer

You are assisting with development on **dtwiz**'s OpenTelemetry support. This means downloading the OTel Collector binary, auto-instrumenting one or more running apps (Java, Python, Node.js, Go), and watching Dynatrace for ingested data.

See [AGENTS.md](../../../AGENTS.md) for general Go development guidelines.

## Project Structure (OTel-specific)

```
pkg/installer/
  otel.go                      # Top-level OTel install orchestrator
  otel_collector.go            # Collector binary download + lifecycle
  otel_env.go                  # OTEL_* env var generation (shared)
  otel_java*.go                # Java auto-instrumentation (3 files)
  otel_python.go               # Python auto-instrumentation
  otel_nodejs*.go              # Node.js auto-instrumentation
  otel_go.go                   # Go (manual SDK instructions only)
  otel_process.go              # ManagedProcess: launch, port-detect, lifecycle
  otel_runtime_scan.go         # Project directory scanning
  otel_runtime_scan_unix.go    # Unix process detection
  otel_runtime_scan_windows.go # Windows process detection
  otel_uninstall.go            # Uninstall orchestrator
  ingest_watch.go              # Post-install DQL polling loop
```

## OTel Instrumentation Flow

### Install Orchestrator (`otel.go`)

```
InstallOtelCollectorWithProject()
├── Validate DT credentials (DT_ENVIRONMENT + DT_ACCESS_TOKEN or DT_PLATFORM_TOKEN)
├── prepareCollectorPlan() — download binary, generate config, find running collectors
├── detectAvailableRuntimes() — check for Python, Java, Node.js, Go on PATH
├── detectAllProjects() — parallel scan across enabled runtimes
├── User selects project → createRuntimePlan() → runtime-specific plan
├── Display plan preview (PrintPlanSteps)
├── User confirmation (confirmProceed)
├── Execute plan (start collector + instrument app)
└── WatchIngest() — poll DQL until user exits
```

### Runtime Instrumentation Pattern

Each runtime (Java, Python, Node.js, Go) implements the same interface:
- **Project detection**: scan for language-specific markers (e.g., `pom.xml`, `requirements.txt`, `package.json`)
- **Process enrichment**: match running processes to detected projects (OS-specific via `detectProcesses()`)
- **Plan building**: `buildXxxInstrumentationPlan()` → collects entrypoints, prepares env vars
- **Plan interface**: `Runtime()`, `PrintPlanSteps()`, `Execute()`

Process detection is OS-specific (see AGENTS.md); each runtime has its own heuristics for identifying and enriching running processes (e.g., `javaDescendantPort()` for Maven/Gradle wrappers).

### ManagedProcess (`otel_process.go`)

`ManagedProcess` tracks a launched subprocess: PID, detected ports, logs, exit status. Use `StartManagedProcess()` to launch; `PrintProcessSummary()` polls for port detection with timeout. Shared across all runtimes.

## OTel Runtimes

`detectAvailableRuntimes()` determines which runtimes are available by default.

**Feature flag**: `DTWIZ_ALL_RUNTIMES` / `--all-runtimes` enables all runtimes (Java and Go are gated by default).

## Dynatrace Auth & Environment Variables

```go
// Credentials — never written to disk
DT_ENVIRONMENT    // e.g. https://abc123.live.dynatrace.com
DT_ACCESS_TOKEN   // Classic API token (dt0c01.*)
DT_PLATFORM_TOKEN // Platform API token (dt0s16.*)

// OTel env vars injected into instrumented processes
OTEL_SERVICE_NAME
OTEL_EXPORTER_OTLP_ENDPOINT          // DT ingest URL
OTEL_EXPORTER_OTLP_HEADERS           // Authorization: Api-Token ...
OTEL_EXPORTER_OTLP_PROTOCOL          // http/protobuf
OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE  // delta
```

See `otel_env.go` for the shared env var builder. Runtime-specific additions (Python logging, Node resource detectors) live in each runtime's file.

## OTel Collector

- Latest release resolved from GitHub (avoids API rate limits with fallback)
- Downloads platform-specific binary (darwin/linux/windows, amd64/arm64)
- Config generated from embedded `otel.tmpl` template with DT endpoint + auth header
- Config written to `./opentelemetry/config.yaml`; binary to `./opentelemetry/`
- Lifecycle managed via `ManagedProcess`

## Uninstall (`otel_uninstall.go`)

Heuristic-based, best-effort cleanup:
1. Find running OTel Collector processes
2. Scan for collector install dirs (`~/.opentelemetry/`, `./opentelemetry/`)
3. Detect instrumented processes per runtime (each runtime has its own detection heuristic)
4. Show combined preview, user confirms
5. Kill processes + remove directories

## Testing Strategy

### Unit Tests

- Table-driven tests for all config builders and env var generation
- Test path construction on current platform
- Use interfaces to mock filesystem and OS operations
- Mock HTTP calls for agent/collector downloads

### Platform-Specific Tests

```go
// otel_xxx_unix_test.go
//go:build !windows

// otel_xxx_windows_test.go
//go:build windows
```

### Integration Tests

Use build tag `//go:build integration` for tests requiring external services or real binaries.

## Instrumentation Isolation Principles

Prefer isolation strategies over global installations — use virtual environments, local package directories, and user-level paths rather than system-wide package managers. Current examples:

- **Python**: project-local `.venv/`; OTel packages and project deps installed there
- **Node.js**: project-local `.otel/` directory with its own `package.json` and `node_modules/`; never touches the project's own `node_modules/`
- **Java**: agent JAR downloaded to `~/.opentelemetry/java/`; no packages installed; injected via `-javaagent:` flag or `JAVA_TOOL_OPTIONS` env var

**Never write secrets to disk.** `OTEL_EXPORTER_OTLP_HEADERS` contains the API token — always pass it via `cmd.Env` at process launch, never embed it in generated scripts or config files.

**Fail fast on missing prerequisites.** Check required artifacts exist before doing any work (e.g., `node_modules/` for Node.js, `.output/server/index.mjs` for Nuxt). Emit an actionable error pointing the user to the fix.

## OTel-Specific Reminders

1. Process detection logic is runtime- and OS-specific; keep Unix and Windows paths separated
2. Credentials live in env vars only — never write tokens to disk
3. Default to `http/protobuf` OTLP protocol (Dynatrace requirement)
4. The plan-preview → confirm → execute pattern is consistent across all runtimes — keep it that way
5. `WatchIngest()` is always the final step after instrumentation; don't skip it
6. Runtime-specific detection heuristics may need adjustment per platform (e.g., `javaDescendantPort()` has Unix and Windows implementations)
7. Stop running processes before re-instrumenting — all runtimes detect and stop existing processes as part of `Execute()`
8. `OTEL_SERVICE_NAME` is per-entrypoint — when a project has multiple entrypoints, each gets a distinct service name
