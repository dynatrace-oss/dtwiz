# Tasks

## 1. Extend `pkg/selfmonitoring` with new fields and constants

- [x] 1.1 Add `StepRecommendationsPresented = "rpr"` and `StepRecommendationsSelected = "rsl"` constants to `pkg/selfmonitoring/selfmonitoring.go`
- [x] 1.2 Add entries to `stepFullNames`: `"rpr"` → `"recommendations_presented"`, `"rsl"` → `"recommendations_selected"`
- [x] 1.3 Add a `propOpt = "opt"` constant to `pkg/selfmonitoring/selfmonitoring.go`
- [x] 1.4 Add an `Opt string` field to the `EventParams` struct
- [x] 1.5 Update `buildUserAgent` to include `;opt=<shortOpt>` when `Opt` is non-empty, reusing the existing `subShortMap` for abbreviation
- [x] 1.6 Update `buildEventProps` to include `"option"` (full name) when `Opt` is non-empty

## 2. Update `cmd/selfmonitoring.go`

- [x] 2.1 Update `fireSetupRecommendEvent` to set `p.Opt = method` (instead of `p.Sub`) and use `selfmonitoring.StepRecommendationsSelected` as the step
- [x] 2.2 Add `fireSetupMenuEvent(cmd *cobra.Command, recs []recommender.Recommendation)` that fires one event per recommendation with `Opt=<method>` and `StepID=StepRecommendationsPresented`

## 3. Wire into `cmd/setup.go`

- [x] 3.1 Call `fireSetupMenuEvent(cmd, actionable)` immediately after `fmt.Print(recommender.FormatSetupMenu(...))` and before `reader.ReadString('\n')`

## 4. Tests

- [x] 4.1 In `pkg/selfmonitoring/selfmonitoring_test.go`, add table-driven `buildUserAgent` tests covering: `Opt` set (verifies `opt=` present, `s=` absent), both new steps appearing as `st=`, and the 64-char limit
- [x] 4.2 In `pkg/selfmonitoring/selfmonitoring_test.go`, add a `buildEventProps` test verifying `Opt` produces the `"option"` body key
- [x] 4.3 In `pkg/selfmonitoring/selfmonitoring_test.go`, extend `TestStepFullNames` to cover the two new step constants
- [x] 4.4 In `cmd/selfmonitoring_test.go`, verify `fireSetupMenuEvent` fires one event per presented method with the correct step, and fires nothing for an empty recommendation list
- [x] 4.5 Verify `make test` and `make lint` pass with no new issues

## 5. Changelog

- [x] 5.1 Add an `[Unreleased]` entry noting that `dtwiz setup` now reports the presented recommendation menu and the selected option, since self-monitoring is on by default and this changes what every user sends
