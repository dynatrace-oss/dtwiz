# Tasks

## 1. Thread scan roots through the scanner

- [ ] 1.1 Change `scanProjectDirs` (`pkg/installer/otel/runtime_scan.go`) to accept `roots []string`; iterate the supplied roots via `walkCandidateDirs` instead of calling `os.Getwd()` internally, keeping the always-on `~/.dtwiz/examples/` walk
- [ ] 1.2 Base the progress notice and relative-path/subtree bookkeeping on the first root (the working directory) instead of the removed `workingDir` local
- [ ] 1.3 Update per-runtime detectors to accept and forward `roots`: `detectPythonProjects`, `detectJavaProjects` + `scanJavaProjects`, `detectNodeProjects`, `detectGoProjects`
- [ ] 1.4 Add `roots` to the `runtimeInfo.detect` signature and to `detectAllProjects(runtimes, roots)`; pass `roots` from each parallel `rt.detect(roots)` call

## 2. Root selection and prompt

- [ ] 2.1 Add a helper (e.g. `selectScanRoots`) that computes `cwd = os.Getwd()` and `home = os.UserHomeDir()`, using cleaned, symlink-resolved absolute paths and segment-wise ancestor comparison
- [ ] 2.2 Skip the prompt and return `[cwd]` when `cwd == home` or when `home` is a descendant of `cwd`
- [ ] 2.3 Otherwise present the three-way `Y/c/n` prompt (default `Y`): `Y` → `[cwd, home]`, `c` → `[cwd]`, `n` → signal abort
- [ ] 2.4 In non-interactive contexts (`installer.AutoConfirm`, or stdin is not a TTY) skip the prompt and default to `[cwd, home]`
- [ ] 2.5 Map the `n` outcome to `installer.ErrInstallCancelled` returned from the caller

## 3. Wire into the install flow

- [ ] 3.1 In `InstallOtelCollectorWithProject` (`pkg/installer/otel/otel.go`), inside the `projectPath == ""` branch and before `detectAllProjects`, call the root-selection helper and return `installer.ErrInstallCancelled` on `n`
- [ ] 3.2 Pass the selected `roots` into `detectAllProjects(runtimes, roots)`
- [ ] 3.3 Confirm the explicit `--project` path branch neither prompts nor scans (no change needed beyond not invoking the helper there)

## 4. Tests

- [ ] 4.1 Update existing `TestScanProjectDirs_*` (`runtime_scan_test.go`) to pass explicit `roots`
- [ ] 4.2 Add tests for the decision logic: prompt-vs-skip when `cwd == home`, when `home` is a descendant of `cwd`, and when it is not
- [ ] 4.3 Add tests for the three outcomes (`Y`/`c`/`n`) and the non-interactive default (`AutoConfirm`/non-TTY → `[cwd, home]`)
- [ ] 4.4 Add a test that scanning multiple roots dedups a project reachable from both `cwd` and `home` (common dev case)

## 5. Verify

- [ ] 5.1 `make test` passes
- [ ] 5.2 `make lint` shows no new issues
- [ ] 5.3 Manual smoke: run `dtwiz install otel --dry-run` from `~/some/project` (CWD under home → prompt appears), from `$HOME` (CWD == home → no prompt), and with `--project <path>` (no scan, no prompt)
- [ ] 5.4 Update `CHANGELOG.md` `[Unreleased]` with the new home-directory scan choice
