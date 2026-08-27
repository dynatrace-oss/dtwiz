# Proposal: clarify-analysis-and-recommendation-wording

## Why

Moderated user testing of the Dynatrace QuickStart flow (three participants, 24.08.2026) produced a most-severe finding: a participant chose OneAgent over OpenTelemetry, had his entire laptop instrumented instead of his project, and afterwards stated he "had no basis to choose between OneAgent and OpenTelemetry" — he picked on name recognition alone.

The recommendation menu is the screen where that choice is made, and it gives the user nothing to choose on. Two of its entries open with the identical phrase "This machine's services", and neither states what data will actually be ingested. The System Analysis block printed immediately above shares the problem: it labels the host line "Platform:", which in `dtwiz status` sits eight lines from "Platform Token:" meaning something unrelated, and it labels detected runtimes "Services:" when what it detects is binaries on `PATH`.

## What Changes

System Analysis block (`analyze`, `setup`, `status`):

- The host line is labelled `This host:` instead of `Platform:`, so "platform" refers to exactly one thing in dtwiz output.
- The OneAgent line moves directly below OpenTelemetry. These are the only two lines answering "is this host already monitored?" and the only two whose state changes the recommendation menu printed below; they are no longer separated by the container and cloud lines.
- The runtimes line is labelled `Runtimes:` instead of `Services:`, matching what is detected — runtimes present on `PATH` plus a small set of daemons — rather than implying a set of running services.

Recommendation list (`setup`, `recommend`):

- The list is introduced by a single lead-in, `Monitor Logs, Metrics, Traces of:`, stating once what every option ingests. Each entry then reads as an object completing that phrase.
- The three colliding entries are retitled so scope is the visible difference: `This host and its services (via existing OpenTelemetry Collector)`, `... (via new OpenTelemetry Collector)`, and `... (via OneAgent)`.

No behavior changes: detection logic, method selection, install dispatch, and the `analyze --json` / `recommend --json` payloads are untouched. `analyze --json` still emits the `platform` key.

## Capabilities

### New Capabilities

- `system-analysis-output`: the labels, vocabulary, and line order of the System Analysis block shared by `analyze`, `setup`, and `status`.

### Modified Capabilities

- `setup-recommendations`: the recommendation list gains a signal lead-in, and entry titles must distinguish monitoring scope rather than repeating a shared prefix.

## Impact

- `pkg/analyzer/analyzer.go` — `Summary()` labels and block order.
- `pkg/recommender/recommender.go` — three `Title` values; the lead-in in `FormatRecommendations`.
- `cmd/setup.go` — the lead-in in the interactive menu.
- `pkg/analyzer/analyzer_test.go` — label assertions.

Affects the rendered output of four commands: `analyze`, `setup`, `status`, `recommend`.

Feature flag interaction: the `via existing OpenTelemetry Collector` entry (`MethodOtelUpdate`) is only selectable in `setup` when `Experimental` is enabled, so the three-way title collision is visible in `setup` only under `--experimental`. `recommend` does not apply that filter and shows the entry unconditionally — a pre-existing inconsistency this change does not address.

No rollback plan required: the change is display-only, carries no migration, and reverts cleanly with the commit.
