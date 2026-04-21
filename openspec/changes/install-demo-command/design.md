## Context

`dtwiz` is a Go CLI that deploys Dynatrace observability automatically. Currently all `install` subcommands require users to bring their own application. There is no built-in demo path.

The OTel install flow (`install otel`, `install otel-python`) is interactive: it scans for projects on disk, lists them, and prompts the user to pick one. Confirmation prompts (`confirmProceed`) are defined in `pkg/installer/kubernetes.go` — a misleading location for shared utility code. There is no way to skip prompts non-interactively.

## Goals / Non-Goals

**Goals:**
- Add `dtwiz install demo` that bootstraps the schnitzel Python app and wires it to Dynatrace OTel end-to-end
- Add `--yes` / `-y` to skip all confirmation prompts across install/update/uninstall
- Add `--project <path>` to pre-select a project in the OTel install flow
- Refactor `confirmProceed` into the shared `installer.go` file
- Auto-install Python if not present (macOS/Linux/Windows)

**Non-Goals:**
- Supporting multiple demo apps (schnitzel is the only one for now)
- Persistent demo state tracking beyond checking if `./schnitzel/` exists
- Modifying the schnitzel app itself

## Decisions

### `--yes` / `-y` as a package-level var, not passed through function signatures

**Decision:** Add `var AutoConfirm bool` to `pkg/installer/installer.go`. The flag setter in `cmd/` sets this var. `confirmProceed()` checks it at call time.

**Alternative considered:** Thread a `confirm bool` param through every installer function signature. Rejected — too many call sites, too much churn, no benefit since it's a process-wide setting.

### Move `confirmProceed` to `installer.go`

**Decision:** Physically move the function definition from `kubernetes.go` to `installer.go` where other shared utilities (`AuthHeader`, `APIURL`, `RunCommand`) already live.

**Rationale:** `confirmProceed` is not Kubernetes-specific. Placing it next to `AuthHeader` makes the shared-utilities intent clear.

### `--project` skips scan entirely, does not filter scan results

**Decision:** When `--project <path>` is provided to `install otel` or `install otel-python`, skip `detectAllProjects` / `selectProject` entirely. Detect the runtime from the given path and build the instrumentation plan directly.

**Alternative considered:** Run the scan and auto-select the matching project by path. Rejected — unnecessary work, and the path might not appear in scan results (e.g. if the demo was just extracted and processes aren't running yet).

### Demo extracts to `./schnitzel/` in CWD

**Decision:** Extract the demo zip into the current working directory as `schnitzel/`. Check if `./schnitzel/` exists before downloading.

**Rationale:** Matches developer tool conventions (git clone, npm init). The demo is meant to be explored, not hidden in a dotdir.

### Python installation uses platform package manager with explicit confirmation

**Decision:** If `python3` is not found, show a plan line "Install Python 3 via `<tool>`" and include it in the single pre-execution confirmation prompt. Install using:
- macOS: `brew install python3` (error with helpful message if brew not found)
- Linux (Debian/Ubuntu): `sudo apt-get install -y python3`
- Linux (RHEL/Fedora/CentOS): `sudo dnf install -y python3`
- Windows: `winget install Python.Python.3`

Detect Linux distro via `/etc/os-release`.

### Demo command calls `InstallOtelCollector` directly (not via subprocess)

**Decision:** `demo.go` calls `installer.InstallOtelCollector(...)` directly in-process, with `AutoConfirm = true` and the `--project ./schnitzel` path pre-set.

**Rationale:** Subprocess exec of `dtwiz install otel` would require re-parsing credentials and flags. In-process call reuses resolved credentials and is simpler.

## Risks / Trade-offs

- **Brew not installed on macOS** → Surface a clear error: "Python 3 is required. Install Homebrew first: https://brew.sh" rather than failing silently.
- **schnitzel URL changes** → The download URL (`https://github.com/dietermayrhofer/schnitzel/archive/refs/heads/master.zip`) is hardcoded. If the repo moves or branch is renamed, the command breaks. Mitigation: make URL a named constant, easy to update.
- **`AutoConfirm` is a global var** → Slightly less clean than passing context, but consistent with how `installDryRun` and other package-level flags already work in this codebase.
- **Partial extraction on download failure** → If the zip download or extract fails mid-way, a partial `./schnitzel/` dir may exist. Mitigation: extract to a temp dir, rename atomically on success.

## Open Questions

None — all design decisions resolved in explore phase.
