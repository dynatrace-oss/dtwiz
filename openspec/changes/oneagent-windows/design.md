# Design

## Context

Tasks 2–7 implement the OneAgent PoC install flow with Linux as the primary target. Windows-specific branches were noted with stubs or cross-references to "Task 11". This change fills those stubs: correct installer URL/extension, Authenticode verification, admin privilege check, and Windows-specific command construction. The goal is Windows parity with the Linux flow — same stage sequence, platform-specific implementation at each branch point.

## Goals / Non-Goals

**Goals:**

- All four Windows-specific implementation areas: installer download, signature verification, privilege check, command construction.
- Unit tests that do not require Windows at test time (mock `powershell.exe` via a fake `$PATH` script; mock `isAdmin` via a package-level variable).

**Non-Goals:**

- UAC re-launch from a non-elevated process — the installer `.exe` triggers UAC natively. No dtwiz-level re-launch.
- macOS support — remains out of scope.

## Decisions

### 1. Installer download: URL path and temp file extension

Windows installer API path:

```bash
/api/v1/deployment/installer/agent/windows/default/latest?arch=x86
```

Temp file: `os.CreateTemp("", "dynatrace-oneagent-*.exe")` — the `.exe` suffix is required for Windows to recognize the file as executable. `os.Chmod` is guarded by `if runtime.GOOS != "windows"` (NTFS ACLs are handled by the OS).

### 2. Authenticode verification: PowerShell

On Windows with `skip == false`:

```powershell
powershell.exe -NoProfile -NonInteractive -Command "(Get-AuthenticodeSignature '<installerPath>').Status"
```

`Valid` output → `nil`. Any other result (e.g. `NotSigned`, `HashMismatch`, `UnknownError`) → wrapped error including the status string. Missing `powershell.exe` → hard error; `--no-verify-signature` is the only skip path.

Tests mock via a fake `powershell.exe` script in a temp dir prepended to `$PATH` (`t.Setenv("PATH", ...)`).

### 3. Privilege check: process token SID

`isAdminWindows()` in `pkg/installer/preflight_windows.go` (`//go:build windows`) uses `golang.org/x/sys/windows` to open the process token and check for BUILTIN\Administrators SID membership. Injected into `CheckPrivilege()` via a package-level variable:

```go
var isAdmin = isAdminWindows  // replaced in tests
```

This avoids requiring elevated privileges at test time. The corresponding Unix check continues to use `needsSudo()` semantics.

### 4. Command construction: no shell wrapper, --quiet first

Windows argv:

```bash
[<installerPath.exe>, --quiet?, --set-monitoring-mode=<mode>, --set-app-log-content-access=<bool>, --set-host-group=<group>?]
```

No `/bin/sh` prefix (the `.exe` is directly executable). No `sudo` prefix (UAC is handled by the installer). `--quiet` must appear before any `--set-*` flag when present.

### 5. Windows vs Unix file split

Platform-specific code goes in:

- `pkg/installer/preflight_windows.go` (`//go:build windows`) — `isAdminWindows`, Windows privilege check.
- `pkg/installer/preflight_unix.go` (`//go:build !windows`) — `needsSudo`-based check (already exists as `sudo_unix.go`; extend or add file as needed).

Shared code in `pkg/installer/oneagent/` calls `CheckPrivilege()` without a runtime `GOOS` check — the build tag dispatch handles it.

### 6. Logging

Windows-specific log lines follow the same keys as their Linux counterparts:

| Stage | Level | Message | Keys |
|---|---|---|---|
| PowerShell lookup | Debug | `"powershell lookup"` | `path`, `found` |
| Authenticode check | Debug | `"windows authenticode check"` | `status`, `path` |
| Verify success | Verbose | `"installer signature verified"` | — |
| Privilege check | Debug | `"privilege check"` | `privileged`, `os` |

The Verbose milestone `"installer signature verified"` is identical on Linux and Windows, so log consumers see a consistent event regardless of platform.
