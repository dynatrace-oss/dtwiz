# Tasks

## 1. Implementation

- [x] 1.1 Add `watchTimeout = 10 * time.Minute` constant alongside `watchPollInterval` in `pkg/installer/ingest_watch.go`
- [x] 1.2 Add `watchPhase` type and `watchPhaseProbe` / `watchPhaseMetrics` constants
- [x] 1.3 Add `watchQueryState` struct with `logs watchPhase` and `requests watchPhase` fields
- [x] 1.4 Add `watchInput` struct with `line string` and `err error` fields; replace byte-by-byte stdin goroutine with `bufio.NewReader.ReadString('\n')` writing to `inputCh chan watchInput`
- [x] 1.5 Add `Status string` field to `watchSection`; update `renderSection` to show `Status` when `Count == 0` and `Status != ""`
- [x] 1.6 Initialize `qs := watchQueryState{}` in `watchIngest`; pass `&qs` to `pollAll`
- [x] 1.7 Implement timeout block in main select loop: check `elapsed >= watchTimeout`, print prompt inline, read from `inputCh`, increment `prevLines`, call `ticker.Reset`, drain `ticker.C` and `inputCh`
- [x] 1.8 Update `pollAll` signature to accept `qs *watchQueryState`; implement two-phase logic for Logs (probe → summarize) and Requests (probe → summarize) with phase transition and `Status` assignment

## 2. Spec

- [x] 2.1 Update `openspec/specs/ingest-watch/spec.md`: add CHANGED requirements for timeout, two-phase Logs, two-phase Requests
- [x] 2.2 Create `openspec/changes/watch-query-optimization/specs/ingest-watch/spec.md` with the change-scoped delta
