# Design

## Context

All three GCP entry points resolve `gcloud` state through one function, `gcpAccountInfo` in `pkg/installer/gcp/preflight.go`, but each uses it differently:

- **install** calls it first and in sequence (`install.go`). When a complete connection exists, install delegates to the update flow, which calls it again.
- **update** calls it inside `installer.RunConcurrently`, alongside two Dynatrace lookups (`update.go`). The errors are joined.
- **uninstall** calls it *after* Dynatrace discovery and ignores the error (`uninstall.go`), which is the silent skip described in proposal.md.

Today the function reads the project first and the account second, and ignores the account result. Cobra prints `Error: <msg>` for every error returned from `RunE`, and `cmd.Execute` exits 1. `display.PrintInfoBox` already exists for bordered boxes.

## Goals / Non-Goals

**Goals:**

- One place decides whether `gcloud` is ready, and one place renders the guidance, shared by all three commands.
- Guidance is printed once, from the calling flow, never from a goroutine.
- Only "already reported" errors are kept off the `Error:` line. Every other error prints exactly as before.

**Non-Goals:**

- Changing how `pkg/analyzer` detects GCP. It already distinguishes "authenticated, no project" for the recommender.
- Making the box show the `setup` command as the re-run step (see Risks).

## Decisions

### 1. Typed preflight errors; callers render the box

`gcpAccountInfo` returns a typed error for each missing setting (`errNoGCloudAccount`, `errNoGCloudProject`). It does not print anything itself. Each entry point checks with `errors.Is` and calls a renderer with its verb, `reportPreflight(verb, err) error`. The renderer prints the box and returns the error that ends the command.

*Why:* `gcpAccountInfo` stays verb-agnostic and print-free, so the concurrent call in update stays safe. `errors.Is` sees through the joined error from `RunConcurrently`, so update renders once after the fan-out without restructuring.

*Alternative:* printing inside `gcpAccountInfo` would print from a goroutine in update, and the function would need to know the verb.

The renderer and box texts live in a new file, `pkg/installer/gcp/preflight_guidance.go`, following the one-concern-per-file rule.

### 2. Check order: CLI, then account, then project

The account comes from `gcloud config get-value account`, the call that already exists and whose result is currently ignored. Empty or `(unset)` means not logged in. If a `gcloud config get-value` call itself fails, that becomes a wrapped system error (`failed to read gcloud configuration: %w`) instead of the current misleading "Not logged in" text.

*Why account first:* `gcloud projects list` in the no-project box only works when logged in, so a user missing both gets the steps in the order they can follow them.

*Alternative:* `gcloud auth list --filter=status:ACTIVE`, as in the analyzer. It's more exact, because it catches revoked credentials that still have a configured account, but it adds a subprocess call. Credential problems still surface at step 1 with gcloud's own error, so the config value is enough here.

### 3. Exit signalling: `ErrAlreadyReported` and `ConfigError`, joined

A new sentinel `installer.ErrAlreadyReported` goes in `pkg/installer/errors.go`. The renderer returns:

```go
fmt.Errorf("%w: %w", installer.ErrAlreadyReported,
    &installer.ConfigError{MissingFields: []string{"gcloud.project"}}) // or "gcloud.account"
```

- `selfmonitoring.ClassifyError` already matches `*ConfigError` with `errors.As`, so the outcome is `config_error` with `config.missing_fields`. No taxonomy change is needed.
- The command handlers in `cmd/install.go`, `cmd/update.go` and `cmd/uninstall.go` need no change. The error is not `ErrInstallCancelled`, so they fire `StepFailed` and return it.

*Alternative:* returning `ErrInstallCancelled` would exit 0 and report "cancelled", which hides a real setup failure from scripts and telemetry.

### 4. Root prints errors itself

In `cmd/root.go`, set `rootCmd.SilenceErrors = true`. After `rootCmd.Execute()`, if the error is not `ErrAlreadyReported`, print it to stderr in cobra's format (`Error: <msg>`), then keep the existing `os.Exit(1)`. Usage printing is controlled separately (`SilenceUsage`) and does not change.

*Why:* cobra prints inside `Execute`, before our code sees the error, so suppressing it selectively means taking over printing. Matching cobra's prefix and stream exactly keeps every other command's output the same.

*Alternative:* a per-command `cmd.SilenceErrors = true` set right before returning. It's scattered across handlers and easy to forget for new GCP entry points.

### 5. Uninstall resolves the project before discovery

`uninstallGCPWithRunner` calls `gcpAccountInfo` before the concurrent DT lookups and stops on error. The `projectID != ""` branches in `gcpGatherServiceAccounts`, `uninstallStepCount`, the preview and `runUninstallSteps` become unreachable and are removed, so the code matches the spec instead of keeping dead tolerance paths.

### 6. Box content

Every box starts with the same line pattern: `Google Cloud <verb> not run: <reason>.` The no-project box adds a `Logged in as: <account>` line, because `gcloud projects list` only shows projects visible to that account, which helps when the user is logged in as the wrong identity. The account is already read during preflight, so the typed no-project error carries it to the renderer. Each box continues with numbered steps aligned in two columns (description, command), and the uninstall no-project box adds one explanatory paragraph. The commands are plain text in the box (no backticks), because the column layout already separates them from the prose. The re-run line is `dtwiz <verb> gcp`. Colors come from the existing `display` palette: the heading uses `ColorWarning` (stopped, not broken), labels and step descriptions use `ColorMuted`, and commands use a new `ColorEmphasis` (bold, no foreground color). `ColorBold` is not used because its fixed white foreground is unreadable on light-background terminals; bold in the terminal's own text color works on both. `fatih/color` drops them for non-TTY output and `NO_COLOR`.

### 7. Recommender hint

In `pkg/recommender/recommender.go`, the no-project `UnlockCommand` becomes `` `gcloud projects list`, then `gcloud config set project <PROJECT_ID>` ``. Rendering is unchanged.

### 8. Analyzer: account first, stdout only

`detectGCP` currently reads the project first and only runs `gcloud auth list` when no project is set. Its `runCmd` merges stdout and stderr. When logged out, `gcloud auth list --filter=status:ACTIVE --format=value(account)` writes nothing to stdout, exits 0, and prints `WARNING: The following filter keys were not present in any resource : status` to stderr. That warning was read as the account.

New order, mirroring the installer preflight:

1. `gcloud auth list --filter=status:ACTIVE --format=value(account)`, stdout only. A failure (no gcloud) or empty output means not signed in, and detection stops.
2. `gcloud config get-value project`, stdout only. Empty or `(unset)` means authenticated without a project.
3. Only then GCP counts as available, and services are probed.

The commands are run through the existing stdout-only `runCmdWithTimeout` (`detect_kubernetes.go`), injected into a `detectGCPWith(run)` function so tests can feed it canned outputs, as `probeK8sSubVariant` does. The account now comes from `auth list`, which is the active account and replaces the separate `config get-value account` read, so the number of gcloud calls stays the same when a project is set.

*Alternative:* keep `runCmd` and filter out lines starting with `WARNING:`. That's fragile against other gcloud notices, and stdout is the documented channel for `--format` output.

## Risks / Trade-offs

- [The root error-printing change touches every command] → Reproduce cobra's exact output (`Error:` prefix to stderr) and add a root-level test for both paths: a normal error is printed, an already-reported error is not.
- [`dtwiz setup` → install with no account shows `dtwiz install gcp` as the re-run command, not `dtwiz setup`] → Accepted. It's correct and runnable. Setup already hides GCP when there's no project, so only the rare "logged out, project set" case reaches it.
- [Uninstall behavior change: scripts that relied on DT-only cleanup without a project now fail] → The box states the cause and the fix. Called out as BREAKING in the proposal and the PR.
- [`unlock_command` in `recommend --json` now contains prose] → No known consumer parses it as a single command.
- [Revoked credentials with a configured account pass the account check] → Same as today. The failure surfaces at step 1 with gcloud's own error.

## Migration Plan

No data migration. Ship as one change. Rollback is a revert. The root printing change can be reverted on its own, because it reproduces cobra's default behavior.
