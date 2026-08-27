## Why

Users running `dtwiz install otel` without `--project` see a list of detected projects and a bare `Select a project to instrument [1-N]:` prompt, with no indication of which runtimes dtwiz can actually auto-instrument. Runtimes outside that set (PHP, C++, .NET, Elixir, Erlang, Go, Ruby, Rust) have no supported path forward from the prompt itself, leaving users to guess or search docs. Fixing this surfaced a real bug in the shared `display.PrintInfoBox` helper: it measured line width by rune count, so any line containing an OSC 8 hyperlink or ANSI color escape sequence broke the box's right-border alignment.

## What Changes

- Add an info box, printed once per install run right after the detected-projects list and before the selection prompt, stating that dtwiz auto-instruments Python, Java, and Node.js, and linking to the OpenTelemetry walkthroughs docs for other runtimes (PHP, C++, .NET, Elixir, Erlang, Go, Ruby, Rust). The link renders as a clickable OSC 8 hyperlink on terminals that support it, falling back to the plain URL otherwise.
- The box is shown once per install run — a guard prevents it from reprinting when the user retries project selection after picking a project that can't be auto-instrumented.
- Fix `display.PrintInfoBox` to compute line padding from visible width (stripping ANSI/OSC 8 escape sequences) instead of raw rune count, so box borders stay aligned when a line contains a hyperlink or color code. This also fixes the pre-existing AWS Lambda install info box, which embeds a hyperlink the same way.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `otel-project-scan`: the interactive project-selection flow now informs the user which runtimes are auto-instrumented before they choose a project.
- `display-package`: `PrintInfoBox` now computes padding from visible (escape-sequence-stripped) width, so lines containing hyperlinks or color codes render with correctly aligned borders.

## Impact

- `pkg/installer/otel/otel.go`: new `printSupportedRuntimesInfoBox()` helper and one call site in the interactive project-selection loop.
- `pkg/display/print.go`: new `visibleWidth()` helper; `PrintInfoBox` padding calculation updated to use it.
- No new dependencies, no breaking changes, no feature flag involved.
