# OneAgent PoC Scaffolding

> **Note:** The `OneAgentPoC` feature flag and the V1/V2 branching described in this spec were temporary development scaffolding. The flag has since been removed: `InstallOneAgentV2` and `UninstallOneAgentV2` are now the unconditional defaults.

## ADDED Requirements

### Requirement: New package pkg/installer/oneagent/ with core installer logic

`pkg/installer/oneagent/` SHALL be created as a standalone Go package (`package oneagent`) with the installer split across three source files:

- `oneagent.go`: types, entry point `InstallOneAgentV2`, `DefaultAgentConfig`, `ResolveAgentConfig`, `detectRuntimeEnvironment`
- `download.go`: `DownloadInstaller`, `readErrorBody`, `installerOSSegment`, `installerExtension`, `humanBytes`
- `verify.go`: `VerifyInstallerSignature`, `fetchDynatraceRootCA`, `runOpensslVerify`

Type definitions: `Environment`, `AgentConfig`, `InstallOptions`

#### Scenario: package compiles

- **GIVEN** `pkg/installer/oneagent/` is created
- **WHEN** `go build ./...` runs
- **THEN** the code compiles without errors

#### Scenario: Type definitions are exported

- **GIVEN** `Environment`, `AgentConfig`, `InstallOptions` are defined as public types
- **WHEN** test code imports `github.com/dynatrace-oss/dtwiz/pkg/installer/oneagent`
- **THEN** it can construct and reference these types

### Requirement: Test file pkg/installer/oneagent/oneagent_test.go

`pkg/installer/oneagent/oneagent_test.go` SHALL be created with:

- Test helper functions for mocking HTTP server responses (`newMockTenantServer`, `newMockClient`, `newTestClassicClient`, `createStubFile`)
- Full test coverage for `ResolveAgentConfig`, `DownloadInstaller`, `VerifyInstallerSignature`, and `InstallOneAgentV2`
- All tests passing with `go test ./pkg/installer/oneagent/...`

#### Scenario: Test file compiles and all tests pass

- **GIVEN** `pkg/installer/oneagent/oneagent_test.go` exists
- **WHEN** `go test ./pkg/installer/oneagent/... -v` runs
- **THEN** all tests pass without errors

### Requirement: ~~Feature-flag branching in cmd/install.go~~ (removed)

> **Removed:** The feature-flag branch in `cmd/install.go` and `cmd/setup.go` has been deleted. `InstallOneAgentV2` is now called unconditionally.

### Requirement: CLI flag definitions for Task 2–7

New flags SHALL be defined on `installOneAgentCmd` (not on the parent `installCmd`):

| Flag | Type | Default | Purpose | Task |
|---|---|---|---|---|
| `--monitoring-mode` | string | `"fullstack"` | Passed through as `--set-monitoring-mode`; value is not validated — the caller is responsible for passing a value that OneAgent accepts | 2 |
| `--no-verify-signature` | bool | `false` | Skip Linux signature verification | 5 |
| `--skip-connectivity-check` | bool | `false` | Skip connectivity probe | 9 |
| `--connectivity-check-only` | bool | `false` | Run only probe, then exit | 9 |

Pre-existing flags (`--dry-run`, `--quiet`) remain unchanged.

#### Scenario: Flags are available on install-oneagent subcommand

- **GIVEN** `--help` text is displayed for `dtwiz install oneagent`
- **THEN** the flag list includes `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only`

### Requirement: InstallOptions struct carries all CLI-derived inputs

`InstallOptions` struct in `pkg/installer/oneagent/oneagent.go` SHALL carry all CLI flags and options:

```go
type InstallOptions struct {
    DryRun                bool
    MonitoringMode        string
    NoVerifySignature     bool
    SkipConnectivityCheck bool
    ConnectivityCheckOnly bool
    Quiet                 bool
}
```

This struct is populated from `cmd/install.go` and passed to `InstallOneAgentV2`.

#### Scenario: InstallOptions struct is constructible from CLI flags

- **GIVEN** all new CLI flags are defined
- **WHEN** `cmd/install.go` constructs an `InstallOptions` from the flags
- **THEN** the struct carries all values correctly

### Requirement: Core functions implemented in pkg/installer/oneagent/

The following functions SHALL be implemented (not stubs) in `pkg/installer/oneagent/`:

- `DefaultAgentConfig() AgentConfig` — returns config with `MonitoringMode: "fullstack"`
- `ResolveAgentConfig(opts InstallOptions) AgentConfig` — applies opts overrides over defaults
- `DownloadInstaller(c *client.ClassicClient, env Environment) (string, error)` — downloads installer binary
- `VerifyInstallerSignature(env Environment, installerPath string, skip bool) error` — CMS signature verification via openssl
- `detectRuntimeEnvironment() (Environment, error)` — detects OS/arch at runtime

#### Scenario: Functions are implemented and testable

- **GIVEN** `pkg/installer/oneagent/` is built
- **WHEN** tests call `ResolveAgentConfig`, `DownloadInstaller`, or `VerifyInstallerSignature`
- **THEN** the functions execute real logic (not placeholder errors)
