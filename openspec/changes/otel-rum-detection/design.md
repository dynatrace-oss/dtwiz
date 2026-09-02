# Design

## Context

The OTel installer (`pkg/installer/otel/`) handles instrumentation of server-side runtimes. RUM onboarding (PRODUCT-18188) extends this flow to also set up frontend monitoring. Detection is the first of three phases: detect, create frontend entity and fetch JS tag, inject/print. This change covers detection only.

The detector runs synchronously during `dtwiz install otel`, before any Dynatrace API calls or file writes. Its output drives whether the preview shows "will auto-inject into N file(s)" or "will print tag for manual addition".

## Goals / Non-Goals

**Goals:**

- Classify a project directory as auto-injectable or requiring manual setup.
- Enumerate candidate `.html` files for injection (those in the project root, not inside build output dirs).
- Detect frameworks and build tools that prevent direct HTML modification.
- Probe write permission for each candidate file in a cross-platform-safe way.
- Produce a single result struct that downstream steps consume without re-scanning.

**Non-Goals:**

- Inject the JavaScript tag (follow-on change).
- Create the Dynatrace frontend entity or fetch the JS tag (follow-on change).
- Display anything to the user (follow-on change).
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
    Mode           InjectionMode
    InjectableFiles []string // absolute paths; populated when Mode == ModeAuto
    ManualReason   string   // human-readable; populated when Mode == ModeManual
}
```

Consumers (the OTel installer preview step) inspect `Mode` to branch the UX; they iterate `InjectableFiles` for the preview list; they display `ManualReason` when manual setup is required.

## Detection Algorithm

### Step 1 — Framework detection

Detect frameworks that the OTel installer already recognizes, ensuring consistency between backend instrumentation and RUM setup. Use the existing functions from `pkg/installer/otel/nodejs_project.go` when possible; otherwise follow the same pattern.

Check for the following frameworks in order (first match wins):

| Framework | Detection method |
|---|---|
| Next.js | Config files: `next.config.js`, `next.config.mjs`, `next.config.ts`; or dependency: `"next"` in `package.json` |
| Nuxt | Config files: `nuxt.config.js`, `nuxt.config.mjs`, `nuxt.config.ts`; or dependency: `"nuxt"` in `package.json` |
| Create React App | Dependency: `"react-scripts"` in `package.json` |

For each framework, check config files first. If no config file is found, read and JSON-parse `package.json` and check `dependencies` and `devDependencies`. If `package.json` cannot be read or parsed, treat it as "framework not found" and continue to step 2.

When a framework is detected, set `Mode = ModeManual` and populate `ManualReason` with the framework name.

**Rationale:** Only detect frameworks that OTel already instruments, ensuring consistency. This avoids false positives where RUM instructs manual setup for a framework that OTel doesn't support, creating confusion. Reuse the existing detection logic from `nodejs_project.go` where applicable.

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

For each collected `.html` file, probe writability by attempting `os.OpenFile(path, os.O_WRONLY, 0)`. If the call fails, exclude the file and record it as a permission error. Do not use Unix mode bits — they are unreliable on Windows.

### Step 4 — Mode assignment

- If no writable `.html` files remain after step 3: `Mode = ModeManual`, `ManualReason = "no writable HTML files found"`.
- Otherwise: `Mode = ModeAuto`, `InjectableFiles` = sorted list of writable `.html` absolute paths.

## Decisions

- **Reuse existing OTel detection functions.** Import `isNextJSProject`, `isNuxtProject`, and `hasDependency` from `pkg/installer/otel/nodejs_project.go` to maintain consistency and avoid code duplication. RUM detection and OTel instrumentation use the same framework classification.
- **Detect framework before walking for HTML.** Framework detection is O(1) stat calls; HTML scanning is O(N) on the directory tree. Stopping early avoids unnecessary I/O in the common case where a framework is present.
- **`fs.WalkDir` over `filepath.Walk`.** `fs.WalkDir` passes `fs.DirEntry` which avoids an extra `os.Stat` syscall per entry. Both are cross-platform.
- **No `os.Stat` mode-bit checks for writability.** `os.Stat` returns Unix-style mode bits that are not meaningful on Windows (all files appear writable). `os.OpenFile` with `O_WRONLY` is portable.
- **Case-insensitive `.html` only on Windows.** Unix filesystems are case-sensitive; treating `.HTML` as `.html` on Unix would be incorrect for most projects.
- **Sorted `InjectableFiles`.** Deterministic ordering simplifies tests and makes the preview stable across runs.

## Risks / Trade-offs

- False negatives: a project may use a framework not in the detection list. The user receives auto-injection when it would fail. Mitigation: the detection list is the primary extension point; it can grow without changing the algorithm.
- False positives: a project may have a config filename collision (e.g., a non-Vite `vite.config.js`). This is accepted — the cost of a false positive (user gets a manual instruction) is lower than the cost of injecting into a framework-managed file.
- Write permission probe opens each file briefly. This is negligible for the expected number of HTML files in a project root.
