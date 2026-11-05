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
- Output of the manual-instructions flows (Go SDK guidance, Windows self-uninstall instructions).

## Decisions

### Step helper in `pkg/display`

`RunStep(label, fn)` draws a spinner on stdout when stdout is a TTY, also with `-v`/`--debug`. It collapses to `✓` or `✗` when `fn` returns. Otherwise only the final line is printed. `Step.Detail` updates the text next to the spinner (progress, "3/5 ready").

### Failed steps say so

Step labels are past tense ("Deployed Dynatrace Operator"), so a failure is printed as `✗ <label>  failed` to avoid reading as a success.

### One header per install

Installers print `dtwiz · <method>` through `display.InstallHeader`. `dtwiz setup` already prints "Installing: …" and marks it shown, so the header is not repeated there; a direct `dtwiz install <method>` still shows it.

### Verbose detail is not slog

`display.Detail` prints dim, indented lines when `logger.Enabled()`. This keeps `-v` readable (commands, paths) and leaves `logger.*` for diagnostics. Log lines written while a spinner is active clear the spinner line first (the logger writes through a display-provided writer), and subprocess output shown with `-v` goes through `display.Stream` for the same reason, so `-v` still shows the step in progress.

### Silent phases get a spinner

`RunQuiet(label, fn)` shows a spinner with a present-tense label and removes it without a result line, for phases whose helpers print `✓`/`⚠` lines themselves (tenant setup, project scans, lookups). Without it these phases showed nothing until they finished.

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
