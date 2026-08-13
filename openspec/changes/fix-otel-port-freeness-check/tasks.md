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

## 3. Review follow-up

- [x] 3.1 Code review caught that the first version of this fix (task 1.2) checked `0.0.0.0` and a hardcoded
  `127.0.0.1`, and that a wildcard bind on `0.0.0.0` does not conflict with an existing bind on a specific loopback
  address (confirmed empirically), so the hardcoded check would miss an existing `localhost`-bound listener on a
  system where `localhost` resolves to `::1` instead. Fixed by probing the literal hostname `localhost` instead of a
  hardcoded IP, so the check always matches whatever the real bind target is. See `design.md`'s "Alternative
  considered and rejected" entries for both rejected versions.
- [x] 3.2 Re-ran `go build ./...` and `go test ./pkg/installer/otel/...` after the correction; both pass.
- [x] 3.3 Code review caught that `proposal.md`'s "Modified Capabilities: (none)" was wrong: an existing requirement
  in `openspec/specs/otel-collector-update/spec.md` ("Generated config ports must not conflict with already-running
  collectors") already covered this behavior and would have been left stale. Confirmed `findFreePort` is only ever
  called via `prepareCollectorPlan`, which only `install otel` uses, never `update otel`'s patch flow that
  `otel-collector-update` actually covers, so the requirement was filed under the wrong capability. Removed it from
  `otel-collector-update` (`specs/otel-collector-update/spec.md`, REMOVED Requirements with reason and migration) and
  updated `proposal.md`'s Modified Capabilities accordingly, rather than leaving it stale or editing it in place under
  the wrong capability. See `design.md`.
- [x] 3.4 Writing task 2.4's regression test (which occupies the collector's default OTLP HTTP port with a decoy)
  surfaced a related bug: `waitForOtelCollectorReady`, `sendOtelVerificationLog`, and `verifyOtelInstall` in
  `pkg/installer/otel/collector.go` hardcoded the default OTLP HTTP port instead of using the port
  `generateOtelConfig` actually allocated, so verification would probe/post to the wrong port whenever the default
  was occupied. Fixed by threading the allocated port (`collectorPlan.httpPort`) through `install otel`'s
  verification path. `update otel` (`pkg/installer/otel/update.go`) patches a config it did not generate, so it has
  no allocated port to thread through; added `extractOtlpHTTPPort` to read the port back out of the patched config,
  falling back to the default only when it cannot be parsed. Updated `proposal.md` (What Changes, Modified
  Capabilities, Impact) and both spec deltas (`otel-collector-port-allocation` new requirement,
  `otel-collector-update` MODIFIED requirement) to reflect this.
- [x] 3.5 Bounded the verification HTTP client (`otlpVerificationClient`, 10s timeout) so
  `sendOtelVerificationLog` cannot hang forever if `httpPort` belongs to a process that accepts the TCP connection
  but never responds, which is possible now that the port comes from config rather than being hardcoded to a
  Dynatrace collector's own default.
- [x] 3.6 Task 3.4/3.5's fix had no direct unit coverage. Simplified `extractOtlpHTTPPort` to use `net.SplitHostPort`
  instead of manual `strings.LastIndex` slicing (correctly handles bracketed IPv6 endpoints; stdlib-endorsed for this
  exact parse), then added unit tests in `pkg/installer/otel/collector_test.go`:
  - `TestExtractOtlpHTTPPort`: table-driven over IPv4/IPv6/hostname endpoints, a missing `http` protocol block, an
    endpoint with no port, a non-numeric port, an empty document, and invalid YAML.
  - `TestWaitForOtelCollectorReady_ReturnsImmediatelyWhenPortIsOpen` /
    `_TimesOutOnActualPortProbed` / `_AbortsImmediatelyOnCrash`: confirm the function dials the `httpPort` actually
    passed in (not a hardcoded default), including that the timeout error names that same port.
  - `TestSendOtelVerificationLog_PostsToGivenPort` / `_NonSuccessStatus` / `_GivesUpAfterRetriesOnRefusedPort`: confirm
    the POST goes to the given `httpPort` via `httptest.Server`, and that non-2xx responses and exhausted retries both
    surface as errors.
  - `TestVerifyOtelInstall_UsesGivenHTTPPort`: end-to-end through `verifyOtelInstall` with `waitForLogInDynatraceFn`
    stubbed, confirming the readiness check and verification log both reach an `httptest.Server` bound to the given
    `httpPort` rather than a port either helper assumes on its own.
  Verified with `go build ./...`, `go vet ./pkg/installer/otel/...`, and `go test ./pkg/installer/otel/...` (all pass).
- [x] 3.7 `TestSendOtelVerificationLog_GivesUpAfterRetriesOnRefusedPort` (task 3.6) failed on the Windows CI runner:
  `sendOtelVerificationLog`'s retry-on-transient-error check matched the connection-refused/reset error text by
  substring (`"connection reset"` / `"connection refused"`), which is Unix-specific wording. Windows reports the same
  condition as `"No connection could be made because the target machine actively refused it."`, so the retry never
  fired and the request failed on the first attempt instead of after exhausting retries. Fixed by matching the
  underlying syscall errno via `errors.Is(err, syscall.ECONNREFUSED)` / `errors.Is(err, syscall.ECONNRESET)` instead
  of `err.Error()` text; `syscall.ECONNREFUSED`/`ECONNRESET` are defined on all platforms `dtwiz` targets, including
  Windows ("invented values" the Windows `syscall` package maps WSA errors to expressly so cross-platform code can
  use `errors.Is` against them), so this now correctly retries on all of them. Verified with `go build`/`go vet`
  under both the host `GOOS` and `GOOS=windows`, and `go test ./pkg/installer/otel/...`.

## 4. Verification and docs

- [x] 4.1 Run `make lint` and address any new findings.
- [x] 4.2 Update `CHANGELOG.md` `[Unreleased]` with a `Fixed` entry: `install otel` no longer selects a port that is
  already occupied on `0.0.0.0` when a listener already on that port is bound to a specific loopback address instead
  (`localhost`, whichever of `127.0.0.1` or `::1` that resolves to), which previously caused the newly started
  collector to exit immediately with `exit status 1`.
- [x] 4.3 Manually re-verify against the originally reproduced scenario: with another OTel Collector already bound to
  `0.0.0.0:4317`/`4318`/`8888`/`13133`, run `install otel` and confirm it allocates ports above those defaults and the
  new collector starts successfully instead of exiting immediately. Now also covered automatically by task 2.4's
  integration test.
