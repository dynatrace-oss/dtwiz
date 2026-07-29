# Proposal

## Why

When a user runs `dtwiz install otel` from a working directory that does not contain their code, the scanner finds nothing and the flow dead-ends, even though the user's projects usually live somewhere under their home directory. Today the scanner is deliberately limited to the working directory and never traverses ancestors, so there is no way to widen the search without `cd`-ing elsewhere and re-running. We want to offer that wider search interactively, without silently walking the user's entire home directory when they did not ask for it.

## What Changes

- When `dtwiz install otel` runs **without** an explicit `--project` path, and the working directory does not already cover the home directory, prompt the user once (before the parallel runtime scan begins) with a three-way choice:
  - `Y` (default): scan the working directory **and** the home directory
  - `c`: scan the working directory only
  - `n`: abort the whole `install otel` command
- The prompt is skipped (no change from today, working-directory scan only) when `cwd == home` or when `home` is a descendant of `cwd`, because the working-directory walk already covers home in those cases.
- In non-interactive contexts (`--yes`/`AutoConfirm`, or stdin is not a TTY) the default `Y` is assumed: scan working directory + home.
- The scanner accepts an explicit set of scan roots instead of hard-coding the working directory. Roots are chosen once, before the parallel fan-out, and threaded down into every per-runtime scan.
- The `~/.dtwiz/examples/` bundled-examples scan stays always-on and unchanged, independent of the prompt outcome.
- **BREAKING** (spec-level only): the scanner may now traverse an ancestor of the working directory (the home directory) when the user opts in. This relaxes the current "SHALL NOT traverse any ancestor directories" guarantee.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `otel-project-scan`: The "scan scope limited to working directory" requirement is relaxed. The scanner now selects scan roots via an interactive three-way prompt (working directory only, working directory + home, or abort) when run from a directory that does not already cover home. Non-interactive default is working directory + home. The bundled-examples requirement is unchanged.

## Impact

- **Code:** `pkg/installer/otel/otel.go` (`InstallOtelCollectorWithProject` gains the root-selection prompt before `detectAllProjects`; `detectAllProjects` and `runtimeInfo.detect` gain a `roots []string` parameter), `pkg/installer/otel/runtime_scan.go` (`scanProjectDirs` accepts and iterates `roots` instead of calling `os.Getwd()` internally), and the per-runtime detectors (`detectPythonProjects`, `detectJavaProjects`/`scanJavaProjects`, `detectNodeProjects`, `detectGoProjects`) which thread `roots` through.
- **UX:** one new interactive prompt in the otel install flow; abort reuses the existing `installer.ErrInstallCancelled` cancellation path.
- **Tests:** `runtime_scan_test.go` scan helpers now pass explicit roots; new coverage for the prompt decision logic (prompt-vs-skip conditions, three-way outcomes, non-interactive default).
- **Accepted trade-off:** in the common case where `cwd` is under `home`, choosing `Y` re-walks the `cwd` subtree while walking `home`; the existing `matchedProjects` dedup prevents duplicate projects, and the redundant walk is accepted (not optimized in this change).
- No dependency, API, or auth changes. No feature flag.
