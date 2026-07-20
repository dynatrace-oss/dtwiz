# Azure Monitor — Env-Scoped Naming

## CHANGED Requirements

### Naming

All resources (Dynatrace connection, monitoring configuration, Azure App Registration) use a name derived from the Dynatrace environment URL: `dtwiz-azure-<tenant-id>`, where `<tenant-id>` is the first DNS label of the URL (e.g. `dtwiz-azure-fds1499d` for `https://fds1499d.apps.dynatracelabs.com`).

### Install (`azure-monitor-install`)

Before installing, discover connections matching the derived name. Exactly one complete (bound application ID) → delegate to update. All other states → abort with guidance to uninstall first. All seven steps use the derived name for the connection and monitoring configuration.

### Update (`azure-monitor-update`)

Account lookup runs before DT resource discovery so a login failure aborts early. Discover connections and monitoring configurations under the derived name. Require exactly one complete connection; abort with guidance to install (none found) or uninstall+reinstall (multiple found).

### Uninstall (`azure-monitor-uninstall`)

Discover all connections and monitoring configurations matching the `dtwiz-azure*` prefix to cover both the old fixed name and new env-scoped names.

App Registration cleanup is split into **current** (connection-bound IDs + display-name lookup of `dtwiz-azure-<tenant-id>`, ownership-verified by the dtwiz federated credential fingerprint) and **legacy** (display-name lookup of `dtwiz-azure` only). Current failures are fatal; legacy failures are warn-only.
