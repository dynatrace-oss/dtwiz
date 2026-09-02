# Tasks

## 1. Package Scaffold

- [ ] 1.1 Create `pkg/installer/otel/rum/` directory and `detect.go` file with the `rum` package declaration.
- [ ] 1.2 Define `InjectionMode` string type and `ModeAuto` / `ModeManual` constants.
- [ ] 1.3 Define `DetectionResult` struct with `Mode InjectionMode`, `InjectableFiles []string`, and `ManualReason string`.

## 2. Framework Detection

- [ ] 2.1 Import and alias `isNextJSProject` and `isNuxtProject` from `pkg/installer/otel/nodejs_project.go` (reuse existing OTel detection logic).
- [ ] 2.2 Implement `detectFramework(dir string) (name string, found bool)` in `detect.go` that:
  - Returns `"Next.js"` if `isNextJSProject(dir)` is true
  - Returns `"Nuxt"` if `isNuxtProject(dir)` is true
  - Returns `""` and `false` if none are found
- [ ] 2.3 Expose `detectFramework` for use by the main `Detect` entry point.

## 3. HTML File Walk

- [ ] 3.1 Implement `walkHTML(dir string) ([]string, error)` using `fs.WalkDir` (via `os.DirFS`) to collect `.html` files.
- [ ] 3.2 Skip excluded directories (`node_modules`, `.git`, `.next`, `.nuxt`, `.svelte-kit`, `dist`, `build`, `out`, `.output`, `__pycache__`, `.venv`, `venv`) by returning `fs.SkipDir` from the walk function.
- [ ] 3.3 Apply case-insensitive `.html` matching only when `runtime.GOOS == "windows"`; use exact match otherwise.

## 4. Write Permission Probe

- [ ] 4.1 Implement `isWritable(path string) bool` using `os.OpenFile(path, os.O_WRONLY, 0)` and closing immediately on success; return false on any error.
- [ ] 4.2 Apply `isWritable` to each collected `.html` path in `Detect`; exclude non-writable files from `InjectableFiles`.

## 5. Main Entry Point

- [ ] 5.1 Implement `Detect(dir string) (DetectionResult, error)` as the package's public entry point.
- [ ] 5.2 Run framework detection first; if a framework is found, return `ModeManual` immediately with the framework name in `ManualReason`.
- [ ] 5.3 Run the HTML walk and write-permission probe to collect writable `.html` files.
- [ ] 5.4 If any collected file is named `index.html`, filter to only those files (drop all other `.html` files from the candidate list).
- [ ] 5.5 If no files remain, return `ModeManual` with reason `"no writable HTML files found"`.
- [ ] 5.6 Return `ModeAuto` with `InjectableFiles` sorted (`sort.Strings`) and an empty `ManualReason`.

## 6. Tests

- [ ] 6.1 Create `pkg/installer/otel/rum/detect_test.go` with a table-driven test for `Detect` using `t.TempDir()` to build temp directory trees.
- [ ] 6.2 Test cases:
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
  - Mixed: framework detected alongside `.html` files → `ModeManual` (framework takes precedence).
- [ ] 6.3 Run `go test ./pkg/installer/otel/rum/...` and confirm all cases pass.
- [ ] 6.4 Run `make lint` to confirm no golangci-lint issues in the new package.
