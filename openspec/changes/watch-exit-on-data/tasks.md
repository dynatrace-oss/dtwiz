# Tasks

## 1. Watch loop: exit-on-data mode

- [ ] 1.1 In `pkg/installer/ingest_watch.go`, turn `watchPollInterval` and `watchTimeout` from constants into package variables, and verify `go test ./pkg/installer/` still passes unchanged
- [ ] 1.2 Add an `exitOnData bool` parameter to the private `watchIngest()` and pass `false` from every existing exported wrapper; verify with `go build ./...` and by checking that no existing call site in `cmd/` or `pkg/installer/*/` changed
- [ ] 1.3 Add the exported standalone entry point (design decision 1) that passes `exitOnData` through, with a doc comment saying it is for `dtwiz watch` only, and verify `go build ./...`
- [ ] 1.4 After rendering a frame in exit-on-data mode, return with `ExitReason = "first_data"` once `result.FirstDataMs` is non-empty, calling `onSnapshot` synchronously on that path (design decisions 3 and 6); verify with the tests in 1.7
- [ ] 1.5 Leave out the "Press Enter to continue or keep watching..." line on the frame that ends the watch (design decision 4), and verify with the test in 1.7 that checks the captured output
- [ ] 1.6 At the 10-minute check in exit-on-data mode, return with `ExitReason = "timeout"` without printing the prompt or reading input, on TTY and non-TTY (design decision 5); verify with the test in 1.7
- [ ] 1.7 Add loop tests to `pkg/installer/ingest_watch_test.go` against an `httptest` DQL server, with the interval and timeout shortened, following Arrange / Act / Assert from `docs/contributing/testing.md`, and verify `go test -race ./pkg/installer/` passes:
  - `TestWatchIngest_ExitOnData_ExitsOnFirstPollWithData`: the server returns host data on the first query; the call returns with `ExitReason == "first_data"`, `FirstDataMs` holds `hosts`, and the output has no "Press Enter" line on the final frame
  - `TestWatchIngest_ExitOnData_ExitsWhenDataArrivesOnLaterPoll`: empty results first, data on the second poll; it returns after the second poll with `"first_data"`
  - `TestWatchIngest_ExitOnData_SnapshotSentBeforeReturn`: the `onSnapshot` callback has been called exactly once by the time the call returns, with the signals the poll found, and `onComplete` is called after it
  - `TestWatchIngest_ExitOnData_TimeoutWithoutData`: the server never returns data; it returns with `ExitReason == "timeout"`, the output has no "Continue watching?" line, and `onSnapshot` is never called
  - `TestWatchIngest_NoExitOnData_KeepsPollingAfterData`: with `exitOnData` false and data on the first poll, the loop is still polling after several more intervals and ends only at the shortened timeout, so post-install watches are unaffected

## 2. `dtwiz watch` flag

- [ ] 2.1 In `cmd/watch.go`, register `--exit-on-data` (bool, default false, help text: "stop after the first data arrives, or at the 10-minute timeout without prompting") and call the new standalone entry point with it; verify `dtwiz watch --help` lists the flag
- [ ] 2.2 Add `cmd/watch_test.go` with `TestWatchCmd_ExitOnDataFlagRegistered` (the flag exists, is a bool and defaults to false) and verify `go test ./cmd/` passes
- [ ] 2.3 Check that `--exit-on-data` is not added to `install`, `update`, `uninstall` or `setup`, and that their `--yes` handling is untouched: `git diff --stat` shows no changes to `cmd/install.go`, `cmd/update.go`, `cmd/uninstall.go` or `cmd/setup.go`
- [ ] 2.4 Document the flag: the `dtwiz watch` line in `AGENTS.md`, the command table in `README.md`, the command table in `skills/dtwiz/SKILL.md`, and an `Added` entry under `[Unreleased]` in `CHANGELOG.md`; verify each mentions `--exit-on-data` with the same meaning

## 3. Integration check

- [ ] 3.1 Run `make test` and `make lint` and verify both pass with no new lint findings
- [ ] 3.2 Against a tenant with recent data, run `dtwiz watch --exit-on-data --from "now()-1h"` and verify it renders one frame without the "Press Enter" line and exits with code `0` (`echo $?`); then run `dtwiz watch --exit-on-data | cat` and verify the non-TTY output ends the same way
