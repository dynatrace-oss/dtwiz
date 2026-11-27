# Design

## Context

All three GCP entry points resolve `gcloud` state through one function, `gcpAccountInfo` in `pkg/installer/gcp/preflight.go`, but each uses it differently:

- **install** calls it first and in sequence (`install.go`). When a complete connection exists, install delegates to the update flow, which calls it again.
- **update** calls it inside `installer.RunConcurrently`, alongside two Dynatrace lookups (`update.go`). The errors are joined.
- **uninstall** calls it *after* Dynatrace discovery and ignores the error (`uninstall.go`), which is the silent skip described in proposal.md.

Today the function reads the project first and the account second, and ignores the account result. Cobra prints `Error: <msg>` for every error returned from `RunE`, and `cmd.Execute` exits 1. `display.PrintInfoBox` already exists for bordered boxes.

## Goals / Non-Goals

**Goals:**

- One place decides whether `gcloud` is ready, shared by system detection and all three commands, and one place renders the guidance.
- A deleted or inaccessible project must not leave the Dynatrace resources impossible to remove.
- Guidance is printed once, from the calling flow, never from a goroutine.
- Only "already reported" errors are kept off the `Error:` line. Every other error prints exactly as before.

**Non-Goals:**

- Detecting expired or reauth-required credentials. That needs a network call (`gcloud auth print-access-token`), which is slower and can prompt.
- Making the box show the `setup` command as the re-run step (see Risks).

## Decisions

### 1. Typed preflight errors; callers render the box

`gcpAccountInfo` returns a typed error for each missing setting (`errNoGCloudAccount`, `errNoGCloudProject`). It does not print anything itself. Each entry point checks with `errors.Is` and calls a renderer with its verb, `reportPreflight(verb, err) error`. The renderer prints the box and returns the error that ends the command.

*Why:* `gcpAccountInfo` stays verb-agnostic and print-free, so the concurrent call in update stays safe. `errors.Is` sees through the joined error from `RunConcurrently`, so update renders once after the fan-out without restructuring.

*Alternative:* printing inside `gcpAccountInfo` would print from a goroutine in update, and the function would need to know the verb.

The renderer and box texts live in a new file, `pkg/installer/gcp/preflight_guidance.go`, following the one-concern-per-file rule.

### 2. Check order: CLI, then account, then project

Account and project are read by the shared `analyzer.ReadGCloudIdentity` (decision 8). An empty account means not logged in. If a read itself fails, that becomes a wrapped system error (`failed to read gcloud configuration: %w`) instead of the current misleading "Not logged in" text.

*Why account first:* `gcloud projects list` in the no-project box only works when logged in, so a user missing both gets the steps in the order they can follow them.

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

### 8. One identity read for detection and preflight: `gcloud auth list`, stdout only

`detectGCP` currently reads the project first and only runs `gcloud auth list` when no project is set. Its `runCmd` merges stdout and stderr. When logged out, `gcloud auth list --filter=status:ACTIVE --format=value(account)` writes nothing to stdout, exits 0, and prints `WARNING: The following filter keys were not present in any resource : status` to stderr. That warning was read as the account. The installer preflight used a different check, `gcloud config get-value account`, so `setup` and `install` could disagree about the same machine.

A new `analyzer.ReadGCloudIdentity(read)` in `pkg/analyzer/gcloud_identity.go` is the single read for both:

1. `gcloud auth list --filter=status:ACTIVE --format=value(account)`, stdout only, first non-empty line. Empty means not logged in, and the project is not read.
2. `gcloud config get-value project`, stdout only, with the Cloud Shell notice stripped. Empty or `(unset)` means no project.
3. A failing read returns what was resolved so far plus the error.

Callers adapt it: detection maps the result to `Authenticated`/`Available` and only then probes services; the preflight maps it to `errNoGCloudAccount`/`noProjectError` and turns a read failure into a system error. `read` is a stdout-only `func(args ...string) (string, error)`: detection wraps the existing `runCmdWithTimeout` (`detect_kubernetes.go`) through an injectable `detectGCPWith(run)`, the preflight wraps its `cmdRunner` (`installer.RealRunner` already returns stdout only). It lives in `pkg/analyzer` because `pkg/installer/gcp` already imports it; the reverse would be an import cycle. `CleanGCloudConfigValue` becomes package-private.

*Why `auth list` over `config get-value account`:* `core/account` is only a setting. It stays set after `gcloud config set account x@y` without a login, or through `CLOUDSDK_CORE_ACCOUNT`, so the preflight passed and the install failed at step 1 with an auth error. `auth list` reports an account as ACTIVE only when credentials for it are stored. Both are local reads with no network call; neither catches expired credentials (see Non-Goals).

*Alternative:* keep `runCmd` and filter out lines starting with `WARNING:`. That's fragile against other gcloud notices, and stdout is the documented channel for `--format` output.

### 9. `dtwiz uninstall gcp --dynatrace-only`

Uninstall needs an active account and project to remove the GCP service account and its binding (decision 5). When the project was deleted, or the user lost access to it, that can never succeed, and without an escape hatch the Dynatrace connection and monitoring configuration could not be removed with dtwiz at all.

A local flag `--dynatrace-only` on `dtwiz uninstall gcp` (name exported as `gcp.DynatraceOnlyFlag`, so the command and the box can't drift apart) routes to `uninstallDynatraceOnly` in `pkg/installer/gcp/uninstall_dynatrace_only.go`. It:

- does not run `gcloud` at all, so neither the CLI nor a login is needed;
- discovers monitoring configurations and connections concurrently, as the full uninstall does;
- previews them, plus a warning that GCP resources stay in place, listing the service-account emails bound to the discovered connections (the email contains the project, so the user can still clean up if the project exists);
- reuses the existing step runner with no service-account steps, so step numbering, best-effort deletion and `--dry-run`/`--yes` behave as in the full uninstall.

Both uninstall guidance boxes (no account, no project) end with `Project deleted or no longer accessible? Remove only the Dynatrace side:` and the command. Install and update boxes don't show it.

*Why a flag rather than falling back automatically:* a missing project is usually a forgotten `gcloud config set project`, and an automatic fallback would bring back the silent partial cleanup this change removes. The flag makes leaving GCP resources behind an explicit choice.

*Alternative:* detect a deleted project (e.g. `gcloud projects describe` returning NOT_FOUND) and offer the fallback. It still needs a login and access, which is exactly what's missing when the user lost access.

## Risks / Trade-offs

- [The root error-printing change touches every command] → Reproduce cobra's exact output (`Error:` prefix to stderr) and add a root-level test for both paths: a normal error is printed, an already-reported error is not.
- [`dtwiz setup` → install with no account shows `dtwiz install gcp` as the re-run command, not `dtwiz setup`] → Accepted. It's correct and runnable. Setup already hides GCP when there's no project, so only the rare "logged out, project set" case reaches it.
- [Uninstall behavior change: scripts that relied on DT-only cleanup without a project now fail] → The box states the cause, the fix, and `--dynatrace-only` for a deliberate DT-only cleanup. Called out as BREAKING in the proposal and the PR.
- [Project deleted but still selected in `gcloud`] → Preflight passes; the service-account steps fail and are reported, while the best-effort runner still deletes the Dynatrace resources. Re-running with `--dynatrace-only` then reports nothing left to remove.
- [`unlock_command` in `recommend --json` now contains prose] → No known consumer parses it as a single command.
- [Expired credentials pass the account check] → Same as today. The failure surfaces at step 1 with gcloud's own error, and `setup` flags it through the service probes (`ServicesAuthError`).

## Migration Plan

No data migration. Ship as one change. Rollback is a revert. The root printing change can be reverted on its own, because it reproduces cobra's default behavior.
