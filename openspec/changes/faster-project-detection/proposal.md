## Why

Project detection (`scanProjectDirs` in `pkg/installer/otel_runtime_scan.go`) was slow on large directory trees and missed deeply nested projects. The scan ran sequentially, paid `os.Stat` for every marker on every directory, resolved symlinks even when no project was found, and capped recursion at 15 levels — combining to make `dtwiz analyze` / `dtwiz setup` noticeably slow when run from a directory above many subprojects, while still missing some.

## What Changes

- Parallel directory walk in `walkCandidateDirs` using a bounded worker pool (`max(NumCPU*2, 4)` goroutines). When the pool is full, child scans run synchronously in the caller's goroutine — keeps goroutine count strictly bounded.
- `os.ReadDir` once per directory; entries are reused for both marker matching and child recursion. Marker matching is now a hash lookup against the entry names instead of N `os.Stat` calls per directory.
- `filepath.EvalSymlinks` deferred to match-time — only called when a directory actually contains marker files (was: every visited directory).
- Symlink dedup via `sync.Map` keyed by lowercased resolved path, evaluated only on a match — same physical directory reached via two paths is recorded once.
- Depth limit removed from `walkCandidateDirs` (was capped at 15).
- Expanded ignored-directory list: Windows system dirs (`Windows`, `System32`, `SysWOW64`, `WinSxS`, `ProgramData`, `AppData`, `$Recycle.Bin`) and any `$`-prefixed directory.

## Capabilities

### New Capabilities

None. The scanner internals are implementation detail, not a behavioral contract worth promoting to a capability spec.

### Modified Capabilities

- `nodejs-project-detection`: add scenarios for the user-visible behavior changes — deeply nested projects are now found (depth cap removed), and the scanner skips Windows system directories. The capability description, marker list, and existing scenarios are unchanged.

## Impact

- **Code**: `pkg/installer/otel_runtime_scan.go` (rewrite of `walkCandidateDirs` + `scanProjectDirs.dirMatches`), `pkg/installer/otel_uninstall.go` (visit callback signature update), `pkg/installer/otel_runtime_scan_test.go` (parallelism test added, removed depth-limit test).
- **APIs / external contract**: none — the public callers (`detectNodeProjects`, `detectPythonProjects`, `detectJavaProjects`, `detectGoProjects`, `findNodeOtelDirs`) keep identical signatures and return the same `[]ScannedProject` / `[]string` shapes.
- **Dependencies**: none added.
- **Risk**: low. Symlink dedup semantics narrowed slightly (only on match, not on every visit) but the walker's own `queued` map still prevents raw-path revisits, and the test suite covers symlink, monorepo, deep-nesting, and parallel-tree scenarios.
