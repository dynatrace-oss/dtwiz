# Proposal

## Why

`dtwiz` could detect GCP environments but had no way to enable monitoring for them. Connecting Dynatrace to a GCP project is otherwise a long manual process: create a Dynatrace connection, enable the required Google Cloud APIs, create a service account, grant it Viewer on the project, grant the Dynatrace principal impersonation rights on that service account, finalize the Dynatrace connection with the service account identity, then create a monitoring configuration with the right defaults. Each step touches two systems (the `gcloud` CLI and the Dynatrace Platform API), the ordering matters, and GCP's eventual consistency (IAM propagation) means naive scripting is flaky. As a result, GCP, unlike OneAgent, Kubernetes, AWS, and Azure, fell outside the "if we detect it, we enable it" principle.

This change adds a single-command GCP Monitor integration that automates the full workflow, is safe to re-run (including resuming a previously interrupted install), and cleans up exactly the resources it created.

## What Changes

- **`dtwiz install gcp`**: new installer that runs the full 7-step workflow end-to-end using GCP service-account impersonation, so no long-lived key file is ever created or stored. Uses the Dynatrace `dtctl` SDK for Platform API calls and the `gcloud` CLI for GCP changes.
- **`dtwiz uninstall gcp`**: removes every Dynatrace connection and `da-gcp` monitoring configuration carrying the `dtwiz-gcp` name, plus the GCP service account (which cascades to its IAM impersonation binding) and its project-level Viewer role binding. Cleanup is best-effort and continues past individual step failures.
- **Update via `dtwiz setup`**: when a complete GCP connection already exists, the setup flow reconciles the monitoring configuration in place using the latest defaults and leaves the authentication chain untouched, instead of erroring. There is intentionally no `dtwiz update gcp` subcommand.
- **Resumable partial installs**: if a previous install was interrupted after creating the Dynatrace connection but before it was finalized, re-running `dtwiz install gcp` (directly or via `dtwiz setup`) reuses that connection instead of failing with a duplicate-name conflict or bouncing between install and update.
- **Zero-config defaults from the live extension schema**: monitoring feature sets are read from the current `da-gcp` extension schema at install time rather than hardcoded. Project filtering is scoped to the active `gcloud` project.
- **Resilience to GCP propagation delays**: retry loops handle IAM policy binding propagation on the `gcloud` steps and impersonation-binding propagation during Dynatrace connection finalization, with jittered backoff to avoid retry storms across concurrent installs.
- **Preflight and transparency**: checks that `gcloud` is installed and has an active project, shows a full command preview with tokens masked, and prompts for a single confirmation. `--dry-run` supported on all paths.

## Capabilities

### New Capabilities

- `gcp-monitor-install`: The 7-step `dtwiz install gcp` workflow: enable required APIs, create/resume the DT connection, create the GCP service account, grant it Viewer, grant the Dynatrace principal impersonation rights, finalize the DT connection, create the `da-gcp` monitoring configuration; includes preflight, preview, partial-install resume, and propagation-retry behavior.
- `gcp-monitor-uninstall`: `dtwiz uninstall gcp`: discovery and best-effort removal of all dtwiz-created Dynatrace and GCP resources, tolerant of partial or interrupted prior runs.
- `gcp-monitor-update`: The in-place reconcile flow reached from `dtwiz setup` when a complete GCP connection already exists: rewrites the `da-gcp` monitoring configuration to the latest schema-derived defaults (or creates it when missing) while leaving the connection, service account, and IAM bindings untouched.

## Impact

- New package `pkg/installer/gcp/`: `config.go`, `install.go`, `dtapi.go`, `helpers.go`, `preflight.go`, `uninstall.go`, `update.go` (+ tests).
- `cmd/install.go`: adds `dtwiz install gcp`.
- `cmd/uninstall.go`: adds `dtwiz uninstall gcp`.
- `cmd/setup.go`: badges the GCP entry when a complete connection is already configured, routes to update vs install (running both existence checks concurrently with Azure's), and suppresses the generic post-install watch for GCP (the installer runs it itself).
- `pkg/analyzer/detect_gcp.go`: exports `CleanGCloudConfigValue` so the installer parses `gcloud config get-value` output identically to detection (stripping the Cloud Shell "active configuration" notice line); GCP detection itself already existed.
- `pkg/recommender/recommender.go`: drops the `coming soon` framing for the GCP recommendation (removes `ComingSoon: true` and the "(coming soon)" title suffix) so GCP becomes an actionable recommendation (+ `recommender_test.go` update).
- `pkg/installer/{cmdrunner.go,concurrent.go,extension_client.go,retry.go}`: new shared utilities (command execution, concurrent error-joining fan-out, Dynatrace Settings/Extensions API client, retry-with-jitter) extracted so the Azure and GCP installers share one implementation instead of two near-duplicates; `pkg/installer/azure/*` was refactored onto these in the same change.
- No new top-level flags; reuses `--environment` / `--platform-token` and the shared `--dry-run`.
