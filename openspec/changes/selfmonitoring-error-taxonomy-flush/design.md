# Design

## Context

`pkg/selfmonitoring` sends per-invocation `CUSTOM_INFO` events to the Dynatrace Events v2 API. The `fireSelfMonitoringEvent` function in `cmd/selfmonitoring.go` spawns a goroutine that calls `getDtEnvironment()` and then `selfmonitoring.SendEvent`. This goroutine is detached — if the process exits before the HTTP call completes, the event is silently lost.

`EventParams` already has an `Err string` field and the `er=` segment in the User-Agent header, but no call site ever populates it. `StepInvoked` and `StepCompleted` are the only step constants; there is no `StepFailed` or `StepCancelled`.

Errors throughout the codebase are opaque `fmt.Errorf` strings. There are no typed error types, no shared platform-unsupported sentinel, no structured dependency-missing error. `cmd/auth.go` distinguishes error cases by embedded string prefixes that cannot be matched programmatically.

## Goals / Non-Goals

**Goals:**

- Enqueue self-monitoring events synchronously and flush them (≤200ms) before process exit on every code path.
- Classify every command failure into a structured `ErrorType` and extract additional attributes where specified by the taxonomy.
- Fire a terminal event (`StepFailed` or `StepCancelled`) at every `RunE` return point across all command handlers.
- Replace opaque error strings in auth validation, dependency checks, and platform-unsupported guards with typed errors that carry machine-readable fields.

**Non-Goals:**

- Implement the `confirmed` stage (fired when the user passes Y/N). That is part of the onboarding funnel, not the error taxonomy.
- Classify HTTP failures inside installer packages (aws, azure, gcp, kubernetes, otel, oneagent) with `NetworkError`. Those failures fall through to the `install_failed` catch-all.
- Flush events fired outside of a command `RunE` (e.g. from `WatchIngestWithEvent` mid-session callbacks). Those remain fire-and-forget goroutines; flush covers the events fired at command boundaries.

## Decisions

### Flush hook: `Execute()`, not `PersistentPostRun`

Cobra skips `PersistentPostRun` when `RunE` returns an error. The only hook that fires on every exit path — success, error, cancel, CTRL+C — is the code immediately after `rootCmd.Execute()` returns in `cmd/root.go`. The flush call belongs there.

- Alternative considered: `PersistentPostRun` on root. Skipped on error paths — discarded.
- Alternative considered: `defer` in each `RunE`. Requires touching every handler twice per change, and still misses CTRL+C on long-running commands. Discarded.

### WaitGroup approach: goroutines tracked, not queued

`fireSelfMonitoringEvent` keeps the goroutine-based send — events are dispatched the moment they are fired, so the Dynatrace ingestion timestamp reflects when each event actually occurred. This matters because the self-monitoring dashboard queries on ingestion time; buffering all events to send at process exit would collapse `st=inv` and `st=fai` to the same timestamp, making duration calculation and event ordering impossible.

Instead of a queue, a package-level `sync.WaitGroup` tracks all in-flight sends. Each goroutine registers with the WaitGroup before spawning and signals done when the HTTP call completes. `Flush(timeout)` waits on the WaitGroup for up to `timeout`, then returns — if the timeout fires, remaining goroutines are accepted as lost.

- Alternative considered: enqueue all events and send at flush time. Solves the lost-event problem but assigns all events the same ingestion timestamp, breaking dashboard timing. Discarded.
- Alternative considered: add explicit `startTime` to the payload to work around the ingestion timestamp issue. Requires the dashboard to query `startTime` rather than ingestion time, which is not the current behaviour. Discarded.

### Typed error types in `pkg/installer`, `ClassifyError` in `pkg/selfmonitoring`

The typed error types (`AuthError`, `ConfigError`, `DependencyMissingError`, etc.) are defined in `pkg/installer` — that is where they originate and where they are returned. `ClassifyError` lives in `pkg/selfmonitoring`, because classification is a selfmonitoring concern: it translates Go errors into telemetry values. `pkg/selfmonitoring` imports `pkg/installer` for the typed error types; the dependency is one-directional and introduces no cycle.

- Alternative considered: `ClassifyError` in `pkg/installer`. Works, but forces a selfmonitoring concern into the installer package. Discarded.
- Alternative considered: a new `pkg/errors` package for the typed error types. Maximally clean, but adds a package for a small amount of code. Deferred — can be extracted later if the typed errors are reused beyond selfmonitoring.

The taxonomy categories, their additional attributes, and the conditions under which each applies:

| `error.type` | Additional attributes | When |
|---|---|---|
| `user_cancelled` | — | User declined Y/N prompt or typed `0` at recommendation menu |
| `auth_error` | `auth.failure_reason`: `invalid_token`, `environment_not_reachable`, or `authentication_failed` | Authentication failed, invalid or missing token, environment unreachable |
| `config_error` | `config.missing_fields`: array of missing field names (`DT_ENVIRONMENT`, `DT_PLATFORM_TOKEN`, `project_path`) | Missing environment URL, missing platform token, or project path not found before install could start |
| `dependency_missing` | `dependency.name`: name of the missing binary | A required external tool was not found |
| `network_error` | `network.failure_reason`, `network.url` (if available) | Connection timeout, environment not reachable, or unexpected HTTP response |
| `install_failed` | `install.step`: free-text name of the step that failed | Install execution failed; catch-all for failures not matching a more specific category |
| `platform_unsupported` | — | Current OS or architecture is not supported by the selected install method |

Classification priority (first match wins): `user_cancelled` → `auth_error` → `config_error` → `dependency_missing` → `network_error` → `platform_unsupported` → `install_failed` → fallback `install_failed`. The fallback ensures every error produces a taxonomy value, even unrecognised ones.

### Error values are abbreviated in the User-Agent, full in the event body

The `User-Agent` is capped at 64 characters by the HAProxy capture limit, and that budget has to absorb fields added later. The longest taxonomy value, `platform_unsupported`, spends 20 of those characters on its own. Each value therefore gets a 3-character code in the header:

| Body `error` | `er=` |
|---|---|
| `user_cancelled` | `ucl` |
| `auth_error` | `aut` |
| `config_error` | `cfg` |
| `dependency_missing` | `dep` |
| `network_error` | `net` |
| `install_failed` | `ifl` |
| `platform_unsupported` | `plt` |

The body keeps the full value, which makes it the stable query surface: the header encoding can be shortened or re-keyed later to free budget without any dashboard query changing. This only holds while every header field is also present in the body, so `type` was added to the body alongside the fields already mirrored there.

- Alternative considered: abbreviate in both places. Discarded — it saves nothing (the body has no size limit) and forces queries to decode codes.
- Alternative considered: leave the header unabbreviated. Discarded — worst case reaches 74 characters with a snapshot version string, over the capture limit.

### Two new terminal step values: `StepFailed` and `StepCancelled`

`StepFailed` is the terminal stage for all errors. `StepCancelled` is used exclusively when the user declines a confirmation prompt or selects `0` at the recommendation menu. With these two additions, the dashboard can determine outcome from the step value alone, without inspecting the error field.

### One terminal event per `RunE` return

Every command handler fires exactly one terminal event before every `return` — `StepCancelled` for `ErrInstallCancelled`, `StepFailed` with a classified error for all other errors, `StepCompleted` on success. Auth and config errors returned at the top of `RunE` are covered by the same pattern; `ClassifyError` produces the right type regardless of where in the handler the error originated.

### Typed errors wrap, not replace, existing display strings

Auth, config, platform-unsupported, and dependency-missing errors replace opaque `fmt.Errorf` strings with typed structs. In each case, the existing human-readable message is preserved via wrapping — callers that display or assert on the message text are unaffected. The typed wrapper is used only for programmatic classification.

`ErrPlatformUnsupported` is a sentinel value rather than a struct because the taxonomy specifies no additional attributes for that error type.

## Risks / Trade-offs

- `config_error` with missing `DT_ENVIRONMENT` is always lost — no destination URL. This is a known limitation with no workaround.
- `auth_error` events are still sent, but the Events v2 call itself fails: the same invalid or rejected token is used to deliver the self-monitoring event to the same tenant. The event is nonetheless observable, because the request reaches the tenant's HAProxy and its `User-Agent` capture records the attempt. Only the tenant URL is required to send; a missing or rejected token never suppresses the attempt.
- Credential resolution moves to the call path of `fireSelfMonitoringEvent`. `getDtEnvironment()` reads env vars and flags only — no I/O — so the performance cost is negligible.
- The 200ms flush cap is imperceptible at the end of commands that take seconds. For fast commands (version, help), the extra wait is at most the HTTP RTT to the tenant, which typically completes well within the cap.
- Wrapping dependency-missing errors preserves existing human-readable messages in `Error()`, so display output and test assertions against those messages are unaffected.
