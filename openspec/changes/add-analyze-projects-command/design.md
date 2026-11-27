# Design

## Context

See proposal.md for motivation and specs/ for the required behavior.

Current state that shapes the approach:

- All project detection lives in `pkg/installer/otel`, unexported: `detectAvailableRuntimes()` defines the runtimes, `detectAllProjects(runtimes, roots)` checks each runtime's binary and fans out the per-runtime detectors, and every detector ends in `scanProjectDirs(markers, excludes, roots)`.
- `detectAllProjects` both decides availability and prints the `Skipping <runtime> instrumentation` lines to stdout. Unavailable runtimes are not scanned at all.
- `scanProjectDirs` always appends `~/.dtwiz/examples/` to the roots it is given. Its only caller chain into a scan is `InstallOtelCollectorWithProject`, via `selectScanRoots()`.
- `inferRuntimeFromPath` checks top-level markers in the order Java, Node.js, Python, Go, .NET (flag-gated). Runtimes are identified internally by display strings (`"Python"`, `"Node.js"`, `dotNetRuntimeName`), which `createRuntimePlan` switches on.
- `cmd/analyze.go` is a leaf with a local `--json` flag; it does not override root's `PersistentPreRun`, so feature flags are already applied for any subcommand.
- `pkg/analyzer` does not import `pkg/installer/otel`; the cheap cwd-only `project_techs` detection stays there and is untouched.

## Goals / Non-Goals

**Goals:**

- One detection path: `analyze projects` and `install otel` call the same detectors and the same availability check, so their results cannot drift.
- Business logic decoupled from Cobra and from presentation: `cmd` only reads flags and wires things together; `pkg` returns a plain-Go result; rendering is a separate package.
- Fail fast: `--runtime` validation on `install otel` happens before credentials are resolved or anything is changed.

**Non-Goals:**

- Moving detection out of `pkg/installer/otel` into its own package (possible follow-up; too large for this change).
- Reusing the new text renderer for the numbered selection list of `install otel`.
- Process matching for Go projects during a scan (today Go PIDs are matched only on the `--project` path; the inventory reports what the scan reports).
- A `--running` filter, runtime aliases, or multiple `--dir` roots.
- Changing `install otel-python`, `install otel-node`, or `install otel-java`. With `install otel --project <path> --runtime <id>` covering the same ground, these subcommands are expected to be removed in a separate change, so this change does not specify or align them. The `project-flag` spec is therefore only modified where `--runtime` contradicts it (runtime inference on `install otel`); its known gaps for these subcommands (Node.js and Java are not mentioned, and their missing-path error messages differ from the specified one) are left for the removal change. Before they can be removed, `install otel` needs an equivalent of their `--service-name` flag, or a decision that it is not needed.

## Decisions

### D1: Layering

```text
cmd/analyze_projects.go        Cobra: flags --dir, --runtime; pass raw strings to pkg;
                               choose renderer by --json; fire self-monitoring
        |
        v
pkg/installer/otel             domain (project_inventory.go, runtime_id.go)
  ParseRuntimeFilter(string) ([]RuntimeID, error)
  ParseRuntime(string) (RuntimeID, error)
  DetectProjects(ProjectQuery) (ProjectInventory, error)
        |
        v  ProjectInventory (no json tags, no colors, absolute paths)
pkg/projectreport              presentation
  RenderText(io.Writer, ProjectInventory) error
  RenderJSON(io.Writer, ProjectInventory) error
```

The domain stays in `pkg/installer/otel` because every detector it needs is unexported there. Exporting a small, intention-revealing API is far smaller than relocating the detectors. *Alternative:* a new `pkg/projects` that owns detection; rejected for this change (large move, mixed with logic changes).

The presentation package is separate because rendering is a different concern from detection and is consumed by `cmd` only. *Alternatives:* put renderers in `pkg/display` (rejected: `display` holds generic, method-agnostic helpers) or in `pkg/installer/otel` (rejected: mixes presentation into the domain package).

### D2: Domain types

```go
type RuntimeID string // "python", "java", "nodejs", "go", "dotnet"

type ProjectQuery struct {
    Dir      string      // absolute or relative; "" means working directory
    Runtimes []RuntimeID // empty means all enabled runtimes
}

type ProjectInventory struct {
    Roots    []string        // absolute
    Runtimes []RuntimeStatus // only those in scope, in identifier-set order
    Projects []InventoryProject
}

type RuntimeStatus struct { ID RuntimeID; Available bool; Reason string }

type ProjectStatus int // Instrumentable, ManualSteps, RuntimeUnavailable

type InventoryProject struct {
    Runtime RuntimeID
    Path    string // absolute
    Markers []string
    PIDs    []int
    Module  string
    Status  ProjectStatus
}
```

`DetectProjects` sorts `Projects` by path, then runtime identifier, so both renderers inherit the same order. Statuses are enums in the domain; their string forms (`instrumentable`, `manual_steps`, `runtime_unavailable`) live in the JSON renderer.

The status names describe the outcome of the install command, not the maturity of a runtime's support. *Alternative:* `detection_only` for .NET (rejected: it stops being true once `add-dotnet-otel-plan-and-install` makes `install otel` install the .NET instrumentation, while "ends in manual steps" stays true until launching ships in .NET step 4). Which status a runtime gets is a property in the runtime table, so step 4 changes one entry.

`runtimeInfo` gains an `id RuntimeID` field; the internal display strings stay as they are so `createRuntimePlan` and the install flow are untouched. A single table maps identifier to `runtimeInfo`, which is also the source of the documented set and of the "valid: ..." error text. The set of active identifiers is computed at call time from the feature flags: `dotnet` is included only while `DotNetOtel` is on, so with the flag off it is indistinguishable from any other unknown value.

### D3: Availability becomes data

The binary check inside `detectAllProjects` is extracted into `checkRuntimeAvailability(runtimeInfo) runtimeAvailability{available bool; reason string; message string}`:

- `reason` is the short form used by the inventory (`python3 not found on PATH`).
- `message` is today's full "Skipping ..." line, which `detectAllProjects` keeps printing for `install otel`, so that output is unchanged.

`exec.LookPath` in that check goes through a package-level function variable (like `goLookPath`) so tests can stub availability.

`DetectProjects` calls the detector for every runtime in scope regardless of availability, then sets `Status = RuntimeUnavailable` for projects of unavailable runtimes. .NET projects get `ManualSteps`. It never writes to stdout.

### D4: Bundled examples root moves to root selection

`scanProjectDirs` stops appending `~/.dtwiz/examples/`. `selectScanRoots()` appends it (when it exists) to whatever roots it returns, in every branch. Because deduplication of matched directories happens inside a single `scanProjectDirs` call across all its roots, the "no duplicate when cwd is under examples" behavior is preserved. `DetectProjects` passes only its own root, so it gets no examples. *Alternative:* a boolean parameter on `scanProjectDirs` and all five detectors; rejected as it threads a flag through every signature for one caller difference.

### D5: Validation errors

The domain returns typed errors so `cmd` stays free of message logic:

- `UnknownRuntimeError{Value string; Valid []RuntimeID}` → `unknown runtime "<v>" (valid: <active identifiers>)`, e.g. `python, java, nodejs, go` with `DotNetOtel` off
- directory missing → `directory not found: <path>`

There is deliberately no separate "feature disabled" error: a gated runtime with its flag off is simply not in the active set. `ParseRuntimeFilter` splits on `,`, rejects empty elements, deduplicates, and validates against the active set. `ParseRuntime` additionally rejects a comma (`--runtime takes a single value`).

Cobra prints returned errors to stderr and `cmd.Execute()` exits 1, which yields the spec's exit codes without extra handling.

### D6: `install otel --runtime`

- `cmd/install.go` adds `--runtime` to `installOtelCmd` only (not `otel-python`/`otel-node`/`otel-java`, whose runtime is implied).
- Before `getDtEnvironment()`, `cmd` calls `otel.ResolveProjectRuntime(projectPath, runtimeFlag) (RuntimeID, error)`, which rejects `--runtime` without `--project`, parses the value, and checks `hasRuntimeMarkers(path, id)`.
- `hasRuntimeMarkers` reuses the per-runtime marker lists that `inferRuntimeFromPath` already checks; `inferRuntimeFromPath` is rewritten as "first runtime in inference order whose markers are present", so both share one marker definition.
- `InstallOtelCollectorWithProject` gains a runtime argument; when non-empty it replaces `inferRuntimeFromPath(projectPath)`. `setup` and `InstallOtelCollector` pass empty.

### D7: Command wiring

- `analyzeCmd` keeps its `RunE` and `Args: cobra.NoArgs`; `analyzeProjectsCmd` is added as a child with `Args: cobra.NoArgs`. Cobra resolves the subcommand before the parent's `Args` check.
- `--json` moves to `analyzeCmd.PersistentFlags()`; `analyzeJSON` stays a package-level variable shared by both handlers (same pattern as `--dry-run`).
- `analyze`'s `Long` text mentions `analyze projects` so it shows up from `dtwiz analyze --help`.
- The `--runtime` usage text lists the active identifiers. `PersistentPreRun` does not run for `--help`, so, as `installCmd` already does, `analyzeCmd` and `installCmd` set a help func that calls `featureflags.ApplyCLIOverrides` and refreshes the `--runtime` usage text before rendering help.
- Self-monitoring: `fireCompletedEvent(cmd, err)`; the existing parent/child mapping yields `command=analyze, method=projects`.

### D8: Rendering details

- Text: header with `display.ColorMessage`, divider, `Scanned: <root>` (plus `Runtimes: a, b` when filtered), table via `text/tabwriter` without colors inside cells (ANSI codes break column widths), footer only when projects exist. Paths are shown with `~` for the home prefix.
- Notes column joins, in this order: availability reason, `manual steps after install`, `PID n` / `PIDs a, b`, `module m`.
- JSON: private DTOs with snake_case tags; `pids`, `module`, `reason`, `install_command` use `omitempty`; `projects` is always an array (never `null`).
- `install_command`: built in the renderer from runtime and absolute path; the path is wrapped in double quotes when it contains whitespace or shell-special characters.

### D9: AGENTS.md convention

The CLI conventions gain one line: a parent command may also be runnable (own `RunE`, `Args: cobra.NoArgs`) when the bare verb is meaningful on its own, as with `analyze`.

### D10: Coordination with `add-dotnet-otel-plan-and-install`

That sibling change (.NET steps 2 and 3) is in flight and touches the same code:

- Its task 5.4 replaces `showDotNetStep1Outcome` in the selection flow and adds the .NET case to `createRuntimePlan` in `pkg/installer/otel/otel.go`. D6 edits `InstallOtelCollectorWithProject` in the same file, including the `--project` branch, which has its own .NET path that also calls `showDotNetStep1Outcome`. Neither change depends on the other's logic; whichever lands second resolves the conflict and keeps both behaviors. The sibling's 5.4 should also cover the `--project` branch; that is its concern, noted here so it is not lost.
- With the sibling landed, `install otel --project <path> --runtime dotnet` builds the .NET plan instead of showing the step-1 outcome. The inventory's `manual_steps` status stays correct, since the sibling's flow still ends in the manual-steps outcome.
- Both keep .NET projects listed when `dotnet` is missing (the sibling reports it through its prerequisite check), so the inventory keeps reporting `dotnet` as available and keeps the `install_command`.
- The umbrella spec's "selection by path" requirement (in `add-dotnet-otel-instrumentation`, not yet in the main specs) describes inference with marker precedence; `--runtime dotnet` deliberately bypasses that precedence, as stated in the `project-flag` delta.

## Risks / Trade-offs

- [Detectors print to stdout somewhere other than `detectAllProjects`] → a JSON-output test captures stdout of `analyze projects --json` with an unavailable runtime and asserts it parses as a whole.
- [Scanning unavailable runtimes costs time that `install otel` saves] → only in the inventory, and `--runtime` narrows it; `install otel` keeps skipping them.
- [Moving the examples root changes `scanProjectDirs` behavior that existing tests rely on] → update `home_scan_test.go` and related tests to assert the root via `selectScanRoots`; keep the dedup scenario covered.
- [Hidden-directory skip could hide `~/.dtwiz/examples` when passed as `--dir`] → the walk does not apply the skip to the root itself; covered by a test for `--dir ~/.dtwiz/examples`.
- [Double-quoting is not a full shell-escaping scheme (`$` or a backtick inside a path still expands in POSIX shells)] → acceptable for real-world project paths; JSON consumers can also build the command from `path` and `runtime`.
- [`install_command` and the accepted flag values could diverge] → both derive from the same identifier table; a test runs `ResolveProjectRuntime` against every `install_command` produced for a fixture tree.

## Migration Plan

Additive. No config or state migration. Rollback is reverting the change; `install otel` without `--runtime` behaves as before throughout.
