# Tasks

## 1. Package Scaffold

- [x] 1.1 Create `pkg/installer/otel/internal/rum/` directory and `detect.go` file with the `rum` package declaration.
- [x] 1.2 Define `InjectionMode` string type and `ModeAuto` / `ModeManual` constants.
- [x] 1.3 Define `DetectionResult` struct with `Mode InjectionMode`, `InjectableFiles []string`, and `ManualReason string`.

## 2. Framework Detection

- [x] 2.1 Reuse `markers.IsNextJSProject`, `markers.IsNuxtProject`, and `markers.HasDependency` from `pkg/installer/otel/markers` in `detect.go`; do not re-implement them.
- [x] 2.2 Implement `detectFramework(dir string) (name string, found bool)` in `detect.go` that:
  - Returns `"Next.js"` if `markers.IsNextJSProject(dir)` is true
  - Returns `"Nuxt"` if `markers.IsNuxtProject(dir)` is true
  - Returns `""` and `false` if none are found
- [x] 2.3 Expose `detectFramework` for use by the main `Detect` entry point.

## 3. HTML File Walk

- [x] 3.1 Implement `walkHTML(dir string) ([]string, error)` using `fs.WalkDir` (via `os.DirFS`) to collect `.html` files.
- [x] 3.2 Skip excluded directories (`node_modules`, `.git`, `.next`, `.nuxt`, `.svelte-kit`, `dist`, `build`, `out`, `.output`, `__pycache__`, `.venv`, `venv`) by returning `fs.SkipDir` from the walk function.
- [x] 3.3 Skip hidden directories (names starting with `.`). The scan is unbounded in depth but never leaves the scan root.
- [x] 3.4 Apply case-insensitive `.html` matching only when `runtime.GOOS == "windows"`; use exact match otherwise.

## 4. Write Permission Probe

- [x] 4.1 Implement `isWritable(path string) bool` using `os.OpenFile(path, os.O_WRONLY, 0)` and closing immediately on success; return false on any error.
- [x] 4.2 Apply `isWritable` to each collected `.html` path in `Detect`; exclude non-writable files from `InjectableFiles`.

## 5. Main Entry Point

- [x] 5.1 Implement `Detect(dir string) (DetectionResult, error)` as the package's public entry point.
- [x] 5.2 Run framework detection first; if a framework is found, return `ModeManual` immediately with the framework name in `ManualReason`.
- [x] 5.3 Run the HTML walk and write-permission probe to collect writable `.html` files.
- [x] 5.4 If any collected file is named `index.html`, filter to only those files (drop all other `.html` files from the candidate list).
- [x] 5.5 If no files remain, return `ModeManual` with reason `"no writable HTML files found"`.
- [x] 5.6 Return `ModeAuto` with `InjectableFiles` sorted (`sort.Strings`) and an empty `ManualReason`.

## 5a. Manual Target Detection

- [x] 5a.1 Add `ManualTarget string` to `DetectionResult`.
- [x] 5a.2 Create `layout.go` with `findManualTarget(dir, framework)`: for Next.js, return the first existing file among `app/layout`, `src/app/layout`, `pages/_document`, `src/pages/_document`, each tried with extensions `.tsx`, `.jsx`, `.ts`, `.js`; for Nuxt, return the first existing `nuxt.config` file with extension `.ts`, `.js`, `.mjs` or `.mts`; return `""` for other frameworks or when none exists.
- [x] 5a.3 Populate `ManualTarget` in `Detect` when a framework is detected.

## 6. Tests

- [x] 6.1 Create `pkg/installer/otel/internal/rum/detect_test.go` with a table-driven test for `Detect` using `t.TempDir()` to build temp directory trees.
- [x] 6.2 Test cases:
  - Static HTML project: one `.html` in root → `ModeAuto`, one injectable file.
  - Next.js via config: `next.config.js` present → `ModeManual`, reason "Next.js".
  - Next.js via dependency: `package.json` with `"next"` dep, no config file → `ModeManual`, reason "Next.js".
  - Nuxt via config: `nuxt.config.js` present → `ModeManual`, reason "Nuxt".
  - Nuxt via dependency: `package.json` with `"nuxt"` dep, no config file → `ModeManual`, reason "Nuxt".
  - No framework: `package.json` with malformed JSON → framework not detected, continues to HTML scan.
  - CRA-style: `public/index.html` + no framework config → `ModeAuto`, `public/index.html` in `InjectableFiles`.
  - HTML under `dist/`: excluded; root `.html` is injectable.
  - HTML under `node_modules/`: excluded.
  - HTML under `build/`: excluded.
  - No HTML files at all → `ModeManual`, reason "no writable HTML files found".
  - `index.html` present alongside other `.html` files → `ModeAuto`, only `index.html` in `InjectableFiles`.
  - Angular-style: `index.html` + many `*.component.html` files → `ModeAuto`, only `index.html` returned.
  - No `index.html`, multiple `.html` files → `ModeAuto`, all files sorted in `InjectableFiles`.
  - Deeply nested `src/main/resources/templates/index.html` → `ModeAuto`, that file in `InjectableFiles`.
  - `.html` under a hidden directory (`.cache/index.html`) → `ModeManual`, reason "no writable HTML files found".
  - `.html` in a sibling directory and in the parent of the scan root → ignored; only files under the scan root are returned.
  - Manual target (`layout_test.go`): each of `.js`, `.jsx`, `.ts`, `.tsx` in `app/layout`; `src/app/layout`; `pages/_document`; `src/pages/_document`; App Router wins over Pages Router; no layout file → empty target; Nuxt config in each of `.ts`, `.js`, `.mjs`, `.mts`; Nuxt ignores Next.js layout paths; Nuxt without a config file → empty target; `Detect` on Next.js returns the layout as `ManualTarget`.
  - Mixed: framework detected alongside `.html` files → `ModeManual` (framework takes precedence).
- [x] 6.3 Run `go test ./pkg/installer/otel/internal/rum/...` and confirm all cases pass.
- [x] 6.4 Run `make lint` to confirm no golangci-lint issues in the new package.

## 7. OTel Installer Integration

- [x] 7.1 Move `IsNextJSProject`, `IsNuxtProject`, and `HasDependency` into `pkg/installer/otel/markers/js_markers.go` and update the caller in `nodejs_project.go`.
- [x] 7.1a Move the `IsNextJSProject` tests from `nodejs_project_test.go` into `markers/js_markers_test.go`, next to the code under test, and add tests for `IsNuxtProject` (config files incl. `.mts`, dependency, negative case) and `HasDependency` (dependencies, devDependencies, absent, malformed JSON, missing file).
- [x] 7.2 Import `pkg/installer/otel/internal/rum` in `pkg/installer/otel/otel.go`.
- [x] 7.3 In `InstallOtelCollectorWithProject`, after project detection and before the preview display: check `featureflags.IsEnabled(featureflags.Experimental)`; if true, call `rum.Detect` on the selected project directory (falling back to `projectPath`, then `os.Getwd()`, when no project was selected) and store the result; log errors at debug level without aborting.
- [x] 7.4 In the preview display block, if the `rumResult` is non-nil, print a numbered "Real User Monitoring [experimental]" section showing `Mode: auto-inject` with the injectable file paths, or `Mode: manual (auto-injection not possible)` without naming the detected framework, followed by an `Add:` line with the target file when `ManualTarget` is set.
- [x] 7.5 Run `go test ./pkg/installer/otel/...` and confirm all packages pass.
