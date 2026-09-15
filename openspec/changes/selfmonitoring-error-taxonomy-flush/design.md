# Design

## Context

`pkg/selfmonitoring` sends per-invocation `CUSTOM_INFO` events to the Dynatrace Events v2 API. The `fireSelfMonitoringEvent` function in `cmd/selfmonitoring.go` spawns a goroutine that calls `getDtEnvironment()` and then `selfmonitoring.SendEvent`. This goroutine is detached — if the process exits before the HTTP call completes, the event is silently lost.

`EventParams` already has an `Err string` field and the `er=` segment in the User-Agent header, but no call site ever populates it. `StepInvoked` and `StepCompleted` are the only step constants; there is no `StepFailed` or `StepCancelled`.

Errors throughout the codebase are opaque `fmt.Errorf` strings. There are no typed error types, no shared platform-unsupported sentinel, no structured dependency-missing error. `cmd/auth.go` distinguishes error cases by embedded string prefixes that cannot be matched programmatically.

## Goals / Non-Goals

**Goals:**

- Enqueue self-monitoring events synchronously and flush them (≤200ms) before process exit on every code path.
- Classify every command failure into a structured `ErrorType` and extract additional attributes where specified by the VI taxonomy.
- Fire a terminal event (`StepFailed` or `StepCancelled`) at every `RunE` return point across all command handlers.
- Replace opaque error strings in auth validation, dependency checks, and platform-unsupported guards with typed errors that carry machine-readable fields.

**Non-Goals:**

- Implement the `confirmed` stage (fired when the user passes Y/N). That is part of the onboarding funnel, not the error taxonomy.
- Classify HTTP failures inside installer packages (aws, azure, gcp, kubernetes, otel, oneagent) with `NetworkError`. Those failures fall through to the `install_failed` catch-all, which is the designed behaviour per the VI.
- Flush events fired outside of a command `RunE` (e.g. from `WatchIngestWithEvent` mid-session callbacks). Those remain fire-and-forget goroutines; flush covers the events fired at command boundaries.

## Decisions

### Flush hook: `Execute()`, not `PersistentPostRun`

Cobra skips `PersistentPostRun` when `RunE` returns an error. The only hook that fires on every exit path — success, error, cancel, CTRL+C — is the code immediately after `rootCmd.Execute()` returns in `cmd/root.go`. The flush call belongs there.

- Alternative considered: `PersistentPostRun` on root. Skipped on error paths — discarded.
- Alternative considered: `defer` in each `RunE`. Requires touching every handler twice per change, and still misses CTRL+C on long-running commands. Discarded.

### Enqueue model: credentials resolved at enqueue time

`fireSelfMonitoringEvent` currently resolves credentials inside the goroutine. Moving credential resolution to the call site keeps each queued event ready to send with a resolved URL and token. `getDtEnvironment()` reads env vars and flags only; it is essentially free.

If credential resolution fails at enqueue time, the event is silently dropped — same behaviour as today. Flush sends all queued events concurrently with a timeout and is silent: no output, no spinner.

### `ClassifyError` lives in `pkg/installer`

The typed error types are defined in `pkg/installer` (where installers live). The classifier must be in the same package to avoid a circular import — `pkg/selfmonitoring` already imports `pkg/installer` for URL helpers. The `cmd` layer is the only place that bridges both packages: it calls `ClassifyError` and passes the result into `EventParams`.

Classification priority (first match wins): `user_cancelled` → `auth_error` → `config_error` → `dependency_missing` → `network_error` → `platform_unsupported` → `install_failed` → fallback `install_failed`. The fallback ensures every error produces a taxonomy value, even unrecognised ones.

### Two new terminal step values: `StepFailed` and `StepCancelled`

`StepFailed` is the terminal stage for all errors. `StepCancelled` is used exclusively when the user declines a confirmation prompt or selects `0` at the recommendation menu. With these two additions, the dashboard can determine outcome from the step value alone, without inspecting the error field.

### One terminal event per `RunE` return

Every command handler fires exactly one terminal event before every `return` — `StepCancelled` for `ErrInstallCancelled`, `StepFailed` with a classified error for all other errors, `StepCompleted` on success. Auth and config errors returned at the top of `RunE` are covered by the same pattern; `ClassifyError` produces the right type regardless of where in the handler the error originated.

### Typed errors wrap, not replace, existing display strings

Auth, config, platform-unsupported, and dependency-missing errors replace opaque `fmt.Errorf` strings with typed structs. In each case, the existing human-readable message is preserved via wrapping — callers that display or assert on the message text are unaffected. The typed wrapper is used only for programmatic classification.

`ErrPlatformUnsupported` is a sentinel value rather than a struct because the taxonomy specifies no additional attributes for that error type.

## Risks / Trade-offs

- `config_error` with missing `DT_ENVIRONMENT` is always lost — no destination URL. This is a known limitation acknowledged by the VI and requires no mitigation.
- Credential resolution moves to the call path of `fireSelfMonitoringEvent`. `getDtEnvironment()` reads env vars and flags only — no I/O — so the performance cost is negligible.
- The 200ms flush cap is imperceptible at the end of commands that take seconds. For fast commands (version, help), the extra wait is at most the HTTP RTT to the tenant, which typically completes well within the cap.
- Wrapping dependency-missing errors preserves existing human-readable messages in `Error()`, so display output and test assertions against those messages are unaffected.
