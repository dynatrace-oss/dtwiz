# OneAgent Configure Tasks

## 0. Investigate and Confirm Assumptions

Before implementing, review the design and spec documents to understand the requirements and constraints.

**Files:** `design.md`, `spec.md`

- [x] 0.1 Read `design.md` and `spec.md` to understand agent configuration and token resolution requirements
- [x] 0.2 Identify and document any unclear assumptions about API scopes or monitoring mode defaults
- [x] 0.3 Review existing token patterns and security considerations in the codebase
- [x] 0.4 Confirm logging and error handling practices align with the specification

## 4. Resolve Installer Token from Existing Credentials

`classicTok` is already resolved upstream by `validateCredentials` and embedded in `c.Classic`. Token resolution for the download step is deferred to the task that implements `DownloadInstaller`, which will use `c.Classic.HTTP().R()` directly — the same pattern used by all other installers.

**No code changes required in this task.** The credential is available via `c.Classic` when needed.

- [x] 4.1 Confirmed: `validateCredentials` in `cmd/auth.go` resolves access token (override) vs platform token (fallback) before `InstallOneAgentV2` is called — no additional resolution needed at the installer layer
