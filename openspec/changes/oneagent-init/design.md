# Design

## Context

The OneAgent PoC epic runs Tasks 1–11 to implement a production-ready installer flow. Tasks 2–7 build the core logic (environment detection, pre-flights, token minting, download, verification, execution, post-install confirmation). Tasks 8–11 add refinements and cross-platform support. All subsequent tasks depend on:

1. A feature flag (`ONEAGENT_POC`) to gate the new flow during development and make rollback trivial.
2. Type definitions (`Environment`, `AgentConfig`, `Endpoint`, etc.) so Tasks 2–7 can be implemented incrementally without redefining types.
3. CLI flags for all new options so callers in `cmd/install.go` have a unified way to pass options to `InstallOneAgentV2`.
4. Stub function signatures matching the design so Tasks 2–7 can be implemented in any order.
5. Feature-flag branching in the command handler so both old and new flows coexist until Task 8.

This change provides all five without implementing any business logic.

## Goals / Non-Goals

**Goals:**

- Lay the scaffolding for all Tasks 2–11: types, flags, stubs, branching.
- Ensure the code compiles cleanly (stubs return documented placeholder errors).
- Keep the feature flag and branching changes minimal and mechanical — designed for easy removal at Task 8.

**Non-Goals:**

- Implementing any installer logic (environment detection, token minting, etc.) — those are Tasks 2–7.
- Changing the existing `InstallOneAgent` flow — it remains the default path.
- Defining or validating CLI option values (e.g., no allow-list for `--monitoring-mode`).

## Decisions

### 1. Feature flag: `ONEAGENT_POC`, disabled by default

The flag lives in `pkg/featureflags/featureflags.go` as a single constant and registry entry:

```go
const (
    AllRuntimes Flag = iota
    OneAgentPoC
)

// registry entry: name "oneagent-poc", env "DTWIZ_ONEAGENT_POC", default false
```

Default-false means existing behavior is preserved. At Task 8, the constant and registry entry are deleted in a single mechanical commit; no search-and-replace needed.

### 2. New package: pkg/installer/oneagent/

All new code lives in a dedicated `oneagent` package, leaving the rest of `pkg/installer/` untouched. The package is split into three files for clarity:

- `oneagent.go` — types (`Environment`, `AgentConfig`, `InstallOptions`), entry point `InstallOneAgentV2`, `DefaultAgentConfig`, `ResolveAgentConfig`, `detectRuntimeEnvironment`
- `download.go` — `DownloadInstaller`, `readErrorBody`, `installerOSSegment`, `installerExtension`, `humanBytes`
- `verify.go` — `VerifyInstallerSignature`, `fetchDynatraceRootCA`, `runOpensslVerify`, `dtRootCertURL`

This keeps each concern isolated and makes rollback mechanical (delete the package directory).

### 3. Type definitions: `Environment`, `AgentConfig`, `InstallOptions`

All types are exported from `pkg/installer/oneagent` and structured so they can be populated and tested independently:

```go
type Environment struct {
    OS        string
    Arch      string
    Supported bool
    Reason    string
}

type AgentConfig struct {
    MonitoringMode string
}

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

### 4. CLI flag definitions on `installOneAgentCmd`

All new flags are added to the `installOneAgentCmd` Cobra command definition (not the parent `installCmd`, which already owns `--dry-run`):

| Flag | Type | Default |
|---|---|---|
| `--monitoring-mode` | string | `"fullstack"` |
| `--no-verify-signature` | bool | `false` |
| `--skip-connectivity-check` | bool | `false` |
| `--connectivity-check-only` | bool | `false` |
| `--print-endpoints` | bool | `false` |

 Pre-existing flags (`--dry-run`, `--quiet`, `--host-group`) remain unchanged on their current parent command(s). `--host-group` continues to apply to the existing v1 installer flow; the v2 scaffolding in this change does not model it in `InstallOptions` yet.

### 5. InstallOptions struct from CLI

`cmd/install.go`'s `installOneAgentCmd.RunE` constructs an `InstallOptions` struct from the flags:

```go
opts := oneagent.InstallOptions{
    DryRun:                installDryRun,
    MonitoringMode:        flagMonitoringMode,
    NoVerifySignature:     flagNoVerifySignature,
    SkipConnectivityCheck: flagSkipConnectivityCheck,
    ConnectivityCheckOnly: flagConnectivityCheckOnly,
    PrintEndpoints:        flagPrintEndpoints,
    Quiet:                 quiet,
}
```

This replaces the positional `(dryRun, quiet, hostGroup)` signature at Task 8 but uses the new struct immediately.

### 6. Feature-flag branching in cmd/install.go

The command handler branches based on the feature flag:

```go
// Task 1 — feature-flag branching; remove at Task 8
if featureflags.IsEnabled(featureflags.OneAgentPoC) {
    return oneagent.InstallOneAgentV2(c, opts)
}

// Existing flow (default)
return installer.InstallOneAgent(c.Classic, installDryRun, quiet, hostGroup)
```

The comment is a clear marker for Task 8 search-and-replace: remove the if-block, keep only the existing `InstallOneAgent` call, then replace it with the `InstallOneAgentV2` code at Task 8.

### 7. Test file: pkg/installer/oneagent/oneagent_test.go

Created with:

- Helper functions for mocking HTTP server responses (`newMockTenantServer`, `newMockClient`, `newTestClassicClient`, `createStubFile`)
- Full test coverage for `ResolveAgentConfig`, `DownloadInstaller`, `VerifyInstallerSignature`, and `InstallOneAgentV2`
- All tests compilable and passing with `go test ./...`

### 8. No breaking changes during development

The existing `InstallOneAgent` signature and behavior are unchanged. By default (with `ONEAGENT_POC=false`), `dtwiz install oneagent` runs the old flow. Only when the flag is explicitly enabled does it attempt the new flow (which returns "not yet implemented" until Tasks 2–7 are complete).

## Risks / Trade-offs

- **Feature flag adds a code path:** During development, both code paths exist. The old path must continue working. This is the trade-off for parallel development and trivial rollback.
- **Stub errors are not user-friendly:** Until Tasks 2–7 are implemented, enabling the feature flag produces "not yet implemented" errors. This is intentional — the flag is for development/testing only, not for users.
