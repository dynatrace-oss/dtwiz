# Design

## Context

Each installer prints through a mix of `display.*` helpers and raw `fmt` calls. The logger already provides `-v` (`logger.Verbose`) and `--debug`, but it writes `slog` key/value lines and is almost unused by install output.

## Goals / Non-Goals

**Goals:**

- One line per step, with a spinner while it runs.
- Wordy output available with `-v`; full plan with `--dry-run`.
- Errors always visible, with enough output to act on.

**Non-Goals:**

- Changing what the installers do.
- Converting the OTel runtime executors and AWS Lambda instrumentation (follow-up).

## Decisions

### Step helper in `pkg/display`

`RunStep(label, fn)` draws a spinner on stdout when stdout is a TTY and logging is off. It collapses to `✓` or `✗` when `fn` returns. Otherwise only the final line is printed. `Step.Detail` updates the text next to the spinner (progress, "3/5 ready").

### Verbose detail is not slog

`display.Detail` prints dim, indented lines when `logger.Enabled()`. This keeps `-v` readable (commands, paths) and leaves `logger.*` for diagnostics. In `-v`/`--debug` mode the spinner is off so log lines do not tear it.

### Printing while a spinner runs

`StepDone`, `StepWarn` and `Detail` take a mutex shared with the spinner and clear the spinner line first, so helpers called inside a step can print safely.

### Errors are shown once

`RunStep` prints `✗ label` plus the last 10 lines of the error and returns the error wrapped as already reported. `cmd.Execute` prints other errors itself (`Error: ...`) because cobra's printing is silenced. A hint points to `-v`/`--debug`.

### Preview stays transparent

The default preview shows a short summary of what will change. `-v` adds the manifest, commands and config; `--dry-run` shows the full plan, so nothing is hidden from a user who wants to inspect it before applying.

### sudo

A spinner would overwrite a sudo password prompt, so the OneAgent install primes `sudo -v` on the terminal before starting the step.

## Risks / Trade-offs

- Installer subprocess output (OneAgent installer) is captured unless `-v` is set; it appears in the error on failure.
- Tests that asserted on commands and manifests in the default preview now enable verbosity.

## Follow-up

The OTel runtime executors (Python, Node.js, Java, Go) and the AWS Lambda instrumentation output still print inline status lines. They are converted in a separate change.
