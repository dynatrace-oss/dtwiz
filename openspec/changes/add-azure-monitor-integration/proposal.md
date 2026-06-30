# Proposal

## Why

`dtwiz` could detect Azure environments but had no way to enable monitoring for them. Connecting Dynatrace to Azure Monitor is otherwise a long manual process: create a Dynatrace connection, register an Azure App and Service Principal, set up a federated identity credential, grant Monitoring Reader access, then create a monitoring configuration with the right settings. Each step touches two systems (the Azure CLI and the Dynatrace Platform API), the ordering matters, and Azure's eventual consistency means naive scripting is flaky. As a result, Azure — unlike OneAgent, Kubernetes, and AWS — fell outside the "if we detect it, we enable it" principle.

This change adds a single-command Azure Monitor integration that automates the full workflow, is safe to re-run, and cleans up exactly the resources it created.

## What Changes

- **`dtwiz install azure`** — new installer that runs the full 7-step workflow end-to-end using federated (workload) identity, so no client secret is ever created or stored. Uses the Dynatrace `dtctl` SDK for Platform API calls and the `az` CLI for Azure changes.
- **`dtwiz uninstall azure`** — removes every Dynatrace connection and `da-azure` monitoring configuration carrying the `dtwiz-azure` name, plus the Azure App Registration (which cascades to Service Principal and federated credentials) and its Monitoring Reader role assignment. Cleanup is best-effort and ownership-verified.
- **Update via `dtwiz setup`** — when an Azure connection already exists, the setup flow reconciles the monitoring configuration in place using the latest defaults and leaves the authentication chain untouched, instead of erroring. There is intentionally no `dtwiz update azure` subcommand.
- **Zero-config defaults from the live extension schema** — monitoring locations and feature sets are read from the current `da-azure` extension schema at install time rather than hardcoded, so defaults stay current as the extension evolves. Subscription filtering is scoped to the logged-in subscription.
- **Resilience to Azure propagation delays** — retry loops handle Service Principal propagation (step 4), federated credential propagation errors during DT connection finalization (step 6), and stale federated credential replacement (step 3).
- **Preflight and transparency** — checks that `az` is installed and logged in, runs an advisory (non-blocking) permissions check, shows a full command preview with tokens masked, and prompts for a single confirmation. `--dry-run` supported on all paths.

## Capabilities

### New Capabilities

- `azure-monitor-install`: The 7-step `dtwiz install azure` workflow — create DT connection, create Azure SP and federated credential and role assignment, finalize DT connection, create `da-azure` monitoring configuration with schema-derived defaults — including preflight, preview, and propagation-retry behavior.
- `azure-monitor-uninstall`: `dtwiz uninstall azure` — discovery and ownership-verified, best-effort removal of all dtwiz-created Dynatrace and Azure resources, tolerant of partial or interrupted prior runs.
- `azure-monitor-update`: The in-place reconcile flow reached from `dtwiz setup` when an Azure connection already exists — rewrites the `da-azure` monitoring configuration to the latest schema-derived defaults (or creates it when missing) while leaving the connection, Service Principal, federated credential, and role assignment untouched.

## Impact

- New package `pkg/installer/azure/` — `config.go`, `install.go`, `dtapi.go`, `helpers.go`, `preflight.go`, `uninstall.go`, `update.go` (+ tests).
- `cmd/install.go` — adds `dtwiz install azure`.
- `cmd/uninstall.go` — adds `dtwiz uninstall azure`.
- `cmd/setup.go` — badges the Azure entry when already configured and routes to update vs install; suppresses the generic post-install watch for Azure (the installer runs it itself).
- New dependency: `github.com/dynatrace-oss/dtctl/sdk` (`httpclient`, `api/settings`, `api/extension`).
- No changes to existing installers; Azure detection (`pkg/analyzer/detect_azure.go`) and `recommender.MethodAzure` already existed and are unchanged.
- No new top-level flags; reuses `--environment` / `--platform-token` and the shared `--dry-run`.
