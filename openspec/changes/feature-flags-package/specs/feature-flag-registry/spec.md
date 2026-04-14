# Feature Flag Registry

## ADDED Requirements

### Requirement: Central registry with declarative flag definitions

The `pkg/featureflags` package SHALL maintain a registry of known feature flags. Each flag is defined by a constant, an environment variable name, a default value, and a cobra flag name. Adding a new feature flag SHALL require only a single constant definition and a single registry entry — no changes to consuming code patterns.

#### Scenario: New flag added to the registry

- **GIVEN** a developer wants to gate a new feature behind a flag
- **WHEN** they add a `const` and a registry entry in `pkg/featureflags/featureflags.go`
- **THEN** the flag is automatically available via `IsEnabled()`, `List()`, cobra CLI flags, and env var lookup — no other files need changes

### Requirement: `IsEnabled(flag)` as single entry point

The package SHALL expose `IsEnabled(flag) bool` as the single entry point for checking whether a feature flag is enabled. Resolution order: test override → CLI flag → environment variable → default value.

#### Scenario: Default value — flag not set anywhere

- **GIVEN** the `AllRuntimes` flag has default `false`
- **AND** no env var `DTWIZ_ALL_RUNTIMES` is set
- **AND** no `--all-runtimes` CLI flag is passed
- **WHEN** `IsEnabled(AllRuntimes)` is called
- **THEN** it returns `false`

#### Scenario: Env var override with "true"

- **GIVEN** `DTWIZ_ALL_RUNTIMES=true` is set in the environment
- **AND** no CLI flag overrides it
- **WHEN** `IsEnabled(AllRuntimes)` is called
- **THEN** it returns `true`

#### Scenario: Env var override with "1"

- **GIVEN** `DTWIZ_ALL_RUNTIMES=1` is set in the environment
- **WHEN** `IsEnabled(AllRuntimes)` is called
- **THEN** it returns `true`

#### Scenario: CLI flag overrides env var

- **GIVEN** `DTWIZ_ALL_RUNTIMES=false` is set in the environment
- **AND** `--all-runtimes` is passed on the command line
- **WHEN** `IsEnabled(AllRuntimes)` is called
- **THEN** it returns `true` (CLI takes precedence)

#### Scenario: Unknown flag returns false

- **GIVEN** a `Flag` value that is not registered in the registry
- **WHEN** `IsEnabled(unknownFlag)` is called
- **THEN** it returns `false`

### Requirement: `List()` returns all flags with resolved state

The package SHALL expose `List() []FlagState` returning all registered flags with their current resolved value and the source of that value (`"cli"`, `"env"`, `"default"`, or `"test"`).

#### Scenario: Mixed sources

- **GIVEN** `AllRuntimes` is enabled via env var
- **WHEN** `List()` is called
- **THEN** it returns a `FlagState` with `Name: "all-runtimes"`, `EnvVar: "DTWIZ_ALL_RUNTIMES"`, `Enabled: true`, `Source: "env"`

### Requirement: Cobra flag registration

The package SHALL expose `RegisterFlags(flags *pflag.FlagSet)` that registers a persistent boolean flag for each registry entry. The package SHALL expose `ApplyCLIOverrides(flags *pflag.FlagSet)` that checks `flags.Changed(name)` for each entry and stores CLI-set values as overrides. `ApplyCLIOverrides` SHALL be called from `rootCmd.PersistentPreRun`.

#### Scenario: Cobra flag registered and discoverable

- **GIVEN** `RegisterFlags` has been called with the root command's persistent flags
- **WHEN** a user runs `dtwiz --help`
- **THEN** `--all-runtimes` appears in the flag list with its description

#### Scenario: Cobra flag not passed — env var not stomped

- **GIVEN** `DTWIZ_ALL_RUNTIMES=true` is set in the environment
- **AND** `--all-runtimes` is NOT passed on the command line
- **WHEN** `ApplyCLIOverrides` runs and then `IsEnabled(AllRuntimes)` is called
- **THEN** it returns `true` (cobra's default `false` does NOT override the env var)

### Requirement: Test helper `SetForTest`

The package SHALL expose `SetForTest(t testing.TB, flag Flag, val bool)` that sets a test-scoped override. The override SHALL be automatically removed via `t.Cleanup`. Test overrides take highest precedence in the resolution order.

#### Scenario: Test override scoped to test

- **GIVEN** a test calls `SetForTest(t, AllRuntimes, true)`
- **WHEN** `IsEnabled(AllRuntimes)` is called within the test
- **THEN** it returns `true`
- **AND** after the test completes, the override is removed

### Requirement: Zero external dependencies

The `pkg/featureflags` package SHALL use only Go standard library packages (plus `testing` for the test helper). The cobra integration (`pflag.FlagSet`) is the only non-stdlib dependency, and it is already used throughout the project.

### Requirement: Backward compatibility

The `DTWIZ_ALL_RUNTIMES` environment variable SHALL continue to work identically to the current behavior. Values `"true"` and `"1"` enable the flag; all other values (including empty string) leave it disabled.
