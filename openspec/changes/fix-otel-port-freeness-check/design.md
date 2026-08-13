# Design

## Context

`generateOtelConfig` in [pkg/installer/otel/collector.go](../../../pkg/installer/otel/collector.go) allocates the ports
the Dynatrace OTel Collector will bind by calling `findFreePort(startPort)`, which returns the lowest port at or above
`startPort` on which it can start accepting connections. Today it checks the hostname `localhost`:

```go
addr := fmt.Sprintf("localhost:%d", port)
l, err := net.Listen("tcp", addr)
```

The rendered config ([otel.tmpl](../../../pkg/installer/otel/otel.tmpl)) binds two different addresses depending on the
receiver: `otlp` and `health_check` bind `0.0.0.0` explicitly; the Prometheus telemetry reader binds `localhost`, which
resolves to `127.0.0.1` or `::1` depending on the system. `net.Listen("tcp", "localhost:<port>")` only ever listens on
one of those, never on `0.0.0.0`. A listener already bound to `0.0.0.0` and a later attempt to bind a specific loopback
address on the same port can coexist (they are not the same address), so checking only `localhost` never notices that
`0.0.0.0` is already taken. The Dynatrace collector then tries to bind that same port for real, gets an "address
already in use" error from the operating system, and exits immediately.

Reproduced live: with a foreign collector bound to `0.0.0.0:4317`/`4318`, `install otel` allocated `grpc=4317
http=4318` (unchanged, wrongly free) while the metrics port was correctly moved to the next one, because its own
conflict happened to be on `localhost` on both sides (the foreign collector's own telemetry reader also binds
`localhost`, so the two probes landed on the same address). The Dynatrace collector then crashed with `exit status 1`
on startup.

## Goals / Non-Goals

**Goals:**

- Make `findFreePort` report a port as free only when it is free on every address the rendered config will actually
  bind, so the collector started afterward never fails to bind a port allocation reported as available.

**Non-Goals:**

- Changing the allocation algorithm itself (lowest free port, de-duplicated against the other chosen ports) or any
  default port value. That logic is already correct.
- Detecting or reasoning about any other collector on the host. This fix makes port allocation accurate; it has no
  opinion on whose process holds a port.
- Handling any address family explicitly. No receiver in `otel.tmpl` binds an IPv6 address by name; the fix works by
  matching each check to what the config actually binds (`0.0.0.0`, or the literal hostname `localhost`), not by
  reasoning about IPv4 versus IPv6.

## Decisions

### Check `0.0.0.0` and the hostname `localhost` itself, one check per address the config actually binds

`findFreePort` now checks two addresses per candidate port: the literal address `0.0.0.0` (what `otlp`/`health_check`
bind) and the literal hostname `localhost` (what the Prometheus telemetry reader binds to, whatever that resolves to
on the current system). A port is free only when both checks succeed. This is implemented as a small `canBindPort(host
string, port int) bool` helper so the two checks share one code path.

**Alternative considered and rejected: check only `0.0.0.0`.** This was the first version of the fix, and it missed a
real case: a wildcard bind on `0.0.0.0` and a specific bind on a loopback address can coexist on the same port
(confirmed empirically; they are genuinely different addresses, not overlapping ones), so a `0.0.0.0`-only check cannot
see a conflict on whatever address `localhost` resolves to. Checking `0.0.0.0` alone would have reintroduced the same
class of bug for the telemetry reader's port that this change fixes for the OTLP and health-check ports.

**Alternative considered and rejected: check `0.0.0.0` and a hardcoded `127.0.0.1`.** Closer, but still wrong: which
loopback address `localhost` resolves to (`127.0.0.1` or `::1`) depends on the system, and hardcoding one guess means
the check silently stops matching the real bind target on a system that resolves the other way. Probing the literal
hostname `localhost`, exactly as the telemetry reader's own config does, removes the guess entirely: whatever address
the real bind ends up using, the check uses the same one, because it asks the same question the same way.

### Remove the misfiled requirement in `otel-collector-update` rather than leaving it stale

`openspec/specs/otel-collector-update/spec.md` already had a requirement titled "Generated config ports must not
conflict with already-running collectors," describing `findFreePort(8888)` and a `localhost`-only probe. Code review
caught that declaring this change's "Modified Capabilities" as empty was wrong: that requirement exists and would
have been left stale and partly contradicted by this fix.

Reconciling in place (editing that requirement where it sits) was rejected: `otel-collector-update`'s own overview
says the capability patches an *existing* config; it never generates one or allocates ports.
`findFreePort`/`generateOtelConfig` are called only from `prepareCollectorPlan`, which only `install otel` uses. The
requirement was describing `install otel`'s behavior under the wrong capability's name. Editing it in place would
have preserved that misfiling. Instead it is removed from `otel-collector-update` with a reason and migration
pointing at `otel-collector-port-allocation`, which already states the corrected, complete version of the same
requirement where it actually belongs.

### Thread the allocated OTLP HTTP port through verification instead of hardcoding it

Writing task 2.4's regression test exposed that verification (`waitForOtelCollectorReady`, `sendOtelVerificationLog`,
`verifyOtelInstall`) hardcoded the OTLP HTTP receiver's default port. With the test's decoy occupying that default
port, `install otel` would have allocated a different one for the real collector, and hardcoded verification would
have probed/posted to the decoy instead. This is the same class of bug as the port-freeness defect: code that assumes
a fixed port instead of the one `generateOtelConfig` actually chose.

`install otel` has the allocated port on hand (`collectorPlan.httpPort`), so it is threaded straight through.
`update otel` patches a config it did not generate and has no allocated-port value to reuse, so `extractOtlpHTTPPort`
reads the port back out of the patched config's `receivers.otlp.protocols.http.endpoint`, falling back to the default
only when that cannot be parsed, matching the pattern used for the other "config might not be one dtwiz wrote"
cases in `update.go`.

## Risks / Trade-offs

- **The check is strictly more conservative than before**: it can only reject a port the old check would have
  accepted (never the reverse), so a host where the old check already worked correctly sees no behavior change.
- **Two connection checks per candidate port instead of one.** Negligible: `findFreePort` runs a handful of times per
  install, not inside a loop that executes rapidly.
