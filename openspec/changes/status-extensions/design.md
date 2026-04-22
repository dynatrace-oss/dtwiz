# Design: Status Extensions

## Context

All existing installer code creates `resty` clients inline with no shared configuration. There is no central place to apply retry logic, timeouts, or request/response debug logging. The `dtwiz status` command validates credentials but does not probe any functional API endpoints.

## Goals / Non-Goals

**Goals:**

- Provide a shared HTTP client construction path (`NewHTTPClient()`) that all `cmd/` code can use, with consistent retry, timeout, and debug-logging behaviour
- Add an optional `--extensions` probe to `dtwiz status` to verify both Classic and Platform Extensions APIs are reachable

**Non-Goals:**

- Migrating existing installer code in `pkg/installer/` to use the new client — that is a separate refactor
- Exposing extension content (names, versions) — counts are sufficient for connectivity verification

## Decisions

### 1. Client lives in `cmd/`, not `pkg/installer/`

The new `Client` type is a command-layer concern: it reads credentials from flag/env helpers (`environmentHint()`, `accessToken()`, `platformToken()`) that only exist in `cmd/`. Installer packages use their own credential-passing conventions. Mixing the two layers would require threading CLI flag values into packages that currently take plain strings.

### 2. Two sub-clients (`ClassicClient`, `PlatformClient`) on one `Client` struct

Commands frequently need both API families in a single operation. Exposing them as named fields on a parent struct makes the duality explicit and avoids passing two separate clients through call chains.

### 3. Retry on 429 and 5xx only

Retrying on transient server errors is safe. Retrying on 4xx (e.g. 401, 403, 404) would mask configuration problems and slow down error feedback.

### 4. Sensitive header redaction and body size cap in debug output

Authorization tokens must never appear in logs. The redaction list (`authorization`, `x-api-key`, `cookie`, `set-cookie`) is checked at output time, not at client construction, so it applies to all requests regardless of how the header was set.

Response bodies are capped at 2048 bytes. API responses can be large and may contain sensitive fields; arbitrary JSON field redaction would be brittle and hard to maintain. A size cap limits both the blast radius of accidental secret exposure and the volume of stderr noise. The cap is applied as a byte slice, not a JSON parse, to keep the implementation simple and unconditional.

### 5. `--extensions` as an opt-in flag, not default behaviour

Running two extra API calls on every `dtwiz status` invocation adds latency and requires both tokens to be configured. Making it opt-in preserves the fast default path.

## Risks / Trade-offs

- **[Token requirement]** `NewHTTPClient()` requires all three credentials (environment, access token, platform token). Commands that only need one token family cannot use it without providing all three. → Mitigation: `printExtensionsStatus()` calls `NewHTTPClient()` and surfaces a clear error if credentials are incomplete; the rest of `dtwiz status` still runs.
- **[6-minute timeout]** Generous timeout suits slow or rate-limited environments but means a hung request blocks the command for a long time. → Mitigation: acceptable for a diagnostic command; can be tuned later.
