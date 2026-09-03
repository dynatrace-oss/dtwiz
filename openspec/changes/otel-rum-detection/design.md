# Design

## Context

The OTel installer (`pkg/installer/otel/`) handles instrumentation of server-side runtimes. RUM onboarding (PRODUCT-18188) extends this flow to also set up frontend monitoring. Detection is the first of three phases: detect, create frontend entity and fetch JS tag, inject/print. This change covers detection and its preview integration.

The detector runs synchronously during `dtwiz install otel`, before any Dynatrace API calls or file writes. Its output drives whether the preview shows "will auto-inject into N file(s)" or "will print tag for manual addition". The preview section is gated behind `--experimental` / `DTWIZ_EXPERIMENTAL` until the full injection flow is complete.

## Goals / Non-Goals

**Goals:**

- Classify a project directory as auto-injectable or requiring manual setup.
- Enumerate candidate `.html` files for injection (those in the project root, not inside build output dirs).
- Detect frameworks and build tools that prevent direct HTML modification.
- Probe write permission for each candidate file in a cross-platform-safe way.
- Produce a single result struct that downstream steps consume without re-scanning.
- Show a RUM preview section in `dtwiz install otel` when `--experimental` is active.

**Non-Goals:**

- Inject the JavaScript tag (follow-on change).
- Create the Dynatrace frontend entity or fetch the JS tag (follow-on change).
- Detect mobile or non-web runtimes.
- Detect frameworks beyond those that OTel already instruments. Future framework support is a future change.

## Package Location

`pkg/installer/otel/rum/` — new sub-package of the OTel method folder. Per the folder-per-method rule, RUM logic is OTel-specific and must not land in the shared `pkg/installer/` root.

## Result Struct

```go
type InjectionMode string

const (
    ModeAuto   InjectionMode = "auto"
    ModeManual InjectionMode = "manual"
)

type DetectionResult struct {
    Mode            InjectionMode
    InjectableFiles []string // absolute paths; populated when Mode == ModeAuto
    ManualReason    string   // human-readable; populated when Mode == ModeManual
}
```

Consumers (the OTel installer preview step) inspect `Mode` to branch the UX; they iterate `InjectableFiles` for the preview list; they display `ManualReason` when manual setup is required.

## Detection Algorithm

### Step 1 — Framework detection

Detect frameworks that the OTel installer already recognizes, ensuring consistency between backend instrumentation and RUM setup. The detection logic mirrors `pkg/installer/otel/nodejs_project.go` but is implemented directly in `rum/detect.go` to avoid an import cycle (see Decisions).

Check for the following frameworks in order (first match wins):

| Framework | Detection method |
|---|---|
| Next.js | Config files: `next.config.js`, `next.config.mjs`, `next.config.ts`; or dependency: `"next"` in `package.json` |
| Nuxt | Config files: `nuxt.config.js`, `nuxt.config.mjs`, `nuxt.config.ts`; or dependency: `"nuxt"` in `package.json` |

For each framework, check config files first. If no config file is found, read and JSON-parse `package.json` and check `dependencies` and `devDependencies`. If `package.json` cannot be read or parsed, treat it as "framework not found" and continue to step 2.

When a framework is detected, set `Mode = ModeManual` and populate `ManualReason` with the framework name.

**Rationale:** Only detect frameworks that have no injectable HTML source file. Next.js and Nuxt render pages from JSX/Vue components. There is no `index.html` to inject into. Projects like Create React App or Angular that do have a source `index.html` are handled correctly by the `index.html` preference rule in step 4.

### Step 2 — HTML file scan

Walk the project directory with `fs.WalkDir`. For each entry:

- Skip the following directories entirely (do not recurse into them):
  - `node_modules`
  - `.git`
  - `.next`, `.nuxt`, `.svelte-kit`
  - `dist`, `build`, `out`, `.output`
  - `__pycache__`, `.venv`, `venv`
- Collect files with a `.html` extension (case-insensitive on Windows; case-sensitive on Unix — use `strings.EqualFold` only if `runtime.GOOS == "windows"`).

### Step 3 — Write permission probe

For each collected `.html` file, probe writability by attempting `os.OpenFile(path, os.O_WRONLY, 0)`. If the call fails, exclude the file and record it as a permission error. Don't use `os.Stat` because it works on all supported operating systems.

### Step 4 — Mode assignment

From the writable `.html` files collected in step 3:

- If any file is named `index.html`, keep only those files (prefer the app entry point over partial templates).
- Otherwise, keep all collected files.
- If no files remain: `Mode = ModeManual`, `ManualReason = "no writable HTML files found"`.
- Otherwise: `Mode = ModeAuto`, `InjectableFiles` = sorted list of the selected paths.

## OTel Installer Integration

Detection is gated behind `featureflags.Experimental` in `InstallOtelCollectorWithProject`. When the flag is active:

1. `rum.Detect` is called on `projectPath` (if provided via `--project`) or `os.Getwd()`.
2. The result is stored and displayed as a numbered preview section after the runtime auto-instrumentation block.
3. No injection happens; the section is informational only.

Preview output when auto-inject is possible:

```
  N) Real User Monitoring [experimental]
     Mode:  auto-inject
     File:  /path/to/public/index.html
```

Preview output when manual setup is required:

```
  N) Real User Monitoring [experimental]
     Mode:  manual (Next.js)
```

Failures during detection are logged at debug level and do not abort the install.

## Decisions

- **Self-contained framework detection.** The `rum` package implements `isNextJSProject`, `isNuxtProject`, and `hasDependency` directly, mirroring the logic in `pkg/installer/otel/nodejs_project.go`. Importing that package would create a circular dependency (`otel` → `rum` → `otel`). The functions are small enough that the duplication cost is lower than the architectural cost of introducing an intermediate shared package.
- **`IsNextJSProject` and `IsNuxtProject` exported from `otel`.** Even though `rum` does not import them, these are exported so other packages (e.g., future CLI commands) can reuse them without going through the `rum` package.
- **Detect framework before walking for HTML.** Framework detection is O(1) stat calls; HTML scanning is O(N) on the directory tree. Stopping early avoids unnecessary I/O in the common case where a framework is present.
- **`fs.WalkDir` over `filepath.Walk`.** `fs.WalkDir` passes `fs.DirEntry` which avoids an extra `os.Stat` syscall per entry. Both are cross-platform.
- **No `os.Stat` mode-bit checks for writability.** `os.Stat` returns Unix-style mode bits that are not meaningful on Windows (all files appear writable). `os.OpenFile` with `O_WRONLY` is portable.
- **Case-insensitive `.html` only on Windows.** Unix filesystems are case-sensitive; treating `.HTML` as `.html` on Unix would be incorrect for most projects.
- **Prefer `index.html` over other `.html` files.** Frameworks like Angular have many `.html` component templates alongside `src/index.html`. Targeting only `index.html` when present avoids injecting into partial templates, which would corrupt them. For projects with no `index.html` (multi-page static sites), all writable `.html` files are candidates.
- **Sorted `InjectableFiles`.** Deterministic ordering simplifies tests and makes the preview stable across runs.
- **Experimental gate for the preview.** The full RUM flow (entity creation, tag fetch, injection) is not implemented yet. Showing the detection result only under `--experimental` signals to users that this is in-progress and prevents the section from appearing in production installs until the feature is complete.

## Risks / Trade-offs

- False negatives: a project may use a framework not in the detection list. The user receives auto-injection when it would fail. Mitigation: the detection list is the primary extension point; it can grow without changing the algorithm.
- False positives: a project may have a config filename collision (e.g., a non-Vite `vite.config.js`). This is accepted — the cost of a false positive (user gets a manual instruction) is lower than the cost of injecting into a framework-managed file.
- Write permission probe opens each file briefly. This is negligible for the expected number of HTML files in a project root.
- Detection uses CWD when no `--project` path is given. For monorepos, this may scan a wider tree than intended. The follow-on injection change should refine the scan root.
