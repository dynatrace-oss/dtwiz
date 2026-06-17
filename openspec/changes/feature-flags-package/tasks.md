# Tasks

## 1. Core featureflags package

Create the `pkg/featureflags` package with the registry, `IsEnabled`, `List`, and `SetForTest`.

**Files:** `pkg/featureflags/featureflags.go` (create), `pkg/featureflags/featureflags_test.go` (create)

- [x] 1.1 Create `pkg/featureflags/featureflags.go` with `Flag` type (int const), `CLIFeatureFlag` struct (flag, name, envVar, defaultVal, desc, bound), and `registry` slice with the `AllRuntimes` entry (`"all-runtimes"`, `"DTWIZ_ALL_RUNTIMES"`, `false`); add unexported helpers `getFlag(flag Flag) *CLIFeatureFlag` (linear registry scan) and `resolveFlag(r *CLIFeatureFlag) (bool, string)` (precedence resolution)
- [x] 1.2 Implement `IsEnabled(flag Flag) bool` — resolution order: CLI override → env var (`"true"`/`"1"`) → default
- [x] 1.3 Implement `List() []FlagState` — iterate registry, resolve each flag, return name, env var, enabled state, and source
- [x] 1.4 Implement `SetCLIOverrideForTest(t testCleaner, flag Flag, val bool)` in `test_utils.go` — store override in mutex-protected `cliOverrides` map, restore previous value via `t.Cleanup`
- [x] 1.5 Add unit tests: default returns false, env var `"true"` returns true, env var `"1"` returns true, env var `"false"` returns false, unknown flag returns false, `SetCLIOverrideForTest` overrides and restores, `List` returns a correct source for each override type

## 2. Cobra integration

Wire feature flags into the cobra command tree as persistent boolean flags on `rootCmd`.

**Files:** `pkg/featureflags/featureflags.go` (modify), `cmd/root.go` (modify)

- [x] 2.1 Add `RegisterFlags(flags *pflag.FlagSet)` — iterate registry, call `flags.BoolVar` for each entry binding cobra's output to the `bound` field of the registry entry
- [x] 2.2 Add `ApplyCLIOverrides(flags *pflag.FlagSet)` — iterate registry, check `flags.Changed(name)`, store value in `cliOverrides` map if changed
- [x] 2.3 In `cmd/root.go` `init()`, call `featureflags.RegisterFlags(rootCmd.PersistentFlags())` after existing flag registrations
- [x] 2.4 In `rootCmd.PersistentPreRun`, call `featureflags.ApplyCLIOverrides(cmd.Flags())` after `logger.Init`
- [x] 2.5 Add tests: CLI override takes precedence over env var, CLI flag not passed does not stomp env var (`Changed` is false)

## 3. Migrate otel.go

Replace `allRuntimesEnabled()` with `featureflags.IsEnabled(featureflags.AllRuntimes)` and update tests.

**Files:** `pkg/installer/otel.go` (modify), `pkg/installer/otel_test.go` (modify)

- [x] 3.1 Remove `allRuntimesEnabled()` function from `otel.go`
- [x] 3.2 Replace `allEnabled := allRuntimesEnabled()` in `detectAvailableRuntimes()` with `allEnabled := featureflags.IsEnabled(featureflags.AllRuntimes)`
- [x] 3.3 Remove `"os"` import from `otel.go` if no longer needed
- [x] 3.4 Update `TestDetectAvailableRuntimes_DefaultEnabled` — replace `t.Setenv("DTWIZ_ALL_RUNTIMES", "")` with `featureflags.SetCLIOverrideForTest(t, featureflags.AllRuntimes, false)`
- [x] 3.5 Update `TestDetectAvailableRuntimes_UnlockAll` — replace `t.Setenv("DTWIZ_ALL_RUNTIMES", "true")` with `featureflags.SetCLIOverrideForTest(t, featureflags.AllRuntimes, true)`
- [x] 3.6 Update `TestDetectAvailableRuntimes_UnlockAll_1` — replace `t.Setenv` and `allRuntimesEnabled()` call with `featureflags.SetCLIOverrideForTest` and `featureflags.IsEnabled`
- [x] 3.7 Verify all existing tests in `otel_test.go` pass: `make test`

## 5. Verification

End-to-end verification of all flows.

- [x] 5.1 Run `make test` — all existing and new tests pass
- [x] 5.2 Run `make lint` — no new lint issues
- [x] 5.3 Manual: unset all feature flag env vars → `dtwiz install otel` shows only Python projects
- [x] 5.4 Manual: `export DTWIZ_ALL_RUNTIMES=true` → `dtwiz install otel` shows all runtimes
- [x] 5.5 Manual: `export DTWIZ_ALL_RUNTIMES=1` → same result as 5.4
- [x] 5.6 Manual: `dtwiz install otel --all-runtimes` → shows all runtimes (CLI flag works)

## 6. Evaluate gating other analyzers/recommendations

Manually evaluate whether Docker, Kubernetes, OneAgent, AWS, Azure, and GCP analysis/recommendations should be gated behind feature flags (Platform, OTel, and Services are already GA and excluded from this evaluation).

- [x] 6.1 Review each analyzer in `pkg/analyzer/` and each recommendation method in `pkg/recommender/` — determine if any non-GA capability would benefit from a feature flag
- [x] 6.2 Document findings: Docker recommendation, demo app installation, and OTel Collector update are gated behind `Experimental` (`--experimental` / `DTWIZ_EXPERIMENTAL`). Kubernetes, OneAgent, AWS, Azure, and GCP are GA and remain ungated.

## 7. Gate `dtwiz update otel` behind `Experimental`

Gate the `dtwiz update otel` command and the corresponding `setup` recommendation behind the `Experimental` feature flag.

**Files:** `cmd/update.go` (modify), `cmd/setup.go` (modify), `cmd/cancel_test.go` (modify)

- [x] 7.1 Set `Hidden: true` on `updateOtelCmd` and add feature flag guard at the top of its `RunE` returning an error when `Experimental` is not enabled
- [x] 7.2 Fix `updateCmd.PersistentPreRun` to call `logger.Init`, `logger.Verbose`, `logger.Debug`, and `featureflags.ApplyCLIOverrides` — it overrides root's `PersistentPreRun`, so `--experimental` was not being applied for `update` subcommands
- [x] 7.3 Add `updateCmd.HelpFunc` override to call `ApplyCLIOverrides` and toggle `updateOtelCmd.Hidden` before rendering help, so `--experimental --help` reveals the subcommand
- [x] 7.4 In `cmd/setup.go`, extend the experimental filter to also skip `MethodOtelUpdate` recommendations when `Experimental` is not enabled
- [x] 7.5 Add tests in `cmd/cancel_test.go`: `TestUpdateOtelCmd_HiddenByDefault`, `TestUpdateOtelCmd_VisibleWhenExperimental`, `TestUpdateOtelCmd_RunE_BlockedWithoutExperimental`
- [x] 7.6 Update the `--experimental` cobra flag description in `pkg/featureflags/featureflags.go` to mention OTel update alongside demo and Docker, so `dtwiz --help` reflects all gated features
