# Design

## Context

`scanProjectDirs` (`pkg/installer/otel/runtime_scan.go`) is the shared filesystem scanner for the OTel install flow. Today it hard-codes its roots: it calls `os.Getwd()` and walks the working directory downward, plus always walks `~/.dtwiz/examples/`. It deliberately never traverses ancestors of the working directory.

The scanner is invoked once per enabled runtime (Python, Java, Node.js, Go). `detectAllProjects` (`otel.go:90`) fans those runtimes out into **parallel goroutines**, so `scanProjectDirs` runs up to four times concurrently. Any user prompt placed inside `scanProjectDirs` would therefore race across goroutines for stdin/stdout. This is the central constraint the design works around.

The install entry point is `InstallOtelCollectorWithProject` (`otel.go:253`). When called with an explicit `--project` path it builds a plan directly and never scans. Otherwise it calls `detectAllProjects` (`otel.go:313`) and already has an established cancellation path via `installer.ErrInstallCancelled` (`otel.go:321`).

## Goals / Non-Goals

**Goals:**

- Let a user running `install otel` from a directory that does not cover home opt into also scanning their home directory, via a single interactive prompt.
- Make the prompt fire exactly once per invocation, before any parallel scanning.
- Keep the working-directory-only behavior available (`c`) and give a clean full-command abort (`n`).
- Preserve the always-on `~/.dtwiz/examples/` scan unchanged.
- Choose scan roots explicitly and pass them down, rather than deriving them deep inside the scanner.

**Non-Goals:**

- Optimizing the redundant walk when `cwd` is under `home` and the user picks `Y` (the `home` walk re-descends into the `cwd` subtree). Dedup already prevents duplicate results; the extra walk is accepted.
- Adding a persistent flag or env var to preselect the choice. Only the interactive prompt plus the existing `AutoConfirm`/TTY signals drive the default.
- Changing scan behavior for the explicit `--project` path (no scan, no prompt).
- Changing uninstall-side scanning.

## Decisions

### Decision: Select scan roots once, above the parallel fan-out

Root selection (and the prompt) lives in `InstallOtelCollectorWithProject`, in the `else` branch (`projectPath == ""`), immediately before `detectAllProjects`. This guarantees a single prompt and lets the `n` case `return installer.ErrInstallCancelled` before any work begins, reusing the existing cancellation pattern.

**Alternative considered:** prompt inside `scanProjectDirs`. Rejected: it runs up to 4× concurrently, so the prompt would race and repeat.

### Decision: Explicit threading of `roots []string`

The chosen roots are passed as a parameter down the call chain rather than stored in package-level state:

```text
InstallOtelCollectorWithProject
  → roots := selectScanRoots(...)          // prompt / decision, once
  → detectAllProjects(runtimes, roots)
      → rt.detect(roots)                    // runtimeInfo.detect signature grows
          → detect{Python,Java,Node,Go}Projects(roots)
              → scanJavaProjects(roots) (Java)
              → scanProjectDirs(markers, excludeNames, roots)
```

`scanProjectDirs` stops calling `os.Getwd()` itself; it iterates the supplied `roots`, walking each via `walkCandidateDirs`, and then appends the always-on `~/.dtwiz/examples/` walk exactly as today. Progress/relative-path bookkeeping that currently keys off `workingDir` keys off the first root (the working directory).

**Alternative considered:** a package-level `scanRoots` set before the fan-out (precedent: `AutoConfirm`, `globalScanCount`). Rejected: shared mutable state is easy to get wrong with concurrent scans and harder to test; the existing `TestScanProjectDirs_*` tests construct scans directly and benefit from an explicit parameter.

### Decision: When to prompt vs. skip

Compute `cwd = os.Getwd()` and `home = os.UserHomeDir()` once.

- `cwd == home` → **no prompt**, `roots = [cwd]` (home is already the working dir).
- `home` is a descendant of `cwd` (i.e. `cwd` is an ancestor of `home`) → **no prompt**, `roots = [cwd]` (the working-dir walk already covers home).
- otherwise (`cwd` under `home`, or an unrelated tree) → **prompt** `Y/c/n`.

Ancestor detection uses cleaned, symlink-resolved absolute paths and a path-segment comparison (not raw string prefix, to avoid `/home/foobar` matching `/home/foo`).

### Decision: Prompt outcomes and non-interactive default

- `Y` (default, Enter) → `roots = [cwd, home]`
- `c` → `roots = [cwd]`
- `n` → abort: `return installer.ErrInstallCancelled`

When `installer.AutoConfirm` is set, or stdin is not a TTY, the prompt is not shown and `Y` is assumed (`roots = [cwd, home]`). This matches the proposal's "zero-config, widen by default" intent while never blocking an unattended run.

### Decision: Bundled examples stay orthogonal

`~/.dtwiz/examples/` remains an unconditional extra walk inside `scanProjectDirs`, independent of `roots` and of the prompt outcome (including `c`). No behavior change; the existing dedup handles overlap when `home` is also a root.

## Risks / Trade-offs

- **Redundant walk when `cwd` is under `home` and user picks `Y`** → the `home` walk re-descends into the `cwd` subtree. Mitigation: existing `matchedProjects` dedup keeps results correct; extra walk time accepted as out of scope.
- **Scanning a large home directory is slow/noisy** → mitigation: the existing large-scan progress notice (`largeScanThreshold`) and the macOS/Windows system-dir excludes already apply to every root, including `home`.
- **Non-interactive default `Y` walks all of home in CI/pipelines** → mitigation: this only affects `install otel` without `--project`; automated flows that target a specific project should pass `--project`, which skips scanning entirely.
- **Relaxing the "never traverse ancestors" guarantee** → this is an intentional spec change (see spec delta); it only happens on explicit opt-in (or the documented non-interactive default), never silently mid-scan.
