# Tasks

## 1. selfmonitoring Package

- [x] 1.1 Create `pkg/selfmonitoring/selfmonitoring.go` with `SendEvent(classicURL, token string, params EventParams) error`.
- [x] 1.2 Define `EventParams` struct: `CmdID`, `SubID`, `StepID` (defaults to `StepInvoked`), `Mode`, `Err`, `Type`.
- [x] 1.3 Define step ID constant `StepInvoked = "inv"`; other lifecycle steps (`confirmed`, `completed`, `failed`, `cancelled`) are reserved for future stories.
- [x] 1.4 Generate a per-process `ex` (execution ID) as 1 random byte / 2 hex chars with `crypto/rand` in `init()`; fall back to `"00"` on error.
- [x] 1.5 Build `User-Agent` as `dtwiz/<version>;c=<cmd>;st=<step>[;s=<sub>][;er=<err>][;t=<type>]` (max 64 chars). Append optional segments only when non-empty.
- [x] 1.6 Build `Tab-Id` as `<execid>;m=<mode>;o=<os>` (max 16 chars). ExecID is positional (2 hex chars); mode and OS are always 3 chars each.
- [x] 1.7 Map `runtime.GOOS` to 3-char codes: `mac` (darwin), `lin` (linux), `win` (windows).
- [x] 1.8 Normalize `mode` to 3-char codes: `deb` (debug), `tty`, `ntt` (non-tty).
- [x] 1.9 Build event body JSON with single/two-letter keys: always-present `e` (exec ID), `c` (cmd), `st` (step); optional `s` (sub), `er` (err), `t` (type) when non-empty.
- [x] 1.10 POST the event body to `/api/v2/events/ingest` with headers: `User-Agent` (operation identity), `Tab-Id` (execution context), `dtwiz-monitoring: dtwiz-start`.
- [x] 1.11 Implement local `authHeader()` helper: `dt0c01.*` tokens use `Api-Token`, all others use `Bearer`.
- [x] 1.12 Swallow all errors at `logger.Debug` level; never return them to the caller.
- [x] 1.13 Define property key constants: `propExecID`, `propCmd`, `propStep`, `propSub`, `propErr`, `propType`.

## 2. Feature Flag Registration

- [x] 2.1 Add `SelfMonitoringPoC` to the `Flag` const in `pkg/featureflags/featureflags.go`.
- [x] 2.2 Register it in the registry with name `self-monitoring-poc`, env var `DTWIZ_SELF_MONITORING_POC`, default `false`.

## 3. Command Wiring

- [x] 3.1 Add `fireSelfMonitoringEvent(cmd *cobra.Command, stepID string)` in `cmd/root.go`: gate on `featureflags.IsEnabled`, build `EventParams` synchronously, then launch a background goroutine to resolve credentials and call `selfmonitoring.SendEvent`.
- [x] 3.2 Add `buildEventParams(cmd, stepID)`: derives `CmdID`/`SubID` from cobra command tree, sets `StepID` from caller, calls `resolveMode()`.
- [x] 3.3 Add `deriveCommandIDs(cmd)` with `normCmd`/`normSub` lookup maps: abbreviate command names to max 3 chars and sub names to max 4 chars; unknown names pass through unchanged.
- [x] 3.4 Add `resolveMode()`: returns `"deb"` if `debugFlag` is set, `"tty"` if `golang.org/x/term.IsTerminal(stdout)`, else `"ntt"` (all 3 chars).
- [x] 3.5 Call `fireSelfMonitoringEvent(cmd, selfmonitoring.StepInvoked)` in `PersistentPreRun` of `rootCmd`, `installCmd`, `updateCmd`, and `uninstallCmd`; this covers all commands including `watch` and `analyze`.
