# GCP Monitor — Env-Scoped Naming

## CHANGED Requirements

### Naming

All resources (Dynatrace connection, monitoring configuration, GCP service account) use a name derived from the Dynatrace environment URL: `dtwiz-gcp-<tenant-id>`, where `<tenant-id>` is the first DNS label of the URL (e.g. `dtwiz-gcp-fds1499d` for `https://fds1499d.apps.dynatracelabs.com`).

### Install (`gcp-monitor-install`)

Before installing, discover connections matching the derived name and classify each as complete (bound service-account email) or incomplete. Exactly one complete → redirect to update. Exactly one incomplete → resume by reusing its object ID. All other states → abort with guidance. All seven steps use the derived name for the connection, service account, and monitoring configuration.

### Update (`gcp-monitor-update`)

Discover connections and monitoring configurations under the derived name. Require exactly one complete connection; abort with guidance to install (none found) or uninstall+reinstall (multiple found).

### Uninstall (`gcp-monitor-uninstall`)

Discover all connections and monitoring configurations matching the `dtwiz-gcp*` prefix to cover both the old fixed name and new env-scoped names.

Service-account cleanup is split into **current** (env-scoped email + any connection-bound emails) and **legacy** (the fixed `dtwiz-gcp@<project>` email when not already in the current set). Current failures are fatal; legacy failures are warn-only.
