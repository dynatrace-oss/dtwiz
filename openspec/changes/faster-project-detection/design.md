## Context

The scanner in `pkg/installer/otel_runtime_scan.go` is invoked once per runtime (Node/Python/Java/Go) by `dtwiz analyze`, `dtwiz setup`, `dtwiz recommend`, and `dtwiz install otel`. Each call recursively walks the CWD subtree plus two ancestor levels looking for marker files (`package.json`, `pyproject.toml`, etc.). Before this change the walk was sequential, paid `os.Stat` for every marker on every visited directory, called `filepath.EvalSymlinks` on every visited directory for dedup, and stopped at depth 15.

## Goals / Non-Goals

**Goals:**
- Reduce duration of project detection on large home directories and monorepos.
- Avoid missing deeply nested projects.
- Keep behavior cross-platform and avoid scanning Windows system directories that produced no useful matches.

**Non-Goals:**
- Changing the public API of `scanProjectDirs` or any caller signatures.
- Sharing one tree walk across multiple runtimes. Each runtime (Node, Python, Java, Go) still issues its own walk; in practice these run concurrently via `detectAllProjects` and walks after the first benefit from the OS cache populated by the earlier ones, so the overhead is much less than 4×. Merging them into a single walk that checks all marker sets per directory is a potential follow-up.
- Recommender behavior (separate PR).

## Decisions

- **Bounded worker pool over `errgroup`/`WaitGroup`-only.** A buffered channel used as a counting semaphore (`sem`) caps concurrent goroutines at `max(NumCPU*2, 4)`. When the semaphore is full, the parent's `select` falls through `default` and runs the child scan synchronously in the caller's goroutine — keeps goroutine count strictly bounded regardless of fan-out. Alternatives considered: unbounded `go func` (rejected — goroutine explosion on deep trees), `errgroup.SetLimit` (rejected — blocks rather than degrades gracefully).
- **`ReadDir` hoisted into the walker; entries passed to `visit`.** The walker already had to `ReadDir` for child recursion; passing the result to `visit` lets the project-detection callback do marker matching as a `map[string]struct{}` lookup against entry names — one syscall per dir instead of N stats + 1 ReadDir.
- **`EvalSymlinks` deferred to match-time.** Matches are rare (a handful per 10k-dir scan), so paying the resolve cost only on match collapses thousands of syscalls into a handful. Dedup happens via `sync.Map.LoadOrStore` on the resolved path; loser of the race returns `true` (skip children) without re-recording.
- **Depth limit removed rather than raised.** Raising it to 30 or 50 would still risk false negatives. The walker already skips heavy noise dirs (`node_modules`, build outputs, system dirs) and dedups via the `queued` map, so an unlimited walk over a pruned tree is bounded in practice.
- **`$`-prefix and Windows-system-dir skips in both predicates.** `isIgnoredDir` (used by uninstall) and the local `shouldSkipDir` inside `scanProjectDirs` were drifting; both now apply the `$` prefix rule plus the expanded ignored-names map.

## Risks / Trade-offs

- [Symlink dedup is now best-effort, not pre-emptive] → Before, two paths to the same physical dir would both early-return after dedup; now both will scan their entries (cheap, in-memory) and race on the resolved-path map. Wasted work is small and the walker's `queued` map still prevents identical raw-path revisits. Mitigation: covered by `TestScanProjectDirs_SymlinkDedup` in the existing test suite.
- [Removed depth limit could in theory walk into pathological symlink loops] → `filepath.EvalSymlinks` is only called on matches, so it does not break symlink cycles on the walk itself. However, the walker's `queued` map keys on raw paths, and the OS-level ignored-dir list excludes the directories where symlink loops typically occur (`.git/`, system dirs). No bug observed in CI or manual testing.
- [Parallel scan increases syscall pressure briefly] → Mitigated by the semaphore cap and the synchronous fallback on saturation.
