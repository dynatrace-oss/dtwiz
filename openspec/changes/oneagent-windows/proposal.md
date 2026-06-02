# Why

Tasks 2–7 of the OneAgent PoC implement the core install flow with Linux as the primary platform. Windows requires platform-specific handling at every stage: different installer URL path, `.exe` temp file extension, no `chmod`, Authenticode signature verification instead of OpenSSL CMS, UAC rather than `sudo`, and an admin-group privilege check using the Windows process token API. This change consolidates all Windows-specific work that was left as stubs or TODOs in earlier tasks.

## What Changes

- **Download**: `DownloadInstaller` maps `env.OS == "windows"` to the `/windows/default/latest?arch=x86` API path; saves the temp file with `.exe` extension; skips `os.Chmod` (NTFS ACLs are managed by the OS).
- **Signature verification**: `VerifyInstallerSignature` on Windows invokes `powershell.exe -NoProfile -NonInteractive -Command "(Get-AuthenticodeSignature '<path>').Status"` and asserts the result is `Valid`. Missing `powershell.exe` is a hard error unless `--no-verify-signature` is passed.
- **Privilege check**: `isAdminWindows()` in `pkg/installer/preflight_windows.go` checks the process token for BUILTIN\Administrators group membership via `golang.org/x/sys/windows`. Injected via a package-level `var isAdmin` for test mockability.
- **Command construction**: Windows argv starts with the `.exe` path directly (no `/bin/sh` wrapper, no `sudo`); `--quiet` is the first flag when present.

## Capabilities

### New Capabilities

- Windows Authenticode signature verification via PowerShell (parallel to Linux OpenSSL path).
- Windows admin-group privilege check via process token SID lookup.

### Modified Capabilities

- `oneagent-installer-download`: Windows download URL, `.exe` extension, no `chmod`.
- `oneagent-install-execution`: Windows argv construction — installer path as first element, `--quiet` before config flags, no `sudo`.

## Impact

- **New files:** `pkg/installer/preflight_windows.go` (`//go:build windows`), corresponding `preflight_unix.go` stub if needed.
- **Modified files:** `pkg/installer/oneagent/` (extend download in `download.go`, verify in `verify.go`, build paths in `oneagent.go`), `pkg/installer/oneagent/oneagent_test.go` (extend with Windows happy-path and PowerShell mock tests).
- **New dependency:** `golang.org/x/sys/windows` for process token / SID inspection (may already be present transitively).
