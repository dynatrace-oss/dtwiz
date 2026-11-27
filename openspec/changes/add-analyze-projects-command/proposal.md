# Proposal

## Why

Agents (and scripts) that want to know which projects dtwiz can instrument with OpenTelemetry currently run `dtwiz setup` or `dtwiz install otel` and abort at the selection prompt. That path needs valid Dynatrace credentials, a round-trip to the tenant, and a cloud metadata probe before the list appears, prints human-formatted text only, and then blocks on stdin. There is no read-only, non-interactive way to get the list, and no way to act on every listed entry afterwards: a directory that holds both a Node.js and a Python project is listed twice, but `--project <path>` always infers the same runtime for it.

## What Changes

- New subcommand `dtwiz analyze projects`, a read-only, credential-free, non-interactive listing of the projects the OTel installer would detect.
  - `--dir <path>`: scan root, default the working directory. No home-directory scan, no prompt, and the bundled examples directory is not added.
  - `--runtime <id>[,<id>...]`: comma-separated filter over a fixed, documented set of runtime identifiers (`python`, `java`, `nodejs`, `go`, plus `dotnet` only while the `DotNetOtel` feature flag is on), exact match only, no aliases.
  - `--json`: machine-readable output (object with scanned roots, per-runtime availability, and projects, each carrying the `install` command that selects it).
  - Human-readable output: a table sorted by path, then runtime, with a single install hint as footer.
  - Projects whose runtime toolchain is missing are listed with a note instead of being dropped, but only for runtimes within the filter.
  - Exit code 0 for any result including an empty one; 1 for an unknown runtime or a missing directory.
- Bare `dtwiz analyze` keeps its current behavior. `--json` becomes available to both `analyze` and `analyze projects`.
- `dtwiz install otel` gains `--runtime <id>` (exactly one value from the same set) as an addition to `--project`, so every entry listed by `analyze projects` can be installed non-interactively.
  - `--runtime` without `--project` is an error.
  - `--runtime` naming a runtime with no project markers in the given directory is an error.
  - `--project` without `--runtime` keeps today's runtime inference.
- `dtwiz install otel` output is unchanged; the `Skipping <runtime> instrumentation` lines are still printed there.

No breaking changes.

## Capabilities

### New Capabilities

- `project-inventory`: the `dtwiz analyze projects` command: scan scope, runtime filter and its value set, availability reporting, human-readable and JSON output, and exit codes.

### Modified Capabilities

- `project-flag`: `install otel` accepts `--runtime` together with `--project` to select the runtime explicitly; validation of the value and of its pairing with `--project`.
- `otel-project-scan`: the always-on bundled examples root applies to the interactive install and setup flows, not to the project inventory listing.

## Impact

- **Code**: `cmd/analyze.go` (persistent `--json`, subcommand registration), new `cmd/analyze_projects.go`, `cmd/install.go` (`--runtime` on `install otel`); `pkg/installer/otel` gains an exported inventory API (query, runtime identifiers, plain-Go result) and the scan gains an opt-out for the bundled examples root; a new presentation package renders the inventory as text or JSON.
- **Feature flags**: `DotNetOtel` (`--dotnet-otel` / `DTWIZ_DOTNET_OTEL`) gates everything .NET: while it is off, `dotnet` is not a valid runtime identifier, is absent from help and error messages, and no .NET projects are listed. This change builds on the .NET detection from the in-flight `add-dotnet-otel-instrumentation` change and overlaps in `pkg/installer/otel/otel.go` with the in-flight `add-dotnet-otel-plan-and-install` change (.NET steps 2 and 3); .NET projects are reported as ending in manual steps, which stays true until .NET launching ships in step 4.
- **Self-monitoring**: `analyze projects` reports as `command=analyze`, `method=projects` with the existing event mapping; no new event types.
- **Docs**: README command table and JSON section, `AGENTS.md` CLI list and conventions (a parent command that is also runnable).
- **Rollback**: purely additive. Removing the subcommand and the `--runtime` flag restores today's behavior; no persisted state or config is involved.
