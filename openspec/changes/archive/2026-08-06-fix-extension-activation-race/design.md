# Design

## Context

The Dynatrace Extensions 2.0 hub install endpoint (`POST /platform/extensions/v2/extensions/{name}`)
returns **202 Accepted**, not 200 OK. The response body contains the installed version
but the extension is not immediately usable — the DT backend completes activation
asynchronously. The `Active` field on the extension version list (`GET /platform/extensions/v2/extensions/{name}`)
flips to `true` once activation is complete.

Previously, `installExtension()` returned `error` only and the caller had no way to
distinguish a fresh install from an already-installed idempotent no-op. Both cases
returned `nil`, so the caller always proceeded directly to monitoring configuration
creation.

This change also builds on `install-cloud-extensions-before-monitoring-config`, which
first introduced the `installExtension()` call in both install flows.

## Goals / Non-Goals

**Goals:**

- Eliminate the first-install race without adding unnecessary latency to the common
  (already-installed) path.
- Surface wait progress to the user so the terminal does not appear hung.
- Keep the polling logic testable via the injected `sleeper`.

**Non-Goals:**

- Retry monitoring configuration creation on failure (active-poll replaces blind retry).
- Change the update flow behavior (extensions are always active by update time).
- Configure the poll timeout via a flag (60 s is sufficient for observed activation times).

## Decisions

### Readiness signal: `Active` field

The `GET /platform/extensions/v2/extensions/{name}` response includes `"active": true`
on the version item once the hub install completes. This is the correct readiness signal
because it reflects the DT backend's own activation state, not a proxy (e.g. schema
availability or monitoring config acceptance).

Alternatives considered:

- **Retry `createMonitoring` on 400**: rejected — it mixes activation lag with a
  genuine bad-request error and gives no user-visible progress during the wait.
- **Fixed sleep**: rejected — wastes time when activation is fast (usually < 10 s) and
  is still a race condition at the boundary.
- **Poll `FetchExtensionSchema`**: rejected — the schema endpoint may return 200 before
  `CreateMonitoringConfiguration` accepts a config; `Active` is the authoritative signal.

### Interface change: `installExtension() (bool, error)`

Returning a bool from `installExtension()` is the minimal contract change needed to let
the caller decide whether to wait. The alternative — always waiting inside
`installExtension()` — would require threading the sleeper into the method, changing
the interface in a more invasive way and making the `sdkDTClient` depend on a
test-injectable sleep function it currently does not hold.

### Shared `IsExtensionActive` on `ExtensionClient`

Both Azure and GCP packages wrap `ExtensionClient`. Adding `IsExtensionActive` there
avoids duplicating the `Extension.Get()` call in each package and keeps extension-API
access centralised.

### Timeout: 60 s (12 × 5 s)

Observed activation times across dev and prod tenants are under 10 s. 60 s gives a
4–6× safety margin without committing users to a multi-minute wait on a true failure.
On timeout the flow logs debug info and proceeds; the subsequent `createMonitoring`
call will then surface the real error with the full API response.

### Update flows: ignore fresh-install signal

The update flow calls `installExtension()` to ensure the extension exists before
reconciling monitoring configs. At update time the extension is always already installed
(it was installed by the preceding `dtwiz install`), so `installExtension()` always
returns `false`. No wait is needed and the `_` discard is intentional.

## Risks / Trade-offs

- If a DT environment never sets `Active: true` (e.g. a backend bug), the 60 s poll
  exhausts and the flow proceeds to `createMonitoring`, which may fail. The error from
  the Extensions API is then surfaced directly to the user with no spurious timeout
  message — the timeout is silent (debug-level only).
- The `fakeDTClient` in tests returns `(false, nil)` from `installExtension()` so no
  wait loop runs in unit tests. The `isExtensionActive()` fake returns `(true, nil)`
  unconditionally; integration coverage for the wait loop relies on the injected
  `sleeper` being `noSleep` in tests that exercise the fresh-install path.
