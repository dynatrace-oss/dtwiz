# Tasks

## 1. Fix port freeness check

- [x] 1.1 In `pkg/installer/otel/collector.go`, add a `canBindPort(host string, port int) bool` helper that opens and
  immediately closes a TCP listener on `host:port`, returning whether it succeeded.
- [x] 1.2 Change `findFreePort` to consider a candidate port free only when `canBindPort("0.0.0.0", port)` and
  `canBindPort("localhost", port)` both succeed, replacing the previous single check against the hostname `localhost`
  alone. Probe the literal hostname `localhost`, not a hardcoded IP: which loopback address it resolves to varies by
  system, and the telemetry reader's own config binds `localhost` the same way (see `design.md`).
- [x] 1.3 Update the doc comment on `findFreePort` to explain why both addresses are checked, and why the second one
  is the literal hostname rather than a guessed IP (see `design.md`).

## 2. Tests

- [x] 2.1 In `pkg/installer/otel/collector_test.go`, replace the single `localhost`-based occupancy test with
  `TestFindFreePort_SkipsOccupiedWildcard` (occupies `0.0.0.0:8888`, the exact bug reproduced in `proposal.md`) and
  `TestFindFreePort_SkipsOccupiedLoopback` (occupies `localhost:8888`, matching the telemetry reader's own bind
  target, whichever loopback address that resolves to on the machine running the test).
- [x] 2.2 Update `TestFindFreePort_ReturnsFreePort` to verify the returned port is bindable on both `0.0.0.0` and
  `localhost`.
- [x] 2.3 Run `go build ./...` and `go test ./pkg/installer/otel/...` and confirm both pass.
- [x] 2.4 Add `TestOTelInstallAvoidsOccupiedPorts` to `test/e2e/otel_test.go` (build tag `integration`): occupy the
  collector's default ports with decoy listeners on `0.0.0.0:4317`, `0.0.0.0:4318`, `0.0.0.0:13133`, and
  `localhost:8888`, matching how `otel.tmpl` binds them, then call `otel.InstallOtelCollectorOnly` and assert it
  succeeds instead of the collector exiting immediately. End-to-end regression test for the unit-level coverage in
  task 2.1, exercising the real collector binary rather than only `findFreePort` in isolation. Not marked parallel,
  matching `TestOTelHostMonitoring`: both install on the collector's real default ports and must not race each other.

## 3. Fix verification targeting the wrong port

- [x] 3.1 Thread the collector's actual OTLP HTTP port through `install otel`'s post-install verification
  (`waitForOtelCollectorReady`, `sendOtelVerificationLog`, `verifyOtelInstall` in `pkg/installer/otel/collector.go`,
  and `collectorPlan.httpPort`) instead of assuming the default port: `generateOtelConfig` can allocate a different
  one whenever the default is occupied, which is exactly the scenario task 2.4 exercises.
- [x] 3.2 In `pkg/installer/otel/update.go`, add `extractOtlpHTTPPort` to read the OTLP HTTP port back out of the
  config being patched (via `net.SplitHostPort` on `receivers.otlp.protocols.http.endpoint`), falling back to the
  default port only when it can't be parsed. `update otel` patches a config it did not generate, so it has no
  allocated-port value to reuse the way `install otel` does.
- [x] 3.3 Bound `sendOtelVerificationLog`'s HTTP client to a fixed timeout (`otlpVerificationClient`, 10s) so it
  cannot hang forever against a port that accepts a TCP connection but never responds.
- [x] 3.4 Add unit tests in `pkg/installer/otel/collector_test.go`: `TestExtractOtlpHTTPPort` (table-driven over
  IPv4/IPv6/hostname endpoints, a missing `http` protocol block, an endpoint with no port, a non-numeric port, an
  empty document, and invalid YAML), `TestWaitForOtelCollectorReady_*` (confirms the function dials the `httpPort`
  actually passed in, not a hardcoded default), `TestSendOtelVerificationLog_*` (confirms the POST goes to the given
  `httpPort` via `httptest.Server`, and that non-2xx responses and exhausted retries both surface as errors), and
  `TestVerifyOtelInstall_UsesGivenHTTPPort` (end-to-end through `verifyOtelInstall` with `waitForLogInDynatraceFn`
  stubbed).
- [x] 3.5 Match connection-refused/reset errors in `sendOtelVerificationLog`'s retry logic using the actual
  platform-specific error codes instead of matching `err.Error()` text (the OS wording for "connection refused"
  differs on Windows, and Windows's `syscall.ECONNREFUSED` is a synthetic stdlib constant that never matches a real
  WinSock error). Split into `conn_error_unix.go` (`syscall.ECONNREFUSED`/`ECONNRESET`) and `conn_error_windows.go`
  (`golang.org/x/sys/windows.WSAECONNREFUSED`/`WSAECONNRESET`, the real WinSock codes), both exposing
  `isTransientDialError(err) bool`, matching this repo's build-tag convention for platform-specific logic (see
  `AGENTS.md`). Add `conn_error_unix_test.go` / `conn_error_windows_test.go`, each dialing a real closed port and
  asserting `isTransientDialError` recognizes the platform's actual connection-refused error, plus a negative case
  for an unrelated error.
- [x] 3.6 Verify with `go build`, `go vet`, and `go test ./pkg/installer/otel/...` under both the host `GOOS` and
  `GOOS=windows GOARCH=amd64` (a `go test -c` compile check; an actual Windows runtime run depends on CI).

## 4. Spec corrections

- [x] 4.1 Remove the misfiled "Generated config ports must not conflict with already-running collectors" requirement
  from `otel-collector-update` (`specs/otel-collector-update/spec.md`, `REMOVED Requirements` with reason and
  migration to `otel-collector-port-allocation`): `findFreePort` is only ever called via `prepareCollectorPlan`,
  which only `install otel` uses, never `update otel`'s patch flow that `otel-collector-update` actually covers.
- [x] 4.2 Add a requirement to `otel-collector-port-allocation` covering post-install verification targeting the
  collector's actual assigned port (task 3.1's behavior). No equivalent delta for `otel-collector-update`: task 3.2's
  fallback is defensive and doesn't change that capability's documented contract (same messages, same best-effort
  container caveat).

## 5. Verification and docs

- [x] 5.1 Run `make lint` and address any new findings.
- [x] 5.2 Update `CHANGELOG.md` `[Unreleased]` with `Fixed` entries: `install otel` no longer selects a port that is
  already occupied on `0.0.0.0` when a listener already on that port is bound to a specific loopback address instead
  (`localhost`, whichever of `127.0.0.1` or `::1` that resolves to), which previously caused the newly started
  collector to exit immediately with `exit status 1`; and `install otel`/`update otel` post-install verification now
  targets the collector's actual OTLP HTTP port instead of always assuming the default.
- [x] 5.3 Manually re-verify against the originally reproduced scenario: with another OTel Collector already bound to
  `0.0.0.0:4317`/`4318`/`8888`/`13133`, run `install otel` and confirm it allocates ports above those defaults and the
  new collector starts successfully instead of exiting immediately. Now also covered automatically by task 2.4's
  integration test.
