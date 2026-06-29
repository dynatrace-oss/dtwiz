# Design

## Context

`installHelm()` in `pkg/installer/kubernetes.go` unconditionally runs `bash -c "curl ... | bash"` when Helm is not found. On Windows, `bash` is not on PATH, so users running `dtwiz install kubernetes` get `exec: "bash": executable file not found in %PATH%` and cannot proceed.

A second problem: after winget installs Helm, the binary lands in a directory that winget adds to the Windows registry user PATH — but the current process inherited its PATH at startup and does not see the update. This breaks both the rest of the install run and any subsequent `dtwiz uninstall kubernetes` invocation, which is a separate process with the original stale PATH.

Windows is a supported target (goreleaser produces `.zip` archives per OS/arch), so both code paths must be functional.

## Goals / Non-Goals

**Goals:**

- On Windows with winget available: install Helm via `winget install --id Helm.Helm -e --source winget`
- On Windows without winget (or winget failure): return a clear, actionable error with manual install steps and a link
- After winget install: make `helm` immediately usable in the current process without a shell restart
- On any kubernetes command startup on Windows: refresh the process PATH so a previously winget-installed helm is always found
- On Unix: preserve existing `bash -c curl | bash` behaviour exactly

**Non-Goals:**

- Supporting choco or scoop (too many package managers to maintain; winget ships with Windows 10/11 by default)
- Installing winget itself
- Changing the Unix flow in any way

## Decisions

### Platform dispatch via `runtime.GOOS` inside `installHelm()`

Keep `installHelm()` as a single function that branches on `runtime.GOOS == "windows"`. An alternative is splitting into `kubernetes_unix.go` + `kubernetes_windows.go` build-tag files. The split is heavier machinery than a single branch warrants — the rest of `kubernetes.go` is platform-agnostic and splitting would duplicate or scatter the function's callers. The branch is the right size here.

### winget only (no choco/scoop fallback chain)

Trying multiple package managers adds complexity and noise. winget ships with Windows 10 (2020+) and Windows 11 by default and is the officially endorsed Microsoft tool. Users on older or restricted machines should follow the manual path; a wall of attempted package managers obscures that path.

### Error message includes winget command + download URL

When winget is absent or fails, the error must be self-contained: the user should be able to act without leaving the terminal or searching the web. The message includes `winget install --id Helm.Helm` and `https://helm.sh/docs/intro/install/`.

### `refreshWindowsPath()` reads user PATH only and appends

winget always writes to the Windows registry user PATH (`HKCU\Environment`), not the machine PATH. Reading only user PATH via `[Environment]::GetEnvironmentVariable('Path','User')` targets exactly what winget changed. New entries are appended to — not replace — the current process PATH, preserving anything the shell session added (virtual envs, nvm, etc.). Duplicates are harmless.

### OS check inside `refreshWindowsPath()`, called at startup of both kubernetes commands

The check `runtime.GOOS != "windows"` lives inside `refreshWindowsPath()` so it is a safe no-op on Unix with no call-site guards needed. Calling it at the start of both `InstallKubernetes` and `UninstallKubernetes` (not just after winget) ensures helm is findable regardless of which command runs first and whether helm was installed in a prior dtwiz session.

## Risks / Trade-offs

- **winget not present on Windows Server or older Windows 10 builds** → Mitigation: fallback error message is shown immediately with manual steps; no silent failure.
- **winget present but restricted by policy** → Mitigation: `RunCommand` captures and surfaces the winget error; user sees both the winget failure and the manual instructions in the returned error.
- **PATH entries duplicated after refresh** → No functional impact; duplicate PATH entries are ignored by the OS.
- **PowerShell not available** → `refreshWindowsPath()` returns an error printed as a warning; the command continues and may still find helm if PATH was already correct.

## Open Questions

None.
