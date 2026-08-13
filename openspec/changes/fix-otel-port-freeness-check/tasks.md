# Tasks

## 1. Fix port freeness check

- [x] 1.1 In `pkg/installer/otel/collector.go`, add a `canBindPort(host string, port int) bool` helper that opens and
  immediately closes a TCP listener on `host:port`, returning whether it succeeded.
- [x] 1.2 Change `findFreePort` to consider a candidate port free only when `canBindPort("0.0.0.0", port)` and
  `canBindPort("127.0.0.1", port)` both succeed, replacing the previous single check against the hostname `localhost`.
- [x] 1.3 Update the doc comment on `findFreePort` to explain why two literal addresses are checked instead of the
  hostname (see `design.md`).

## 2. Tests

- [x] 2.1 In `pkg/installer/otel/collector_test.go`, replace the single `localhost`-based occupancy test with
  `TestFindFreePort_SkipsOccupiedWildcard` (occupies `0.0.0.0:8888`, the exact bug reproduced in `proposal.md`) and
  `TestFindFreePort_SkipsOccupiedLoopback` (occupies `127.0.0.1:8888`, the telemetry-reader case).
- [x] 2.2 Update `TestFindFreePort_ReturnsFreePort` to verify the returned port is bindable on both `0.0.0.0` and
  `127.0.0.1`.
- [x] 2.3 Run `go build ./...` and `go test ./pkg/installer/otel/...` and confirm both pass.

## 3. Verification and docs

- [x] 3.1 Run `make lint` and address any new findings.
- [x] 3.2 Update `CHANGELOG.md` `[Unreleased]` with a `Fixed` entry: `install otel` no longer selects a port that is
  already occupied on `0.0.0.0` when the hostname `localhost` resolves to IPv6 ahead of IPv4 (observed on macOS), which
  previously caused the newly started collector to exit immediately with `exit status 1`.
- [x] 3.3 Manually re-verify against the originally reproduced scenario: with another OTel Collector already bound to
  `0.0.0.0:4317`/`4318`/`8888`/`13133`, run `install otel` and confirm it allocates ports above those defaults and the
  new collector starts successfully instead of exiting immediately.
