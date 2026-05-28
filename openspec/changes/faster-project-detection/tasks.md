## 1. Walker — parallel traversal

- [x] 1.1 Change `walkCandidateDirs` in `pkg/installer/otel_runtime_scan.go` to scan with a bounded worker pool sized at `max(NumCPU*scanConcurrencyPerCPU, minScanConcurrency)`, falling back to synchronous execution when the semaphore is full
- [x] 1.2 Introduce `queued sync.Map` to dedupe directories across all `scanTree` calls (handles ancestor revisits and symlinks)
- [x] 1.3 Remove the `maxScanDepth = 15` cap; allow unbounded recursion (bounded in practice by the skip list and `queued`)
- [x] 1.4 Hoist `os.ReadDir(dir)` to run once per directory; pass the result to the `visit` callback so child recursion and marker matching share one syscall
- [x] 1.5 Update `visit` signature from `func(dir string) bool` to `func(dir string, entries []os.DirEntry) bool`; update the uninstall caller in `pkg/installer/otel_uninstall.go` (`findNodeOtelDirs`)

## 2. Project scanner — marker matching + symlink dedup

- [x] 2.1 In `scanProjectDirs`, build `markerSet map[string]struct{}` once and match by iterating entries (replacing N `os.Stat` calls per directory)
- [x] 2.2 Defer `filepath.EvalSymlinks` so it only runs when markers actually match (was: every visited directory)
- [x] 2.3 Replace the `visitedDirs map[string]bool` + mutex with `matchedProjects sync.Map` keyed by lowercased resolved path; dedup happens at record-time via `LoadOrStore`
- [x] 2.4 Add `$`-prefix skip rule to `shouldSkipDir` so it stays consistent with `isIgnoredDir`
- [x] 2.5 Raise `largeScanThreshold` to 10000 and move the follow-up "tip" line to `2*largeScanThreshold`

## 3. Ignored directories — expand for Windows

- [x] 3.1 Add Windows system directories to `ignoredProjectDirNames`: `Windows`, `System32`, `SysWOW64`, `WinSxS`, `ProgramData`, `AppData`, `$Recycle.Bin`
- [x] 3.2 Add `$`-prefix check to `isIgnoredDir` so any `$`-prefixed name is skipped without explicit listing

## 4. Tests

- [x] 4.1 Add `TestScanProjectDirs_WideParallelTree` — verifies parallel scan correctness over a wide tree
- [x] 4.2 Add `TestScanProjectDirs_WindowsSystemDirSkipped` — verifies new Windows-dir exclusions
- [x] 4.3 Add `TestScanProjectDirs_DevDirFound` — guards against false positives in the expanded ignore list (`dev` is not a system dir)
- [x] 4.4 Remove `TestScanProjectDirs_DepthLimit` (the depth limit it tested has been removed)
- [x] 4.5 Verify all existing scan tests still pass: `go test ./pkg/installer/...`

## 5. Docs / changelog

- [x] 5.1 Add an `Unreleased` entry to `CHANGELOG.md` noting the project-detection improvements
- [x] 5.2 Confirm no live openspec specs need updating (scan internals are not part of any behavioral contract)
