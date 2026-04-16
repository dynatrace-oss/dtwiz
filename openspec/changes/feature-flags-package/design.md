# Design

## Context

`pkg/installer/otel.go` has a single `allRuntimesEnabled()` function that calls `os.Getenv("DTWIZ_ALL_RUNTIMES")` to gate Java, Node.js, and Go runtimes behind a flag. Three tests in `otel_test.go` exercise this via `t.Setenv`. The `dtwiz status` command (`cmd/status.go`) shows connection status and system analysis but has no feature flag awareness.

The project follows cobra conventions: persistent flags on root or parent commands, shared via package-level variables. Existing credential flags (`--environment`, `--access-token`, `--platform-token`) are registered as `PersistentFlags().StringVar()` on `rootCmd`.

dtctl (sibling project) has no centralized feature flag system. Its `pkg/aidetect/detect.go` uses a registry-map pattern — a `map[string]string` of env vars to meanings, checked in a single function — which is the closest analog. Its `cmd/preview_notice.go` attaches `[Preview]` notices to experimental commands via `PersistentPreRunE` hooks, but this is about labeling commands, not gating behavior.

## Goals / Non-Goals

**Goals:**

- Create a `pkg/featureflags` package that centralizes flag registration, env var lookup, CLI override, and testing.
- Expose each feature flag as a cobra persistent boolean flag on `rootCmd`, following the same pattern as `--environment` and `--access-token`.
- Migrate `allRuntimesEnabled()` in `otel.go` to `featureflags.IsEnabled(featureflags.AllRuntimes)`.
- Show active feature flags in `dtwiz status`.
- Provide `SetForTest(t, flag, val)` so tests can override flags without `os.Setenv`.

**Non-Goals:**

- Remote feature flag evaluation (LaunchDarkly, etc.) — this is local-only.
- Runtime notices or `[Preview]` banners when a flag-gated feature activates (status output is sufficient).
- Gating analyzer/recommender capabilities behind feature flags (evaluated and not needed — see task 6).

## Decisions

### 1. Package structure: single file, declarative registry

All feature flag logic lives in `pkg/featureflags/featureflags.go` with tests in `featureflags_test.go`. The registry is a package-level slice of `CLIFeatureFlag` structs:

```go
type Flag int

const (
    AllRuntimes Flag = iota
)

type CLIFeatureFlag struct {
    flag       Flag
    name       string // kebab-case, used as cobra flag name: --all-runtimes
    envVar     string // env var name: DTWIZ_ALL_RUNTIMES
    defaultVal bool   // default value
    desc       string // cobra flag description
    bound      bool   // bound variable for cobra BoolVar; written by cobra when the flag is parsed
}

var registry = []CLIFeatureFlag{
    {AllRuntimes, "all-runtimes", "DTWIZ_ALL_RUNTIMES", false, "enable all runtimes including experimental (Java, Node.js, Go)", false},
}
```

Adding a new flag requires adding one `const` and one `CLIFeatureFlag` entry. No changes elsewhere.

**Alternative considered:** `map[Flag]CLIFeatureFlag` for O(1) lookup. Rejected — the registry will have single-digit entries; a slice with linear scan is simpler, ordered, and sufficient.

### 2. Resolution order: CLI flag → env var → default

`IsEnabled(flag)` resolves in this order:

1. **CLI override** — if a cobra flag was explicitly set (`--all-runtimes` / `--all-runtimes=false`), use that value. Tracked via a package-level `cliOverrides map[Flag]bool` populated by `ApplyCLIOverrides(flags *pflag.FlagSet)`, which checks `flags.Changed(name)` for each registered flag and stores only explicitly set values.
2. **Env var** — check `os.Getenv(envVar)`. Values `"true"` and `"1"` → enabled; everything else → disabled.
3. **Default** — the `defaultVal` field from the registry.

**Alternative considered:** A single `--feature-flag key=value` map flag (like `--set` in Helm). Rejected — individual cobra flags are more discoverable (`--help` lists them), support shell completion, and follow the existing pattern of per-flag registration on `rootCmd`.

### 3. Cobra integration via `RegisterFlags` + `ApplyCLIOverrides`

The package exposes two functions for cmd integration:

- `RegisterFlags(flags *pflag.FlagSet)` — called in `cmd/root.go` `init()`. Iterates the registry and registers a `BoolVar` for each flag, binding cobra's output to the `bound` field of each registry entry.
- `ApplyCLIOverrides(flags *pflag.FlagSet)` — called in `rootCmd.PersistentPreRun`. For each registry entry, checks `flags.Changed(name)` to detect if the user explicitly set the flag. If changed, stores the value in `cliOverrides`. This ensures env vars are not overridden by cobra's default `false`.

Two unexported helpers support the public API:

- `getFlag(flag Flag) *CLIFeatureFlag` — linear scan of the registry returning a pointer to the matching entry, or `nil` for unknown flags.
- `resolveFlag(r *CLIFeatureFlag) (bool, string)` — resolves a single entry's value and source following the precedence order.

This two-step approach is necessary because cobra initializes all bool flags to `false`. Without checking `Changed`, `--all-runtimes` not being passed would always resolve to `false`, stomping the env var.

### 4. Test helper: `SetForTest(t, flag, val)`

```go
func SetForTest(t testing.TB, flag Flag, val bool) {
    // store override in testOverrides map
    // t.Cleanup restores previous state
}
```

Resolution order with test override: **test override → CLI → env var → default**. This allows tests to control flags without touching the environment or cobra state. The override is scoped to the test via `t.Cleanup`.

The `testOverrides` map is package-level and protected by a mutex. This is safe with `t.Parallel()` because each test sets its own flag key, and `t.Cleanup` restores the previous value.

### 5. `List()` returns all flags with current state

```go
type FlagState struct {
    Name    string // "all-runtimes"
    EnvVar  string // "DTWIZ_ALL_RUNTIMES"
    Enabled bool   // resolved value
    Source  string // "cli", "env", "default", or "test"
}

func List() []FlagState
```

`Source` indicates where the value came from — useful for debugging. `dtwiz status` uses `List()` to display active flags.

### 6. Status display: conditional section

`cmd/status.go` calls `featureflags.List()` after system analysis. If any flag is enabled, a new section is printed:

```text
  Feature Flags
  ──────────────────────────────────────────
  DTWIZ_ALL_RUNTIMES  ✓ enabled (env)
```

If no flags are enabled, the section is omitted entirely — keeping the default output clean.

### 7. Migration in `otel.go`

The `allRuntimesEnabled()` function in `otel.go` is replaced with `featureflags.IsEnabled(featureflags.AllRuntimes)`. The function is removed. `detectAvailableRuntimes()` calls `featureflags.IsEnabled` directly.

The three test functions in `otel_test.go` are updated to use `featureflags.SetForTest(t, featureflags.AllRuntimes, true/false)` instead of `t.Setenv("DTWIZ_ALL_RUNTIMES", ...)`.

## Risks / Trade-offs

- **[Package-level mutable state]** → `cliOverrides` and `testOverrides` are package-level maps mutated at runtime. Mitigation: `cliOverrides` is written once during `PersistentPreRun` and read-only after; `testOverrides` is mutex-protected and scoped via `t.Cleanup`. The mutex covers only `testOverrides` reads and writes — `cliOverrides` is intentionally unprotected because it is written exactly once at startup before any concurrent access is possible.
- **[Cobra flag namespace]** → Feature flags share the global flag namespace with operational flags (`--environment`, `--dry-run`). Mitigation: use descriptive names (`--all-runtimes`) and document that these are feature flags in the help text.
- **[Not applicable to non-boolean flags]** → The registry only supports boolean flags. Mitigation: all current and foreseeable feature flags are on/off toggles. String/int flags can be added later if needed.
