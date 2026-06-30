# Tasks: Remove Parent Directory Scanning

## 1. Implementation

- [x] 1.1 In `pkg/installer/otel_runtime_scan.go`, change `walkCandidateDirs(workingDir, 2, ...)` to `walkCandidateDirs(workingDir, 0, ...)` in `scanProjectDirs`
- [x] 1.2 In `pkg/installer/otel_uninstall.go`, change `walkCandidateDirs(cwd, 2, ...)` to `walkCandidateDirs(cwd, 0, ...)` in `findNodeOtelDirs`; update docstring to reflect restricted scope

## 2. Tests

- [x] 2.1 Add unit test: `TestScanProjectDirs_ParentNotScanned` — scanning from a subdirectory does NOT detect a project marker in the parent directory
- [x] 2.2 Add unit test: scanning from a directory detects a project marker in the directory itself
- [x] 2.3 Add unit test: scanning from a directory detects a project marker in a subdirectory
- [x] 2.4 Add unit test: `TestFindNodeOtelDirs_ParentNotScanned` — uninstall scan does NOT find `.otel/` dirs in parent directories
- [x] 2.5 Tighten `TestPromptProjectSelection_SingleProjectRangeHint` assertion: check for `"instrument [1] or press"` instead of `"[1]"` to distinguish the range hint from the project listing line
- [x] 2.6 Add `len(projects) == 0` assertion to `TestScanProjectDirs_ParentNotScanned` to explicitly verify no projects are returned from a cwd with no markers

## 3. Verification

- [x] 3.1 Run `make test` — all tests pass
- [x] 3.2 Run `make lint` — no new issues
