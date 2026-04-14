# Tasks

## 1. Core featureflags package

Create the `pkg/featureflags` package with the registry, `IsEnabled`, `List`, and `SetForTest`.

**Files:** `pkg/featureflags/featureflags.go` (create), `pkg/featureflags/featureflags_test.go` (create)

- [ ] 1.1 Create `pkg/featureflags/featureflags.go` with `Flag` type (int const), `CLIFeatureFlag` struct (flag, name, envVar, defaultVal, desc), and `registry` slice with the `AllRuntimes` entry (`"all-runtimes"`, `"DTWIZ_ALL_RUNTIMES"`, `false`)
- [ ] 1.2 Implement `IsEnabled(flag Flag) bool` — resolution order: test override → CLI override → env var (`"true"`/`"1"`) → default
- [ ] 1.3 Implement `List() []FlagState` — iterate registry, resolve each flag, return name, env var, enabled state, and source
- [ ] 1.4 Implement `SetForTest(t testing.TB, flag Flag, val bool)` — store override in mutex-protected `testOverrides` map, restore previous value via `t.Cleanup`
- [ ] 1.5 Add unit tests: default returns false, env var `"true"` returns true, env var `"1"` returns true, env var `"false"` returns false, unknown flag returns false, `SetForTest` overrides and restores, `List` returns correct source for each override type

## 2. Cobra integration

Wire feature flags into the cobra command tree as persistent boolean flags on `rootCmd`.

**Files:** `pkg/featureflags/featureflags.go` (modify), `cmd/root.go` (modify)

- [ ] 2.1 Add `RegisterFlags(flags *pflag.FlagSet)` — iterate registry, call `flags.BoolVar` for each entry with a package-level bound variable on the `CLIFeatureFlag` struct
- [ ] 2.2 Add `ApplyCLIOverrides(flags *pflag.FlagSet)` — iterate registry, check `flags.Changed(name)`, store value in `cliOverrides` map if changed
- [ ] 2.3 In `cmd/root.go` `init()`, call `featureflags.RegisterFlags(rootCmd.PersistentFlags())` after existing flag registrations
- [ ] 2.4 In `rootCmd.PersistentPreRun`, call `featureflags.ApplyCLIOverrides(cmd.Flags())` after `logger.Init`
- [ ] 2.5 Add tests: CLI override takes precedence over env var, CLI flag not passed does not stomp env var (`Changed` is false)

## 3. Migrate otel.go

Replace `allRuntimesEnabled()` with `featureflags.IsEnabled(featureflags.AllRuntimes)` and update tests.

**Files:** `pkg/installer/otel.go` (modify), `pkg/installer/otel_test.go` (modify)

- [ ] 3.1 Remove `allRuntimesEnabled()` function from `otel.go`
- [ ] 3.2 Replace `allEnabled := allRuntimesEnabled()` in `detectAvailableRuntimes()` with `allEnabled := featureflags.IsEnabled(featureflags.AllRuntimes)`
- [ ] 3.3 Remove `"os"` import from `otel.go` if no longer needed
- [ ] 3.4 Update `TestDetectAvailableRuntimes_DefaultEnabled` — replace `t.Setenv("DTWIZ_ALL_RUNTIMES", "")` with `featureflags.SetForTest(t, featureflags.AllRuntimes, false)`
- [ ] 3.5 Update `TestDetectAvailableRuntimes_UnlockAll` — replace `t.Setenv("DTWIZ_ALL_RUNTIMES", "true")` with `featureflags.SetForTest(t, featureflags.AllRuntimes, true)`
- [ ] 3.6 Update `TestDetectAvailableRuntimes_UnlockAll_1` — replace `t.Setenv` and `allRuntimesEnabled()` call with `featureflags.SetForTest` and `featureflags.IsEnabled`
- [ ] 3.7 Verify all existing tests in `otel_test.go` pass: `make test`

## 4. Status display

Add a conditional "Feature Flags" section to `dtwiz status`.

**Files:** `cmd/status.go` (modify)

- [ ] 4.1 After the system analysis section, call `featureflags.List()` and filter to enabled flags
- [ ] 4.2 If any flags are enabled, print a "Feature Flags" header with separator, then each flag's env var name, status, and source
- [ ] 4.3 If no flags are enabled, print nothing (omit the section entirely)

## 5. Verification

End-to-end verification of all flows.

- [ ] 5.1 Run `make test` — all existing and new tests pass
- [ ] 5.2 Run `make lint` — no new lint issues
- [ ] 5.3 Manual: unset all feature flag env vars → `dtwiz install otel` shows only Python projects
- [ ] 5.4 Manual: `export DTWIZ_ALL_RUNTIMES=true` → `dtwiz install otel` shows all runtimes
- [ ] 5.5 Manual: `export DTWIZ_ALL_RUNTIMES=1` → same result as 5.4
- [ ] 5.6 Manual: `dtwiz status` with `DTWIZ_ALL_RUNTIMES=true` → shows active feature flags section
- [ ] 5.7 Manual: `dtwiz status` with no flags set → no feature flags section in output
- [ ] 5.8 Manual: `dtwiz install otel --all-runtimes` → shows all runtimes (CLI flag works)

## 6. Evaluate gating other analyzers/recommendations

Manually evaluate whether Docker, Kubernetes, OneAgent, AWS, Azure, and GCP analysis/recommendations should be gated behind feature flags (Platform, OTel, and Services are already GA and excluded from this evaluation).

- [ ] 6.1 Review each analyzer in `pkg/analyzer/` and each recommendation method in `pkg/recommender/` — determine if any non-GA capability would benefit from a feature flag
- [ ] 6.2 Document findings as a comment on the change or in the PR description
