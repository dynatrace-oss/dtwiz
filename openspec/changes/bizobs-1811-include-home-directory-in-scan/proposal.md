# Proposal

## Why

When a user runs `dtwiz install otel` from a working directory that does not contain their code, the scanner finds nothing and the flow dead-ends, even though the user's projects usually live somewhere under their home directory. Today the scanner is deliberately limited to the working directory and never traverses ancestors, so there is no way to widen the search without `cd`-ing elsewhere and re-running. We want to offer that wider search interactively, without silently walking the user's entire home directory when they did not ask for it.

## What Changes

- When `dtwiz install otel` runs **without** an explicit `--project` path, and the working directory lies **outside the home-directory tree** (neither contains home nor is contained by it), prompt the user once (before the parallel runtime scan begins) with a three-way choice:
  - `Y` (default): scan the working directory **and** the home directory
  - `c`: scan the working directory only
  - `n`: abort the whole `install otel` command
- The prompt is skipped (no change from today, working-directory scan only) whenever the working directory is in the same lineage as home: `cwd == home`, or `home` is a descendant of `cwd` (the working-directory walk already covers home), or `cwd` is within home (`cwd` is a descendant of home). In the last case we do **not** add home as a second root.
- In non-interactive contexts (`--yes`/`AutoConfirm`, or stdin is not a TTY) the default `Y` is assumed when the working directory is outside the home tree: scan working directory + home.
- The scanner accepts an explicit set of scan roots instead of hard-coding the working directory. Roots are chosen once, before the parallel fan-out, and threaded down into every per-runtime scan.
- The `~/.dtwiz/examples/` bundled-examples scan stays always-on and unchanged, independent of the prompt outcome.
- Because home is only added when it is disjoint from the working directory, the scanner still never traverses an ancestor of the working directory; it may scan an additional, separate root (home). No change to the explicit-`--project` path (no scan, no prompt).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `otel-project-scan`: The "scan scope limited to working directory" requirement is extended. The scanner now selects scan roots via an interactive three-way prompt (working directory only, working directory + home, or abort) when run from a directory outside the home tree. Non-interactive default is working directory + home. The bundled-examples requirement is unchanged.

## Impact

- **Code:** `pkg/installer/otel/otel.go` (`InstallOtelCollectorWithProject` gains the root-selection prompt before `detectAllProjects`; `detectAllProjects` and `runtimeInfo.detect` gain a `roots []string` parameter), `pkg/installer/otel/runtime_scan.go` (`scanProjectDirs` accepts and iterates `roots` instead of calling `os.Getwd()` internally), and the per-runtime detectors (`detectPythonProjects`, `detectJavaProjects`/`scanJavaProjects`, `detectNodeProjects`, `detectGoProjects`) which thread `roots` through.
- **UX:** one new interactive prompt in the otel install flow; abort reuses the existing `installer.ErrInstallCancelled` cancellation path.
- **Tests:** `runtime_scan_test.go` scan helpers now pass explicit roots; new coverage for the prompt decision logic (prompt-vs-skip conditions, three-way outcomes, non-interactive default).
- **Overlap handling:** home is only added when disjoint from the working directory, so the two roots never overlap. The only residual overlap is the always-on `~/.dtwiz/examples/` root (which lives under home); the existing `matchedProjects` dedup already prevents duplicate projects there.
- No dependency, API, or auth changes. No feature flag.
