# Tasks

## 1. Implementation

- [x] 1.1 In `cmd/selfmonitoring.go`, replace `params.Cmd = ""` with `params.Cmd = "watch"` and add `params.Sub = ""` in `buildWatchEventCallback`

## 2. Spec Update

- [x] 2.1 Update `openspec/specs/selfmonitoring-watch/spec.md` to reflect the new behavior: `c=wch` always present in watch completion events, updated worst-case header length (58 chars), and removal of the "st=com omits c=" requirement

## 3. Tests

- [x] 3.1 Add a test for `buildWatchEventCallback` in `cmd/root_test.go` (or `cmd/selfmonitoring_test.go`) verifying that the emitted event always has `Cmd == "watch"` and `Sub == ""`
