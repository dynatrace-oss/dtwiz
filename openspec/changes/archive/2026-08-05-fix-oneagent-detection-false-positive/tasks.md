# Implementation Tasks

## Fix

- [x] 1.1 Hoist the systemd marker path into a package-level `systemdRunDir`
  variable in `pkg/analyzer/detect_oneagent.go` (default
  `/run/systemd/system`).
- [x] 1.2 Add `systemdAvailable()` returning true only when `systemdRunDir`
  exists and is a directory.
- [x] 1.3 Gate the `systemctl is-active --quiet oneagent` check in
  `detectOneAgent()` behind `systemdAvailable()`; keep the
  `oneagentctl --version` fallback unconditional.

## Tests

- [x] 2.1 Add `pkg/analyzer/detect_oneagent_test.go` (build tag `!windows`)
  with `TestSystemdAvailable`: directory exists → true; directory missing →
  false; path is a regular file → false.
- [x] 2.2 Add `TestDetectOneAgentIgnoresSystemctlShimWithoutSystemd`: PATH
  contains only a `systemctl` script that exits 0 (shim reproduction); with
  `systemdRunDir` absent detection returns false, with it present detection
  returns true.
- [x] 2.3 Add `TestDetectOneAgentFallsBackToOneagentctl`: no systemd; detection
  returns true when `oneagentctl` is in PATH and false when it is not.
- [x] 2.4 `go build ./...`, `go vet ./pkg/analyzer/`, and
  `go test ./pkg/analyzer/` pass.
