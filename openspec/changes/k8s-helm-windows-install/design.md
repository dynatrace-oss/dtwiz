## Context

`installHelm()` in `pkg/installer/kubernetes.go` unconditionally runs `bash -c "curl ... | bash"` when Helm is not found. On Windows, `bash` is not on PATH, so users running `dtwiz install kubernetes` get `exec: "bash": executable file not found in %PATH%` and cannot proceed.

Windows is a supported target (goreleaser produces `.zip` archives per OS/arch), so the Windows code path must be functional.

## Goals / Non-Goals

**Goals:**
- On Windows with winget available: silently install Helm via `winget install --id Helm.Helm -e --source winget`
- On Windows without winget (or winget failure): return a clear, actionable error with manual install steps and a link
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

## Risks / Trade-offs

- **winget not present on Windows Server or older Windows 10 builds** → Mitigation: fallback error message is shown immediately with manual steps; no silent failure.
- **winget present but restricted by policy** → Mitigation: `RunCommand` captures and surfaces the winget error; user sees both the winget failure and the manual instructions in the returned error.

## Open Questions

None.
