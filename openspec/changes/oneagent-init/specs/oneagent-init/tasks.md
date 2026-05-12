# OneAgent PoC Scaffolding Tasks

## 1. Scaffolding and Feature Flag

Set up the foundational structure for the OneAgent PoC implementation: feature flag, stub files, type definitions, CLI flags, and feature-flag branching.

**Files:** `pkg/featureflags/featureflags.go` (extend), `pkg/installer/oneagent_v2.go` (create), `pkg/installer/oneagent_v2_test.go` (create), `cmd/install.go` (extend)

### Feature Flag

- [ ] 1.1 Add `OneAgentPoC` constant to the flag enum in `pkg/featureflags/featureflags.go`
- [ ] 1.2 Add a registry entry for `ONEAGENT_POC` with env var `DTWIZ_ONEAGENT_POC`, default `false`
- [ ] 1.3 Unit test: `IsEnabled(featureflags.OneAgentPoC)` returns `false` by default
- [ ] 1.4 Unit test: set `DTWIZ_ONEAGENT_POC=true` and verify `IsEnabled` returns `true`

### Scaffolding: pkg/installer/oneagent_v2.go

- [ ] 1.5 Create `pkg/installer/oneagent_v2.go` with type definitions: `Environment`, `AgentConfig`, `InstallOptions`, `Endpoint`, `ConnectivityReport`, `ConnectivityResult`
- [ ] 1.6 Define `InstallOptions` struct carrying `DryRun`, `Force`, `MonitoringMode`, `NoVerifySignature`, `SkipConnectivityCheck`, `ConnectivityCheckOnly`, `PrintEndpoints`, `HostGroup`, `Quiet`
- [ ] 1.7 Implement stub entry point `InstallOneAgentV2(c *client.Client, opts InstallOptions) error` that returns `errors.New("oneagent v2 not yet implemented")`
- [ ] 1.8 Add stub function signatures (not implemented) for: `DetectEnvironment()`, `RunPreflightChecks()`, `ResolveAgentConfig()`, `ResolveEndpoints()`, `MintInstallerToken()`, `DownloadInstaller()`, `VerifyInstallerSignature()`, `BuildInstallCommand()`, `ExecuteInstallCommand()`, `WaitForHostRegistration()`, `CheckAllEndpoints()`
- [ ] 1.9 Ensure code compiles with `go build ./...`

### Scaffolding: pkg/installer/oneagent_v2_test.go

- [ ] 1.10 Create `pkg/installer/oneagent_v2_test.go` with skeleton structure
- [ ] 1.11 Add test helper functions for mocking HTTP responses (placeholder comments for Tasks 2–7)
- [ ] 1.12 Ensure tests compile with `go test ./... -v` (mark pending tests with `t.Skip()` if needed)

### CLI Flags

- [ ] 1.13 Add flag definitions on `installOneAgentCmd`: `--force`, `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only`, `--print-endpoints`
- [ ] 1.14 Each flag is properly documented in the cobra command definition
- [ ] 1.15 Unit test: flags parse correctly from the command line

### Feature-Flag Branching

- [ ] 1.16 In `cmd/install.go`'s `installOneAgentCmd.RunE`, add branching:

  ```go
  if featureflags.IsEnabled(featureflags.OneAgentPoC) {
      return installer.InstallOneAgentV2(c, opts)
  }
  return installer.InstallOneAgent(c.Classic, installDryRun, quiet, hostGroup)
  ```

- [ ] 1.17 Add a comment marking the branching point: `// Task 1 — feature-flag branching; remove at Task 8`
- [ ] 1.18 Verify existing `InstallOneAgent` call path is unchanged when the flag is disabled (default)
- [ ] 1.19 Integration test: with `DTWIZ_ONEAGENT_POC=true`, verify `InstallOneAgentV2` is called (and returns "not yet implemented")
