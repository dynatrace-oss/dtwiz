# Tasks

## 1. Extend `pkg/selfmonitoring` with new fields and constants

- [x] 1.1 Add `StepRecommendationsPresented = "rpr"` and `StepRecommendationsSelected = "rsl"` constants to `pkg/selfmonitoring/selfmonitoring.go`
- [x] 1.2 Add entries to `stepFullNames`: `"rpr"` → `"recommendations_presented"`, `"rsl"` → `"recommendations_selected"`
- [x] 1.3 Add `propOpt = "opt"` and `propTech = "tx"` constants to `pkg/selfmonitoring/selfmonitoring.go`
- [x] 1.4 Add `Opt string` and `Tech string` fields to the `EventParams` struct
- [x] 1.5 Add `techShortMap` mapping the 8 runtime names from `detect_project.go` (`Node.js`→`nd`, `Go`→`go`, `Python`→`py`, `Java`→`jv`, `Rust`→`rs`, `Ruby`→`rb`, `PHP`→`ph`, `.NET`→`dn`) and a `shortTech` helper
- [x] 1.6 Update `buildUserAgent` to include `;opt=<shortOpt>` when `Opt` is non-empty (using the existing `subShortMap` for abbreviation) and `;tx=<shortTech>` when `Tech` is non-empty
- [x] 1.7 Update `buildEventProps` to include `"option"` (full name) when `Opt` is non-empty and `"technology"` (full name) when `Tech` is non-empty

## 2. Update `cmd/selfmonitoring.go`

- [x] 2.1 Update `fireSetupRecommendEvent` to set `p.Opt = sub` (instead of `p.Sub`) and `p.StepID = selfmonitoring.StepRecommendationsSelected`
- [x] 2.2 Add `fireSetupMenuEvent(cmd *cobra.Command, recs []recommender.Recommendation, techNames []string)` that iterates `recs` and fires: for `MethodOtelCollector` with non-empty `techNames`, one event per tech (each with `Opt=otel-collector`, `Tech=tech`, `StepID=StepRecommendationsPresented`); for all other methods, one event with `Opt=<method>`, no `Tech`, `StepID=StepRecommendationsPresented`

## 3. Wire into `cmd/setup.go`

- [x] 3.1 Extract tech names from `info.ProjectTechs` into a `[]string` after `analyzeSystem()` returns
- [x] 3.2 Call `fireSetupMenuEvent(cmd, actionable, techNames)` immediately after `fmt.Print(recommender.FormatSetupMenu(...))` and before `reader.ReadString('\n')`

## 4. Tests

- [x] 4.1 In `pkg/selfmonitoring/selfmonitoring_test.go`, add table-driven tests for `buildUserAgent` covering: `Opt` set (verifies `opt=` present, `s=` absent), `Tech` set (verifies `tx=`), `Opt` and `Tech` both set, `StepRecommendationsPresented` and `StepRecommendationsSelected` as `st=` values
- [x] 4.2 In `pkg/selfmonitoring/selfmonitoring_test.go`, add tests for `buildEventProps` covering: `Opt` produces `"option"` key in body, `Tech` produces `"technology"` key in body with full name
- [x] 4.3 In `cmd/selfmonitoring_test.go`, verify `fireSetupMenuEvent` fires the correct number of events for: no tech (one per method), OTel with two techs (two events for OTel), mixed methods with techs (correct per-method counts)
- [x] 4.4 Verify `make test` and `make lint` pass with no new issues
