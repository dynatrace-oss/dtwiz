# OneAgent Windows Tasks

## 11. Windows-Specific Support

Complete Windows-specific implementation not covered inline by earlier tasks: correct installer download URL/extension, Authenticode signature verification via PowerShell, and temp-file permission handling. Earlier tasks (2.7b, 5.3, 6.2, 7) reference Windows but leave the platform-specific logic as stubs or TODOs. This spec consolidates all Windows-specific work.

**Files:** `pkg/installer/oneagent_v2.go` (extend), `pkg/installer/oneagent_v2_test.go` (extend), `pkg/installer/preflight_windows.go` (create or extend), `pkg/installer/sudo_windows.go` (extend)

### Part A — Windows installer download

- [ ] 11.1 In `DownloadInstaller` (Task 5.1), map `env.OS == "windows"` to the installer request path/query `/api/v1/deployment/installer/agent/windows/default/latest?arch=x86`; the Linux branch uses `/unix/default/<arch>/`
- [ ] 11.2 Save the Windows temp file with a `.exe` extension so the OS recognises it as an executable (use `os.CreateTemp("", "dynatrace-oneagent-*.exe")`)
- [ ] 11.3 Skip `os.Chmod(tmpPath, 0o700)` on Windows — NTFS ACLs are not meaningful in the same way; guard with `if runtime.GOOS != "windows"`
- [ ] 11.4 Unit tests: `DownloadInstaller` with `env.OS == "windows"` produces a request URL containing the Windows path segment and a temp file ending in `.exe`; `env.OS == "linux"` path is unchanged

### Part B — Windows Authenticode signature verification

- [ ] 11.5 Extend `VerifyInstallerSignature` (Task 5.6): when `env.OS == "windows"` and `skip == false`, verify the installer's Authenticode signature using PowerShell
- [ ] 11.6 Locate `powershell.exe` via `exec.LookPath`; return `"powershell.exe is required for signature verification on Windows. Pass --no-verify-signature to skip."` if not found
- [ ] 11.7 Run `powershell.exe -NoProfile -NonInteractive -Command "(Get-AuthenticodeSignature '<installerPath>').Status"` and assert the trimmed output equals `Valid`
- [ ] 11.8 On any non-`Valid` result, return a wrapped error that includes the reported status string (e.g. `NotSigned`, `HashMismatch`, `UnknownError`)
- [ ] 11.9 Emit `logger.Debug("windows authenticode check", "status", status, "path", installerPath)` after the PowerShell invocation
- [ ] 11.10 On success, emit `logger.Verbose("installer signature verified")` (same milestone line as the Linux path)
- [ ] 11.11 Unit tests: mock `powershell.exe` via a fake script in a temp `$PATH` dir (`t.Setenv("PATH", ...)`) — `Valid` output → nil error; `HashMismatch` output → non-nil error with status in message; `skip == true` → nil without invoking PowerShell; `env.OS != "windows"` → skips to Linux branch

### Part C — Windows privilege check

- [ ] 11.12 Implement `isAdminWindows() bool` in `pkg/installer/preflight_windows.go` (build tag `//go:build windows`) using `golang.org/x/sys/windows` to check process token SID membership in the local Administrators group
- [ ] 11.13 Wire `isAdminWindows` into `CheckPrivilege()` (Task 2.7b) for the `runtime.GOOS == "windows"` branch; the Unix branch stays unchanged
- [ ] 11.14 Inject the check via a package-level `var isAdmin = isAdminWindows` variable so tests can replace it without requiring elevated privileges at test time
- [ ] 11.15 Unit test: set `isAdmin = func() bool { return false }` → `CheckPrivilege` returns the administrator-required error; `isAdmin = func() bool { return true }` → returns nil

### Part D — Windows integration test

- [ ] 11.16 Add `TestInstallOneAgentV2_HappyPath_Windows` to `oneagent_v2_test.go`: mock tenant API, mock download returns a `.exe` body, Authenticode check mocked as `Valid`, install command starts with the installer path then `--quiet` as first flag (no `/bin/sh` prefix, no `sudo`), executor returns exit code 0
- [ ] 11.17 Assert no `chmod`-equivalent is called on the downloaded installer path in the Windows happy-path flow
- [ ] 11.18 Assert the download URL contains the Windows path segment and the temp file name ends in `.exe`
