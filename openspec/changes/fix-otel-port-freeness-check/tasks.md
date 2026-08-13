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

## 4. Verification and docs

- [x] 4.1 Run `make lint` and address any new findings.
- [x] 4.2 Update `CHANGELOG.md` `[Unreleased]` with a `Fixed` entry: `install otel` no longer selects a port that is
  already occupied on `0.0.0.0` when a listener already on that port is bound to a specific loopback address instead
  (`localhost`, whichever of `127.0.0.1` or `::1` that resolves to), which previously caused the newly started
  collector to exit immediately with `exit status 1`.
- [x] 4.3 Manually re-verify against the originally reproduced scenario: with another OTel Collector already bound to
  `0.0.0.0:4317`/`4318`/`8888`/`13133`, run `install otel` and confirm it allocates ports above those defaults and the
  new collector starts successfully instead of exiting immediately.
