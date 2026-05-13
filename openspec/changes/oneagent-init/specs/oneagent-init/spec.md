# OneAgent PoC Scaffolding and Feature Flag

## ADDED Requirements

### Requirement: Feature flag registry entry for ONEAGENT_POC

`pkg/featureflags/featureflags.go` SHALL add a new flag constant and registry entry for `ONEAGENT_POC`. The flag SHALL default to false (disabled) during development. It SHALL be removable at the end of Task 8 with a single constant deletion and one registry entry removal.

#### Scenario: Feature flag defaults to false

- **GIVEN** `DTWIZ_ONEAGENT_POC` environment variable is not set
- **WHEN** the application initializes feature flags
- **THEN** `featureflags.IsEnabled(featureflags.OneAgentPoC)` returns `false`

#### Scenario: Feature flag can be enabled via environment variable

- **GIVEN** `DTWIZ_ONEAGENT_POC=true` is set in the environment
- **WHEN** the application initializes feature flags
- **THEN** `featureflags.IsEnabled(featureflags.OneAgentPoC)` returns `true`

#### Scenario: Feature flag appears in help and status output

- **GIVEN** the feature flag is registered in the flag registry
- **WHEN** `dtwiz --help` runs and `dtwiz status` is executed
- **THEN** the flag appears in `dtwiz status` output showing its enabled/disabled state, and in help text if feature-flag listing is exposed

### Requirement: Scaffold oneagent_v2.go with stub functions

`pkg/installer/oneagent_v2.go` SHALL be created with:

- Entry point: `InstallOneAgentV2(c *client.Client, opts InstallOptions) error` — documented but not implemented (returns `errors.New("not yet implemented")`)
- Type definitions: `Environment`, `AgentConfig`, `InstallOptions`, `Endpoint`, `ConnectivityReport`, `ConnectivityResult`
- Stub function signatures (not implemented) for all Tasks 2–7 functions: `DetectEnvironment()`, `RunPreflightChecks()`, `ResolveAgentConfig()`, `ResolveEndpoints()`, `MintInstallerToken()`, `DownloadInstaller()`, `VerifyInstallerSignature()`, `BuildInstallCommand()`, `ExecuteInstallCommand()`, `WaitForHostRegistration()`, `CheckAllEndpoints()`

#### Scenario: oneagent_v2.go compiles

- **GIVEN** `pkg/installer/oneagent_v2.go` is created with stub function signatures
- **WHEN** `go build ./...` runs
- **THEN** the code compiles without errors

#### Scenario: Type definitions are exported

- **GIVEN** `Environment`, `AgentConfig`, `InstallOptions`, and `Endpoint` are defined as public types
- **WHEN** test code imports `pkg/installer`
- **THEN** it can construct and reference these types

### Requirement: Scaffold oneagent_v2_test.go with test structure

`pkg/installer/oneagent_v2_test.go` SHALL be created with:

- Test helper functions for mocking HTTP server responses (tenant API, token mint, installer download, etc.)
- Skeleton test cases with TODO placeholders for Tasks 2–7 unit tests
- Comment blocks indicating which task each test batch covers

#### Scenario: Test file compiles

- **GIVEN** `pkg/installer/oneagent_v2_test.go` is created with skeleton test structure
- **WHEN** `go test ./... -v` runs
- **THEN** the tests compile without errors (and any TODO tests are skipped or marked as `t.Skip()`)

### Requirement: Feature-flag branching in cmd/install.go

`cmd/install.go` SHALL add feature-flag branching in the `installOneAgentCmd.RunE` handler to conditionally call `InstallOneAgentV2` or the existing `InstallOneAgent`:

```go
if featureflags.IsEnabled(featureflags.OneAgentPoC) {
    return installer.InstallOneAgentV2(c, opts)
}
return installer.InstallOneAgent(c.Classic, installDryRun, quiet, hostGroup)
```

The branching point SHALL be clearly marked with a comment indicating "Task 1 — feature-flag branching; remove at Task 8".

#### Scenario: Feature flag gates the new flow

- **GIVEN** `DTWIZ_ONEAGENT_POC=false` (default)
- **WHEN** `dtwiz install oneagent` runs
- **THEN** the existing `InstallOneAgent` code path is executed

#### Scenario: Feature flag enables the new flow

- **GIVEN** `DTWIZ_ONEAGENT_POC=true`
- **WHEN** `dtwiz install oneagent` runs
- **THEN** the new `InstallOneAgentV2` code path is executed

### Requirement: CLI flag definitions for Task 2–7

New flags SHALL be defined on `installOneAgentCmd` (not on the parent `installCmd`):

| Flag | Type | Default | Purpose | Task |
|---|---|---|---|---|
| `--monitoring-mode` | string | `"fullstack"` | Passed through as `--set-monitoring-mode`; value is not validated — the caller is responsible for passing a value that OneAgent accepts | 2 |
| `--no-verify-signature` | bool | `false` | Skip Linux signature verification | 5 |
| `--skip-connectivity-check` | bool | `false` | Skip connectivity probe | 9 |
| `--connectivity-check-only` | bool | `false` | Run only probe, then exit | 9 |
| `--print-endpoints` | bool | `false` | Print resolved endpoints, then exit | 9 |

Pre-existing flags (`--dry-run`, `--quiet`) remain unchanged.

#### Scenario: Flags are available on install-oneagent subcommand

- **GIVEN** `--help` text is displayed for `dtwiz install oneagent`
- **THEN** the flag list includes `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only`, `--print-endpoints`

### Requirement: InstallOptions struct carries all CLI-derived inputs

`InstallOptions` struct in `pkg/installer/oneagent_v2.go` SHALL carry all CLI flags and options:

```go
type InstallOptions struct {
    DryRun                bool
    MonitoringMode        string
    NoVerifySignature     bool
    SkipConnectivityCheck bool
    ConnectivityCheckOnly bool
    PrintEndpoints        bool
    Quiet                 bool
}
```

This struct is populated from `cmd/install.go` and passed to `InstallOneAgentV2`.

#### Scenario: InstallOptions struct is constructible from CLI flags

- **GIVEN** all new CLI flags are defined
- **WHEN** `cmd/install.go` constructs an `InstallOptions` from the flags
- **THEN** the struct carries all values correctly

### Requirement: Minimal type signatures for all Task 2–7 functions

All stub functions in `oneagent_v2.go` SHALL have signatures matching their design, so they can be called during integration-test setup even if not yet implemented. Each stub returns a documented placeholder error.

#### Scenario: Function signatures match design

- **GIVEN** the design.md describes `func ResolveEndpoints(c *client.ClassicClient) ([]Endpoint, error)`
- **WHEN** `pkg/installer/oneagent_v2.go` is examined
- **THEN** the function signature matches exactly
