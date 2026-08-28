# Design

## Context

`dtwiz install otel` without `--project` scans for projects, prints a numbered list, and prompts `Select a project to instrument [1-N]:` in a loop (retrying if the chosen project can't be auto-instrumented). Auto-instrumentation only covers Python, Java, and Node.js; other runtimes have no path forward from this prompt. The shared `display.PrintInfoBox` helper renders a bordered box by padding each line to a fixed width using `len([]rune(line))`.

## Goals / Non-Goals

**Goals:**

- Tell the user, at the point of decision, which runtimes are auto-instrumented and where to find manual walkthroughs for the rest.
- Do not reprint the notice on every retry of the selection loop.
- Fix `PrintInfoBox` so lines containing OSC 8 hyperlinks or ANSI color codes still render with an aligned right border.

**Non-Goals:**

- Adding auto-instrumentation support for any additional runtime.
- Changing the AWS Lambda install flow's own info box content (only the shared padding bug is fixed, which incidentally corrects that box's alignment too).
- General ANSI-aware text wrapping/truncation inside `PrintInfoBox` — only width measurement changes.

## Decisions

- **Where the box is inserted**: after the detected-projects list, immediately before `selectProject()`'s prompt (inside the retry loop's body), rather than before the list or outside the loop entirely — this keeps it adjacent to the decision the user is about to make, per direct product feedback that this placement is the most likely to be read.
- **Once-per-run guard**: a boolean local to the retry loop (`infoBoxShown`) rather than hoisting the print above the loop, so the notice still appears after the first project listing (not before it) while not repeating on retries. Simplest option that satisfies both "after the list" and "shown once".
- **Hyperlink handling**: reuse the existing `display.Hyperlink`/`display.StdoutSupportsHyperlinks()` pair (same pattern as the AWS Lambda install box) instead of introducing a new helper.
- **Width fix**: strip ANSI/OSC 8 escape sequences with a regex before taking `len([]rune(...))`, rather than tracking visible-vs-raw length alongside each line or banning escape sequences in box content (the previous, more restrictive doc comment). Regex-based stripping is a self-contained fix in one place (`PrintInfoBox`) with no API changes for callers.

## Risks / Trade-offs

- [The escape-sequence regex could miss some exotic ANSI sequence] → Scope is limited to what this codebase actually emits: SGR color codes (`\x1b[...m`) and OSC 8 hyperlinks, both covered.
- [Fixing `PrintInfoBox` changes rendering for the existing AWS Lambda info box] → This is a correctness fix (that box was already misaligned whenever hyperlinks render); no behavior is being removed, only the border alignment is corrected.
