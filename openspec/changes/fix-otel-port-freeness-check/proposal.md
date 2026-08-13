# Proposal

## Why

`install otel`'s port allocation checks whether a port is free by testing the hostname `localhost`. That only tells you
whether the port is free on whichever loopback address `localhost` happens to resolve to (`127.0.0.1` or `::1`,
depending on the system), never on `0.0.0.0`, the address that means "every network interface." A listener already
bound to `0.0.0.0` and a new attempt to bind a specific loopback address can coexist on the same port, so testing only
`localhost` never notices that `0.0.0.0` is already taken. The rendered collector config binds `0.0.0.0` explicitly
for its OTLP and health-check receivers, so when this happens the collector then fails to bind that same port for
real, exits immediately with no clear diagnostic, and the install appears broken. The port allocation logic itself
(picking the lowest free port and avoiding collisions between the chosen ports) is correct; only the freeness check is
wrong.

## What Changes

- Port freeness checks in `pkg/installer/otel/collector.go` now test two addresses per candidate port: `0.0.0.0` (what
  the `otlp` and `health_check` receivers actually bind) and the hostname `localhost` itself (what the Prometheus
  telemetry reader binds to). A port is only considered free when both checks succeed.
- No change to the allocation algorithm (lowest free port at or above each default, de-duplicated against the other
  chosen ports) or to any of the default port values.
- Fixed a related bug surfaced while writing the regression test (task 2.4): `install otel`'s post-install
  verification (`waitForOtelCollectorReady`, `sendOtelVerificationLog`, `verifyOtelInstall`) hardcoded the OTLP HTTP
  receiver's default port instead of using the port actually allocated for it. Whenever that default port was
  occupied, causing a different port to be allocated, exactly the scenario this change's port-freeness fix targets,
  verification silently probed and posted to the wrong port. `install otel` now threads the allocated port through to
  verification. `update otel` also reads the OTLP HTTP port back out of the config it patches instead of assuming the
  default, since it patches an existing config it did not generate; this is a small defensive fallback around
  `update otel`'s existing verification behavior, not a change to its documented contract (same messages, same
  best-effort caveat for containers), so it does not need its own capability delta.

## Capabilities

### New Capabilities

- `otel-collector-port-allocation`: `install otel` allocates the ports its collector will bind by checking whether
  each candidate port is actually free on the addresses the rendered configuration binds, so the collector started
  afterward does not fail to bind a port that port allocation reported as available.

### Modified Capabilities

- `otel-collector-update`: removes a requirement that described this same port-conflict detection, but misfiled under
  the update/patch capability. `findFreePort` is only ever called from `install otel`'s config generation
  (`prepareCollectorPlan`); `update otel`'s config-patching flow, which this capability actually covers, never calls
  it. The requirement is superseded by `otel-collector-port-allocation`, which also corrects it: the removed version
  only covered the Prometheus metrics port's `localhost` binding and did not account for the `otlp`/`health_check`
  receivers binding `0.0.0.0`, which is the defect this change fixes.

## Impact

- **Code:** `pkg/installer/otel/collector.go` (`findFreePort`, new `canBindPort` helper, and the verification helpers
  now parameterized by the collector's actual OTLP HTTP port), `pkg/installer/otel/update.go` (new
  `extractOtlpHTTPPort` to read that port back out of a patched config), and `pkg/installer/otel/collector_test.go`
  for the fix and its unit tests; `test/e2e/otel_test.go` (build tag `integration`) for an end-to-end regression test
  against the real collector binary.
- **Affected commands:** `install otel` directly; any future command that reuses `findFreePort` for port allocation
  inherits the fix automatically, since it is the single implementation.
- **Feature flags:** none. This is not gated: it is a correctness fix to logic that already runs on every
  `install otel`.
- **Dependencies:** none new.
- **Rollback:** low risk, revertible by reverting the single commit. The fixed check is strictly more conservative
  than the old one (it can only reject a port the old check would have accepted, never the reverse), so there is no
  behavior change on a host where the old check already worked correctly.
