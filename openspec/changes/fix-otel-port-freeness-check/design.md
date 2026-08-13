## Context

`generateOtelConfig` in [pkg/installer/otel/collector.go](../../../pkg/installer/otel/collector.go) allocates the ports
the Dynatrace OTel Collector will bind by calling `findFreePort(startPort)`, which returns the lowest port at or above
`startPort` on which it can open a TCP listener. Today it probes the hostname `localhost`:

```go
addr := fmt.Sprintf("localhost:%d", port)
l, err := net.Listen("tcp", addr)
```

The rendered config ([otel.tmpl](../../../pkg/installer/otel/otel.tmpl)) binds two different addresses depending on the
receiver: `otlp` and `health_check` bind `0.0.0.0` explicitly; the Prometheus telemetry reader binds `localhost`. On a
host where `localhost` resolves to the IPv6 loopback (`::1`) ahead of `127.0.0.1`, which is the default on macOS,
`net.Listen("tcp", "localhost:<port>")` opens a listener on `[::1]:<port>` only. That is a different socket from
`0.0.0.0:<port>`, so a port already occupied on `0.0.0.0` (by any process, including another vendor's OTel Collector)
is wrongly reported free. The Dynatrace collector then tries to bind that same port for real, gets `EADDRINUSE`, and
exits immediately.

Reproduced live: with a foreign collector bound to `0.0.0.0:4317`/`4318`, `install otel` allocated `grpc=4317
http=4318` (unchanged, wrongly free) while the metrics port was correctly bumped, because its own conflict happened to
be on `localhost`/`::1` on both sides. The Dynatrace collector then crashed with `exit status 1` on startup.

## Goals / Non-Goals

**Goals:**

- Make `findFreePort` report a port as free only when it is free on every address the rendered config will actually
  bind, so the collector started afterward never fails to bind a port allocation reported as available.

**Non-Goals:**

- Changing the allocation algorithm itself (lowest free port, de-duplicated against the other chosen ports) or any
  default port value. That logic is already correct.
- Detecting or reasoning about any other collector on the host. This fix makes port allocation accurate; it has no
  opinion on whose process holds a port.
- IPv6. No receiver in `otel.tmpl` binds an IPv6 address, so there is nothing to check there; the bug is entirely about
  `localhost` resolving to IPv6 by accident, not about needing IPv6 support.

## Decisions

### Check literal addresses, not the hostname `localhost`

`findFreePort` now checks two literal addresses per candidate port: `0.0.0.0` (what `otlp`/`health_check` bind) and
`127.0.0.1` (what the Prometheus telemetry reader binds via `localhost`). A port is free only when both checks
succeed. This is implemented as a small `canBindPort(host string, port int) bool` helper so the two checks share one
code path.

**Alternative considered: check only `0.0.0.0`.** Binding `0.0.0.0:<port>` fails if anything, wildcard or specific,
already holds that port on most platforms, which would catch the exact bug reproduced above. It was rejected because
it only catches conflicts on the address family `0.0.0.0` implies. The telemetry reader binds `127.0.0.1` specifically
(via `localhost`), and on a host where hostname resolution prefers IPv4 (common on Linux), an existing conflict on
`127.0.0.1` alone would go undetected by a `0.0.0.0`-only check, trading today's bug for a mirror-image one on a
different platform. Checking both addresses removes the dependency on hostname resolution order entirely, rather than
picking a different resolution order to depend on.

**Alternative considered: resolve `localhost` and iterate all returned addresses.** Would generalize better if a
receiver ever bound an IPv6 address, but nothing in this codebase does, and it adds indirection (a DNS/hosts-file
resolution step, plus handling however many addresses it returns) for a case that does not exist. Two literal,
known-correct addresses are simpler and just as correct for the receivers this project actually renders.

## Risks / Trade-offs

- **The check is strictly more conservative than before**: it can only reject a port the old check would have
  accepted (never the reverse), so a host where the old check already worked correctly sees no behavior change.
- **Two `Listen`/`Close` calls per candidate port instead of one.** Negligible; `findFreePort` is called a handful of
  times per install, not in a hot loop.
