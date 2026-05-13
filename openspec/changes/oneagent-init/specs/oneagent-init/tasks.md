# OneAgent PoC Scaffolding Tasks

## 0. Investigate and Confirm Assumptions

Before implementing, review the design and spec documents to understand the requirements and constraints.

**Files:** `design.md`, `spec.md`

- [x] 0.1 Read `design.md` and `spec.md` to understand the scaffolding strategy and requirements
- [x] 0.2 Identify and document any unclear assumptions about type definitions or feature-flag behavior
- [x] 0.3 Verify that the feature-flag implementation pattern matches existing flags in `pkg/featureflags/featureflags.go`
- [x] 0.4 Confirm that the CLI flag structure and defaults align with the specification

## 1. Scaffolding and Feature Flag

Set up the foundational structure for the OneAgent PoC implementation: feature flag, stub files, type definitions, CLI flags, and feature-flag branching.

**Files:** `pkg/featureflags/featureflags.go` (extend), `pkg/installer/oneagent_v2.go` (create), `pkg/installer/oneagent_v2_test.go` (create), `cmd/install.go` (extend)

### Feature Flag

- [x] 1.1 Add `OneAgentPoC` constant to the flag enum in `pkg/featureflags/featureflags.go`
- [x] 1.2 Add a registry entry for `ONEAGENT_POC` with env var `DTWIZ_ONEAGENT_POC`, default `false`
- [x] 1.3 Unit test: `IsEnabled(featureflags.OneAgentPoC)` returns `false` by default
- [x] 1.4 Unit test: set `DTWIZ_ONEAGENT_POC=true` and verify `IsEnabled` returns `true`

### Scaffolding: pkg/installer/oneagent_v2.go

- [x] 1.5 Create `pkg/installer/oneagent_v2.go` with type definitions: `Environment`, `AgentConfig`, `InstallOptions`, `Endpoint`, `ConnectivityReport`, `ConnectivityResult`
- [x] 1.6 Define `InstallOptions` struct carrying `DryRun`, `Force`, `MonitoringMode`, `NoVerifySignature`, `SkipConnectivityCheck`, `ConnectivityCheckOnly`, `PrintEndpoints`, `HostGroup`, `Quiet`
- [x] 1.7 Implement stub entry point `InstallOneAgentV2(c *client.Client, opts InstallOptions) error` that returns `errors.New("oneagent v2 not yet implemented")`
- [x] 1.8 Add stub function signatures (not implemented) for: `DetectEnvironment()`, `RunPreflightChecks()`, `ResolveAgentConfig()`, `ResolveEndpoints()`, `MintInstallerToken()`, `DownloadInstaller()`, `VerifyInstallerSignature()`, `BuildInstallCommand()`, `ExecuteInstallCommand()`, `WaitForHostRegistration()`, `CheckAllEndpoints()`
- [x] 1.9 Ensure code compiles with `go build ./...`

### Scaffolding: pkg/installer/oneagent_v2_test.go

- [x] 1.10 Create `pkg/installer/oneagent_v2_test.go` with skeleton structure
- [x] 1.11 Add test helper functions for mocking HTTP responses (placeholder comments for Tasks 2–7)
- [x] 1.12 Ensure tests compile with `go test ./... -v` (mark pending tests with `t.Skip()` if needed)

### CLI Flags

- [x] 1.13 Add flag definitions on `installOneAgentCmd`: `--force`, `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only`, `--print-endpoints`
- [x] 1.14 Each flag is properly documented in the cobra command definition
- [x] 1.15 Unit test: flags parse correctly from the command line

### Feature-Flag Branching

- [x] 1.16 In `cmd/install.go`'s `installOneAgentCmd.RunE`, add branching:

  ```go
  if featureflags.IsEnabled(featureflags.OneAgentPoC) {
      return installer.InstallOneAgentV2(c, opts)
  }
  return installer.InstallOneAgent(c.Classic, installDryRun, quiet, hostGroup)
  ```

- [x] 1.17 Add a comment marking the branching point: `// Task 1 — feature-flag branching; remove at Task 8`
- [x] 1.18 Verify existing `InstallOneAgent` call path is unchanged when the flag is disabled (default)
- [x] 1.19 Integration test: with `DTWIZ_ONEAGENT_POC=true`, verify `InstallOneAgentV2` is called (and returns "not yet implemented")
