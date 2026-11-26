# Proposal

## Why

When `gcloud` has no active project, `dtwiz install gcp`, `update gcp` and `uninstall gcp` stop with a one-line error. It doesn't explain how to find a project ID or what to do next, so the user has to leave the flow and work it out alone. Uninstall is worse: it quietly removes only the Dynatrace-side resources and leaves the GCP service account and its IAM binding in place, and nothing in the output says so. The "not logged in" message is also misdiagnosed: it fires when `gcloud config get-value project` fails, which practically never happens because of a missing login. A logged-out user with a project set passes preflight and only fails later, at step 1.

## What Changes

- All three GCP commands (`install`, `update`, `uninstall`) check for an active `gcloud` account and an active project before doing anything. When either is missing, they print an info box that says the command was not run and why, followed by the exact commands to fix it:
  - no account: `gcloud auth login`, then re-run the command
  - no project: `gcloud projects list` to find the ID, `gcloud config set project <PROJECT_ID>` to select it, then re-run the command
- `--dry-run` shows the same box, because without a project no accurate preview is possible.
- These outcomes exit non-zero, are classified as `config_error` in self-monitoring, and print no extra `Error: ...` line beneath the box.
- **BREAKING** (behavior): `dtwiz uninstall gcp` no longer runs without an active project. Today it removes the Dynatrace resources and silently skips the GCP service account and Viewer binding. Now it aborts before deleting anything, and the box explains that a project-less uninstall would leave GCP resources behind.
- The `dtwiz setup` unlock hint for an authenticated user with no project becomes `` `gcloud projects list`, then `gcloud config set project <PROJECT_ID>` ``. The backticks make it clear what is a command and what is prose.
- `dtwiz setup` and `dtwiz analyze` detect a missing `gcloud` login correctly. Today a logged-out user is reported as signed in, because a gcloud warning on stderr is read as the account name, so `setup` shows the project hint instead of `gcloud auth login`. A logged-out user who still has a project configured is also reported as having GCP available. Detection now requires an active account before GCP counts as available.
- dtwiz never changes `gcloud` configuration itself. Selecting the project stays an explicit user action.

Not in scope: an interactive project picker, persisting a project on the user's behalf, or deriving the uninstall project from stored resources.

## Capabilities

### New Capabilities

- `gcp-cli-preflight`: the shared `gcloud` account and project checks for all GCP commands, the guidance box shown when one fails, and how that outcome exits.

### Modified Capabilities

- `gcp-monitor-install`: the preflight requirement now relies on `gcp-cli-preflight` (account check added, guidance box instead of a one-line error).
- `gcp-monitor-update`: a failed account or project lookup during the parallel lookup shows the guidance box instead of a plain error.
- `gcp-monitor-uninstall`: an active project becomes mandatory. The "tolerates a missing active project" requirement is removed, and the preview, deletion and step-count requirements drop their "when a project is active" conditions.
- `setup-recommendations`: the unavailable GCP entry shows a different unlock hint for "authenticated, no project" than for "not signed in", and GCP is only offered when `gcloud` has both an active account and a project.

## Impact

- `pkg/installer/gcp/`: preflight (account check, typed errors), a new guidance-box renderer, and the install/update/uninstall entry points. Uninstall resolves the project before discovery.
- `pkg/installer/errors.go`: a new shared `ErrAlreadyReported` sentinel.
- `cmd/root.go`: the root command silences cobra's own error printing and prints errors itself, skipping errors already reported. All other errors keep their current `Error: ...` output and exit code 1.
- `pkg/analyzer/detect_gcp.go`: GCP detection reads the active account from stdout only and checks it before the project. `dtwiz analyze --json` now also reports `authenticated: true` when GCP is available.
- `pkg/recommender/recommender.go`: the GCP no-project unlock hint. This text is also exported in the `unlock_command` field of `dtwiz recommend --json`.
- No new dependencies, and no feature flag: this is a UX fix to existing GCP commands.
- Rollback: revert the change. The root error-printing change is the only cross-cutting piece, and it reproduces cobra's existing output exactly, so reverting it on its own is safe.
