# Tasks

## 1. Thread scan roots through the scanner

- [x] 1.1 Change `scanProjectDirs` (`pkg/installer/otel/runtime_scan.go`) to accept `roots []string`; iterate the supplied roots via `walkCandidateDirs` instead of calling `os.Getwd()` internally, keeping the always-on `~/.dtwiz/examples/` walk
- [x] 1.2 Base the progress notice and relative-path/subtree bookkeeping on the first root (the working directory) instead of the removed `workingDir` local
- [x] 1.3 Update per-runtime detectors to accept and forward `roots`: `detectPythonProjects`, `detectJavaProjects` + `scanJavaProjects`, `detectNodeProjects`, `detectGoProjects`
- [x] 1.4 Add `roots` to the `runtimeInfo.detect` signature and to `detectAllProjects(runtimes, roots)`; pass `roots` from each parallel `rt.detect(roots)` call

## 2. Root selection and prompt

- [x] 2.1 Add a helper (e.g. `selectScanRoots`) that computes `cwd = os.Getwd()` and `home = os.UserHomeDir()`, using cleaned, symlink-resolved absolute paths and segment-wise ancestor/descendant comparison
- [x] 2.2 Skip the prompt and return `[cwd]` in all same-lineage cases: `cwd == home`, `home` is a descendant of `cwd`, or `cwd` is a descendant of `home` (within the home tree). Only prompt when `cwd` and `home` are disjoint trees
- [x] 2.3 Otherwise (disjoint) present the three-way `Y/c/n` prompt (default `Y`): `Y` → `[cwd, home]`, `c` → `[cwd]`, `n` → signal abort
- [x] 2.4 In non-interactive contexts (`installer.AutoConfirm`, or stdin is not a TTY) skip the prompt and default to `[cwd, home]`
- [x] 2.5 Map the `n` outcome to `installer.ErrInstallCancelled` returned from the caller

## 3. Wire into the install flow

- [x] 3.1 In `InstallOtelCollectorWithProject` (`pkg/installer/otel/otel.go`), inside the `projectPath == ""` branch and before `detectAllProjects`, call the root-selection helper and return `installer.ErrInstallCancelled` on `n`
- [x] 3.2 Pass the selected `roots` into `detectAllProjects(runtimes, roots)`
- [x] 3.3 Confirm the explicit `--project` path branch neither prompts nor scans (no change needed beyond not invoking the helper there)

## 4. Tests

- [x] 4.1 Update existing `TestScanProjectDirs_*` (`runtime_scan_test.go`) to pass explicit `roots`
- [x] 4.2 Add tests for the decision logic: skip (no prompt) when `cwd == home`, when `home` is a descendant of `cwd`, and when `cwd` is a descendant of `home`; prompt only when `cwd` and `home` are disjoint
- [x] 4.3 Add tests for the three outcomes (`Y`/`c`/`n`) and the non-interactive default (`AutoConfirm`/non-TTY, disjoint cwd → `[cwd, home]`)
- [x] 4.4 Add a test that scanning `[cwd, home]` dedups a project reachable via both the home root and the always-on `~/.dtwiz/examples/` root (or via a symlink), since the two primary roots are disjoint

## 5. Verify

- [x] 5.1 `make test` passes
- [x] 5.2 `make lint` shows no new issues
- [x] 5.3 Manual smoke: run `dtwiz install otel --dry-run` from a directory outside home like `/tmp/project` (disjoint → prompt appears), from `~/some/project` (within home → no prompt), from `$HOME` (CWD == home → no prompt), and with `--project <path>` (no scan, no prompt)
- [x] 5.4 Update `CHANGELOG.md` `[Unreleased]` with the new home-directory scan choice
