## 1. Implementation

- [x] 1.1 In `pkg/installer/otel_runtime_scan.go:247`, change `walkCandidateDirs(workingDir, 2, ...)` to `walkCandidateDirs(workingDir, 0, ...)`

## 2. Tests

- [x] 2.1 Add unit test: scanning from a subdirectory does NOT detect a project marker in the parent directory
- [x] 2.2 Add unit test: scanning from a directory detects a project marker in the directory itself
- [x] 2.3 Add unit test: scanning from a directory detects a project marker in a subdirectory

## 3. Verification

- [x] 3.1 Run `make test` — all tests pass
- [x] 3.2 Run `make lint` — no new issues
