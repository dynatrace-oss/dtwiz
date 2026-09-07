# Design

## Context

dtwiz currently produces no signal about its own usage. Pipeline propagation has been confirmed: an event originating inside the tool reaches the Dynatrace backend and can be correlated. This change introduces the foundational `pkg/selfmonitoring` package and wires it into the command lifecycle, establishing the pattern that future self-monitoring work will build on. The Events v2 API (`/api/v2/events/ingest`) is the ingestion path used: it requires no pre-existing monitored entity and the confirmation exercise validated it end-to-end.

## Goals / Non-Goals

**Goals:**

- Send one `CUSTOM_INFO` event per invocation of any dtwiz command, carrying per-invocation metadata in the event body properties and dual HTTP headers (`User-Agent` and `Tab-Id`) for maximum query flexibility.
- Event body `properties` always-present keys: `e` (exec ID), `c` (cmd), `st` (step). Optional keys `s` (sub), `er` (err), `t` (type) are included only when non-empty; the Events v2 API rejects null or empty-string property values.
- User-Agent format: `dtwiz/<version>;c=<cmd>;st=<step>[;s=<sub>][;er=<err>][;t=<type>]` — maximum 64 chars (HAProxy capture limit).
- Tab-Id format: `<execid>;m=<mode>;o=<os>` — maximum 16 chars (HAProxy capture limit).
- Mode values: `deb` (debug), `tty`, `ntt` (non-tty), all 3 chars for uniform formatting.
- OS values: `mac` (darwin), `lin` (linux), `win` (windows), all 3 chars.
- Step value abbreviations: `inv` (invoked); future steps will follow the same pattern.
- Embed a stable `dtwiz-monitoring: dtwiz-start` custom HTTP header so the event can also be found by header-matching in a pipeline query.
- Keep the feature invisible to users: no output, no user-facing flags, no error surfacing.
- Gate the code path behind a feature flag so it never runs unless explicitly opted in.

**Non-Goals:**

- Collect command arguments or user-identifying data beyond what is listed above.
- Replace or pre-empt a future production self-monitoring design.
- Persist state across invocations.
- Handle retries or guaranteed delivery.

## Decisions

- Distribute metadata across event body properties and two HTTP headers: `User-Agent` (operation identity: cmd, step, sub, err, type) and `Tab-Id` (execution context: exec ID, mode, OS).
  - Rationale: HAProxy has separate capture limits (64 chars for User-Agent, 16 chars for Tab-Id). Splitting respects both limits while maintaining full query surface. All data appears in both event body and headers for dual-surface query flexibility.
  - Alternative considered: all data in one header. Rejected because it would exceed the 64-char User-Agent limit when all optional fields are present.
- Generate a per-process execution ID (`e`) as 1 random byte formatted as 2 hex chars (256 combinations) via `crypto/rand` in `selfmonitoring.init()`.
  - Rationale: 2 chars keeps Tab-Id short (14 chars worst case); 256 combinations is sufficient to correlate events within a single dtwiz session without persistent storage or a clock-based ID.
- Derive `cmd` and `sub` from the cobra `*cobra.Command` passed to `PersistentPreRun`, then normalize via lookup maps to short abbreviations (max 3 chars for commands, max 4 for subcommands). `sub` is omitted when the command has no subcommand.
  - Command map: `install`→`ins`, `uninstall`→`uni`, `update`→`upd`, `analyze`→`ana`, `recommend`→`rec`, `status`→`sta`, `watch`→`wch`, `setup`→`set`, `version`→`ver`.
  - Sub map: `otel`→`otel`, `otel-collector`→`otlc`, `otel-python`→`otlp`, `otel-node`→`otln`, `otel-java`→`otlj`, `kubernetes`→`k8s`, `oneagent`→`oa`, `gcp`→`gcp`, `azure`→`az`, `aws`→`aws`, `aws-lambda`→`awsl`, `docker`→`dock`, `demo`→`demo`, `self`→`self`.
  - Unknown names pass through unchanged so new subcommands don't silently drop their identity.
  - Rationale: short fixed tokens keep headers compact; the lookup approach avoids coupling command-name parsing to string slicing heuristics.
- Normalize `mode` to 3-char codes: `deb` (debug), `tty`, `ntt` (non-tty) using `debugFlag` and `golang.org/x/term.IsTerminal`.
  - Rationale: `golang.org/x/term` is already a direct dependency; it provides correct cross-platform tty checks. Uniform 3-char codes simplify Tab-Id parsing and formatting.
- Map `runtime.GOOS` to 3-char OS codes: `mac` (darwin), `lin` (linux), `win` (windows).
  - Rationale: uniform 3-char codes simplify Tab-Id parsing; all values are short and descriptive.
- Register `SelfMonitoringPoC` in `pkg/featureflags/` with env var `DTWIZ_SELF_MONITORING_POC` rather than checking the env var manually in `cmd/root.go`.
  - Rationale: the registry is the single place that handles env-var parsing, case-insensitive `true`/`1` normalisation, CLI override precedence, and the `List()` introspection surface. Duplicating that logic in a one-off `os.Getenv` call is unnecessary and bypasses the standard resolution order.
  - Alternative considered: manual `os.Getenv` check inline. Rejected because it would duplicate resolution logic already in `resolveFlag` and diverge from every other flag in the codebase.
- Define only `StepInvoked` as a constant in `pkg/selfmonitoring` for now; pass `stepID string` explicitly from every call site.
  - Rationale: the lifecycle vocabulary is owned by the selfmonitoring package; callers choose the step at each point in the command flow rather than embedding magic strings. Additional step constants (`confirmed`, `completed`, `failed`, `cancelled`) will be added when the corresponding call sites are wired in follow-on stories.
- Fire the event in `PersistentPreRun` with `StepInvoked`.
  - Rationale: `PersistentPreRun` fires as soon as the command is dispatched, before any user prompt or network call, giving a signal for every invocation regardless of outcome. Future steps (confirmed, completed, failed, cancelled) will be fired at their respective points in the command lifecycle.
  - Alternative considered: fire only after a successful install. Rejected because it would miss crashes and early exits.
- Use a package-level `http.Client` with a 3-second timeout rather than `http.DefaultClient` or the typed `client.Client`.
  - Rationale: the selfmonitoring package is a PoC with no dependency on the installer client stack. Using the typed client would couple the package to credential resolution, which is handled one level up in `cmd/root.go`. A dedicated client with a short timeout prevents a slow or unreachable endpoint from blocking `PersistentPreRun` indefinitely.
  - Alternative considered: pass a `*client.Client`. Rejected because it would require the package to import `pkg/client`, creating an unnecessary coupling at this stage.
- Duplicate the `authHeader` logic inside `pkg/selfmonitoring` rather than importing `pkg/installer`.
  - Rationale: importing `pkg/installer` for one two-line helper would pull in its full dependency surface and blur the package boundary.

## Risks / Trade-offs

- Credentials must be resolvable at `PersistentPreRun` time; if the user has not configured them the call silently no-ops, which is the correct behavior.
- The event will fire on every mutating command invocation while the feature flag is set, including `--dry-run` runs, since the check happens in `PersistentPreRun` before `--dry-run` is evaluated. Acceptable at this stage; a production design would filter dry-run invocations.
- A slow or unreachable endpoint is bounded by the 3-second timeout on the package-level HTTP client; the pre-run hook will not block beyond that.
