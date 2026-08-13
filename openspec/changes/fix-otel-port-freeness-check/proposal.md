## Why

`install otel`'s port allocation checks whether a port is free by probing the hostname `localhost`. On systems where
`localhost` resolves to the IPv6 loopback (`::1`) ahead of `127.0.0.1` (macOS is a common case), that probe can report
a port as free when it is actually occupied on the IPv4 wildcard address `0.0.0.0`, because `[::1]:<port>` and
`0.0.0.0:<port>` are different sockets. The rendered collector config binds `0.0.0.0` explicitly for its OTLP and
health-check receivers, so when this happens the collector then fails to bind that same port for real, exits
immediately with no clear diagnostic, and the install appears broken.

This was reproduced live: with another OTel Collector process already bound to `0.0.0.0:4317`/`4318`, `install otel`
logged `grpc=4317 http=4318` (unchanged from the defaults, wrongly reported free) while the metrics port was correctly
bumped away from its own conflict. The Dynatrace collector then started and exited immediately with `exit status 1`.
The port allocation logic itself (picking the lowest free port and avoiding collisions between the chosen ports) is
correct; only the freeness check is wrong.

## What Changes

- Port freeness checks in `pkg/installer/otel/collector.go` probe literal addresses instead of the ambiguous hostname
  `localhost`: `0.0.0.0` (what the `otlp` and `health_check` receivers actually bind) and `127.0.0.1` (what the
  Prometheus telemetry reader binds via `localhost`). A port is only considered free when both checks succeed.
- No change to the allocation algorithm (lowest free port at or above each default, de-duplicated against the other
  chosen ports) or to any of the default port values.

## Capabilities

### New Capabilities

- `otel-collector-port-allocation`: `install otel` allocates the ports its collector will bind by checking whether
  each candidate port is actually free on the addresses the rendered configuration binds, so the collector started
  afterward does not fail to bind a port that port allocation reported as available.

### Modified Capabilities

(none: no existing spec covers collector port allocation)

## Impact

- **Code:** `pkg/installer/otel/collector.go` (`findFreePort`, new `canBindPort` helper) and
  `pkg/installer/otel/collector_test.go`. No other files.
- **Affected commands:** `install otel` directly; any future command that reuses `findFreePort` for port allocation
  inherits the fix automatically, since it is the single implementation.
- **Feature flags:** none. This is not gated: it is a correctness fix to logic that already runs on every
  `install otel`.
- **Dependencies:** none new.
- **Rollback:** low risk, revertible by reverting the single commit. The fixed check is strictly more conservative
  than the old one (it can only reject a port the old check would have accepted, never the reverse), so there is no
  behavior change on a host where the old check already worked correctly.
