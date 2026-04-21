## Why

New users need a fast path to see Dynatrace observability in action without bringing their own application. A single `dtwiz install demo` command bootstraps a ready-made 4-service Python app and wires it up to Dynatrace OTel monitoring end-to-end — zero prior setup required.

## What Changes

- New `dtwiz install demo` subcommand that downloads, extracts, and instruments the "schnitzel" demo Python app
- New `--yes` / `-y` persistent flag on `install`, `update`, and `uninstall` command groups to skip all confirmation prompts
- New `--project <path>` flag on `install otel` and `install otel-python` to pre-select a project and skip interactive scanning
- Refactor `confirmProceed()` and shared `AutoConfirm` var from `pkg/installer/kubernetes.go` into `pkg/installer/installer.go`
- Python installation support (macOS/Linux/Windows) when `python3` is not found on PATH

## Capabilities

### New Capabilities

- `install-demo`: Download, extract, and prepare the schnitzel demo app; check/install Python; orchestrate OTel collector + Python auto-instrumentation; show plan and confirm before executing
- `autoconfirm-flag`: `--yes` / `-y` flag on install/update/uninstall command groups; modifies shared `confirmProceed()` to auto-accept when set
- `project-flag`: `--project <path>` flag on `install otel` and `install otel-python`; skips project scan/selection and instruments the specified path directly

### Modified Capabilities

- `python-install-validation`: Python installation check gains active remediation — if `python3` is not found, dtwiz installs it via the platform package manager (brew/apt/dnf/winget) rather than just erroring

## Impact

- **cmd/install.go**: new `installDemoCmd`, new `--yes`/`-y` persistent flag, new `--project` flags
- **cmd/update.go**: new `--yes`/`-y` persistent flag
- **cmd/uninstall.go**: new `--yes`/`-y` persistent flag
- **pkg/installer/installer.go**: add `AutoConfirm` var, move `confirmProceed()` here
- **pkg/installer/kubernetes.go**: remove `confirmProceed()` definition
- **pkg/installer/otel.go**: accept optional project path parameter
- **pkg/installer/otel_python.go**: accept optional project path parameter
- **pkg/installer/demo.go** (new): full demo install orchestration
