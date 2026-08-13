# Feature Flag Registry

## Purpose

Define the `pkg/featureflags` central registry that manages known feature flags with declarative definitions.

## Requirements

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

The package SHALL expose `List() []FlagState` returning all registered flags with their current resolved value and the source of that value (`"cli"`, `"env"`, or `"default"`).

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

### Requirement: `Experimental` flag gates command visibility in help output

Experimental subcommands (`install docker`, `install demo`, and `update otel`) SHALL be registered with `Hidden: true` and unhidden only when `featureflags.IsEnabled(featureflags.Experimental)` is `true`. Commands that conditionally show hidden subcommands SHALL override their `HelpFunc` to call `ApplyCLIOverrides` before rendering. Execution of an experimental command without the flag SHALL return an error directing the user to `--experimental` or `DTWIZ_EXPERIMENTAL=true`. Any command that defines its own `PersistentPreRun` SHALL call `featureflags.ApplyCLIOverrides`.

#### Scenario: Experimental install command hidden from help by default

- **GIVEN** neither `--experimental` nor `DTWIZ_EXPERIMENTAL` is set
- **WHEN** a user runs `dtwiz install --help`
- **THEN** the `docker` subcommand does NOT appear in the available commands list

#### Scenario: Experimental install command visible with env var

- **GIVEN** `DTWIZ_EXPERIMENTAL=true` is set in the environment
- **WHEN** a user runs `dtwiz install --help`
- **THEN** the `docker` subcommand appears in the available commands list

#### Scenario: Experimental install command visible with CLI flag

- **GIVEN** `--experimental` is passed on the command line
- **WHEN** a user runs `dtwiz install --experimental --help`
- **THEN** the `docker` subcommand appears in the available commands list

#### Scenario: Experimental install command blocked at execution without flag

- **GIVEN** neither `--experimental` nor `DTWIZ_EXPERIMENTAL` is set
- **WHEN** a user runs `dtwiz install docker`
- **THEN** the command exits with an error: `docker installation is an experimental feature; enable it with --experimental or DTWIZ_EXPERIMENTAL=true`

#### Scenario: `update otel` hidden from help by default

- **GIVEN** neither `--experimental` nor `DTWIZ_EXPERIMENTAL` is set
- **WHEN** a user runs `dtwiz update --help`
- **THEN** the `otel` subcommand does NOT appear in the available commands list

#### Scenario: `update otel` visible with experimental flag

- **GIVEN** `--experimental` or `DTWIZ_EXPERIMENTAL=true` is set
- **WHEN** a user runs `dtwiz update --help`
- **THEN** the `otel` subcommand appears in the available commands list

#### Scenario: `update otel` blocked at execution without flag

- **GIVEN** neither `--experimental` nor `DTWIZ_EXPERIMENTAL` is set
- **WHEN** a user runs `dtwiz update otel`
- **THEN** the command exits with an error: `otel update is an experimental feature; enable it with --experimental or DTWIZ_EXPERIMENTAL=true`

### Requirement: `Experimental` flag gates `setup` recommendations

The `dtwiz setup` interactive flow SHALL exclude experimental recommendations from the selectable list when `Experimental` is not enabled. Specifically, `MethodOtelUpdate` (patch existing OTel Collector) and `MethodDocker` SHALL NOT appear as options unless `featureflags.IsEnabled(featureflags.Experimental)` returns `true`.

#### Scenario: OTel update option absent from setup without experimental

- **GIVEN** an OTel Collector is running on the host
- **AND** neither `--experimental` nor `DTWIZ_EXPERIMENTAL` is set
- **WHEN** a user runs `dtwiz setup`
- **THEN** the "patch existing OpenTelemetry Collector" option does NOT appear in the recommendation list

#### Scenario: OTel update option present in setup with experimental

- **GIVEN** an OTel Collector is running on the host
- **AND** `--experimental` or `DTWIZ_EXPERIMENTAL=true` is set
- **WHEN** a user runs `dtwiz setup`
- **THEN** the "patch existing OpenTelemetry Collector" option appears in the recommendation list

### Requirement: Test helper `SetCLIOverrideForTest`

The package SHALL expose `SetCLIOverrideForTest(t testCleaner, flag Flag, val bool)` in `test_utils.go` that injects a CLI-scoped override, equivalent to the user having passed the flag explicitly on the command line. The override SHALL be automatically removed via `t.Cleanup`.

#### Scenario: CLI override scoped to test

- **GIVEN** a test calls `SetCLIOverrideForTest(t, AllRuntimes, true)`
- **WHEN** `IsEnabled(AllRuntimes)` is called within the test
- **THEN** it returns `true` with source `"cli"`
- **AND** after the test completes, the override is removed

### Requirement: Minimal dependencies

The `pkg/featureflags` package SHALL use only Go standard library packages and `github.com/spf13/pflag`. The `testing` package SHALL NOT be imported in production code; test helpers SHALL use a local `testCleaner` interface instead.

#### Scenario: No testing import in production code

- **GIVEN** the `pkg/featureflags` package is built
- **WHEN** `pkg/featureflags/featureflags.go` is inspected
- **THEN** it does not import `"testing"` or any test-only package

### Requirement: Backward compatibility

The `DTWIZ_ALL_RUNTIMES` environment variable SHALL continue to work identically to the current behavior. Values `"true"` and `"1"` SHALL enable the flag; all other values (including empty string) SHALL leave it disabled.

#### Scenario: Legacy env var with "true"

- **GIVEN** `DTWIZ_ALL_RUNTIMES=true` is set
- **WHEN** `IsEnabled(AllRuntimes)` is called
- **THEN** it returns `true`
