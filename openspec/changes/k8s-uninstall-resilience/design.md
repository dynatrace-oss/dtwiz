# Design

## Context

`dtwiz uninstall kubernetes` runs 4 sequential steps to tear down the Dynatrace Operator. The original implementation returned immediately on any step error, skipping the remaining steps. This left the cluster in a partially uninstalled state — most visibly a lingering `dynatrace` namespace when the helm release was already absent. Additionally, Cobra's default `SilenceUsage=false` caused the full usage block to print on any error, and a redundant `fmt.Fprintln` in `Execute` doubled every error message.

## Goals / Non-Goals

**Goals:**

- All 4 uninstall steps always execute regardless of individual step failures
- Usage block appears only for invalid invocations (bad flags, unknown subcommands), not runtime errors
- Each error is printed exactly once

**Non-Goals:**

- Retrying failed steps
- Changing the uninstall step order or what each step does

## Decisions

**Collect-and-continue over fail-fast**
Steps 1, 3, and 4 collect errors into a `[]error` slice and continue. Step 2 (wait for pods) remains non-fatal as before — it was already a warning-only step. At the end, if any errors were collected, return `errors.New("uninstall: one or more steps failed (see above)")` — a sentinel that signals failure without duplicating the already-printed details.

Alternative considered: wrapping all errors with `errors.Join` and returning that. Rejected because Cobra prints the returned error, which would duplicate the inline step output.

**`SilenceUsage` set in `PersistentPreRun`, not on the command struct**
Setting `SilenceUsage: true` statically on `rootCmd` would suppress usage for bad flags too. Setting it inside `PersistentPreRun` — which only fires after flags parse successfully — preserves usage for invalid invocations while suppressing it for runtime errors.

`install`, `update`, and `uninstall` each override `PersistentPreRun` (Cobra does not chain parent hooks by default), so each must set `cmd.Root().SilenceUsage = true` independently.

**Remove duplicate `fmt.Fprintln` from `Execute`**
Cobra's `ExecuteC` already calls `c.PrintErrln(cmd.ErrPrefix(), err.Error())` when `SilenceErrors` is false (the default). The manual print in `Execute` was an unconditional duplicate for every command.

## Risks / Trade-offs

- [Step 3 fails silently to the user mid-flow] → Mitigated: errors are printed inline immediately with `fmt.Printf("  Error: %v\n", err)` before continuing
- [Sentinel error loses root-cause detail] → Mitigated: root cause already printed inline; sentinel is intentionally terse to avoid repetition
