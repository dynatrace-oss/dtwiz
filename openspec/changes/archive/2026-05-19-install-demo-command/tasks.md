# Tasks: install demo command

## 1. Refactor: Move confirmProceed to installer.go

- [x] 1.1 Add `var AutoConfirm bool` to `pkg/installer/installer.go`
- [x] 1.2 Move `confirmProceed()` from `pkg/installer/kubernetes.go` to `pkg/installer/installer.go`, add `AutoConfirm` check (return true immediately when set)
- [x] 1.3 Remove the `confirmProceed()` definition from `pkg/installer/kubernetes.go`
- [x] 1.4 Verify build compiles and all callers still resolve (`make build`)

## 2. Add --yes / -y persistent flag

- [x] 2.1 Add `var autoConfirm bool` in `cmd/install.go` and wire `--yes`/`-y` persistent flag on `installCmd` to set `installer.AutoConfirm`
- [x] 2.2 Add `--yes`/`-y` persistent flag on `updateCmd` in `cmd/update.go`
- [x] 2.3 Add `--yes`/`-y` persistent flag on `uninstallCmd` in `cmd/uninstall.go`
- [x] 2.4 Verify `--yes` skips prompt in `install otel`, `install kubernetes`, `update otel`, `uninstall otel` manually

## 3. Add --project flag to install otel and install otel-python

- [x] 3.1 Add `var otelProject string` in `cmd/install.go` and register `--project` flag on `installOtelCmd` and `installOtelPythonCmd`
- [x] 3.2 Pass `otelProject` into `installer.InstallOtelCollector()` — update function signature to accept `projectPath string`
- [x] 3.3 In `InstallOtelCollector` (`pkg/installer/otel.go`): when `projectPath != ""`, skip `detectAllProjects` + `selectProject`; detect runtime from path and build instrumentation plan directly; error if path does not exist
- [x] 3.4 Pass `otelProject` into `installer.InstallOtelPython()` — update function signature to accept `projectPath string`
- [x] 3.5 In `InstallOtelPython` (`pkg/installer/otel_python.go`): when `projectPath != ""`, use it directly and skip scan; error if path does not exist
- [x] 3.6 Verify `install otel --project ./myapp` works end-to-end (skips scan, instruments path)

## 4. Add Python auto-installation

- [x] 4.1 In `pkg/installer/demo.go` (new file), implement `ensurePython()`: detect `python3`/`python` on PATH; if not found, detect OS and return the install command to run
- [x] 4.2 Implement macOS path: check `brew` availability; if found plan `brew install python3`; if not found return error with Homebrew URL
- [x] 4.3 Implement Linux path: read `/etc/os-release`; for Debian/Ubuntu plan `sudo apt-get install -y python3`; for RHEL/Fedora/CentOS plan `sudo dnf install -y python3`
- [x] 4.4 Implement Windows path: plan `winget install Python.Python.3`
- [x] 4.5 Execute the planned install command after user confirms (or immediately if `AutoConfirm`)

## 5. Implement install demo command

- [x] 5.1 In `pkg/installer/demo.go`, implement `checkDemoExists() bool` — checks if `./schnitzel/` exists in CWD
- [x] 5.2 Implement `downloadAndExtractDemo(dryRun bool) error` — download ZIP to temp file, extract to temp dir, rename `schnitzel-master/` → `./schnitzel/` atomically
- [x] 5.3 Implement `InstallDemo(envURL, accessTok, platformTok string, dryRun bool) error` — orchestrate: build plan steps, print plan, confirm, execute (download, python, otel collector + instrumentation)
- [x] 5.4 Add `installDemoCmd` to `cmd/install.go` with `Args: cobra.NoArgs`, wired to `installer.InstallDemo()`
- [x] 5.5 Register `installDemoCmd` in `installCmd.AddCommand(...)` in `cmd/install.go`
- [x] 5.6 Verify `install demo --dry-run` shows plan without executing
- [x] 5.7 Verify `install demo --yes` skips confirmation prompt

## 6. Tests

- [x] 6.1 Unit test `checkDemoExists()`: exists / not-exists cases
- [x] 6.2 Unit test `ensurePython()` plan generation for each OS branch (mock `runtime.GOOS` and PATH)
- [x] 6.3 Unit test `--project` path-not-found error in `InstallOtelCollector` and `InstallOtelPython`
- [x] 6.4 Unit test `confirmProceed()` with `AutoConfirm = true` returns true without reading stdin
- [x] 6.5 Run full test suite: `make test`

## 7. Final checks

- [x] 7.1 Run `make lint` and resolve any new issues
- [x] 7.2 Update `CHANGELOG.md` with new features under `[Unreleased]`
