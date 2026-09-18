# Design: Instrument Remaining Commands

## Context

Self-monitoring is an existing, feature-flag-gated mechanism (`featureflags.SelfMonitoringPoC`) that fires async telemetry events to the Dynatrace Events v2 API. The infrastructure is fully in place in `pkg/selfmonitoring` and `cmd/selfmonitoring.go`.

Root's `PersistentPreRun` already fires `fireInvokedEvent(cmd)` for all commands that do not define their own `PersistentPreRun`. This covers `analyze`, `recommend`, `status`, and `version` without any code change. What is missing for these commands is a terminal event — a signal that the command ran to completion, and whether it succeeded.

**Current event vocabulary:**

| Field   | Existing values                                              |
|---------|--------------------------------------------------------------|
| `StepID` | `"inv"` (invoked), `"com"` (completed), `"ana"` (analyze), `"rec"` (recommend), `"ist"` (install) |
| `SubID`  | method shortcodes from `normSubMap`                         |
| `Err`    | `"err"` on failure, empty on success                        |

The existing `StepCompleted = "com"` constant covers this use case. No new constants are needed.

**Event distinguishability:** Events from these commands are distinguished from setup's internal steps by `CmdID`: `"ana"`, `"rec"`, `"sta"`, `"ver"` vs `"set"`. There is no ambiguity even when step IDs overlap.

**Credential gap:** `fireSelfMonitoringEvent` resolves credentials internally. If `DT_ENVIRONMENT`/`DT_PLATFORM_TOKEN` are absent, the goroutine logs at debug level and returns silently. This means credential-related errors in `status` are naturally excluded — if credentials are broken, no event can be sent regardless.

## Goals / Non-Goals

**Goals:**

- Add a `StepCompleted` event at the natural exit point of each of the four commands
- Set `Err="err"` when the command's `Run`/`RunE` returns a non-nil error
- Keep event semantics consistent with the patterns established in `watch` and `setup`

**Non-Goals:**

- Adding new `StepID` constants (not needed)
- Adding new `normSubMap` entries (these are top-level commands with no subcommand)
- Fine-grained error categorization (deferred; `"err"` is sufficient)
- Instrumenting `install`, `update`, or `uninstall` (separate task)

## Decisions

### Single helper: `fireCompletedEvent(cmd, err)`

**Decision:** Add one new helper `fireCompletedEvent(cmd *cobra.Command, err error)` to `cmd/selfmonitoring.go`. It builds params using `buildEventParams(cmd, selfmonitoring.StepCompleted)`, sets `Err="err"` when `err != nil`, and calls `fireSelfMonitoringEvent`.

**Rationale:** Mirrors the existing `fireInvokedEvent(cmd)` pattern exactly. A single helper centralizes the error-to-Err mapping and keeps call sites in each command minimal — one line each.

**Alternatives considered:**

- Inline `buildEventParams` at each call site: more repetition, no benefit.
- Separate helpers per command: unnecessary given the uniform pattern.

### version uses `Run`, not `RunE` — no error to propagate

**Decision:** `version.go` uses `Run func(cmd, args)` (no error return). The completed event is fired at the end of `Run` with `err = nil`, always producing an empty `Err` field.

**Rationale:** `version` cannot fail — it only prints a string. Changing it to `RunE` would be scope creep. Calling `fireCompletedEvent(cmd, nil)` directly is correct and clear.

### Err tied to Go error return value only

**Decision:** `Err="err"` is set if and only if the command's `RunE` returns a non-nil error. Partial failures within a command (e.g., a failed credential check in `status` that still lets execution continue) do not set `Err`.

**Rationale:** Consistent with `fireSetupInstallEvent`, which also uses the returned error value. Partial-failure tracking would require threading internal state through the event call, adding complexity for marginal analytical value.

## Risks / Trade-offs

- **Goroutine lifetime:** Each `fireSelfMonitoringEvent` call launches a goroutine with a 3-second HTTP timeout. For very fast commands (e.g., `version`), the goroutine may outlive the process. This is the same accepted risk as `watch` and `setup`. Best-effort delivery is intentional.

- **`version` always reports success:** Since `version` cannot fail, its `StepCompleted` event will always have empty `Err`. This is correct behavior, not a gap.
