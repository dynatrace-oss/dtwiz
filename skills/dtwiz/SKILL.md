---
name: dtwiz
description: Deploy Dynatrace observability with dtwiz — analyze systems, get ranked recommendations, and install monitoring (OneAgent, Kubernetes, Docker, OTel, AWS, Azure, GCP). Use when the user wants to set up, manage, or ingest data into Dynatrace monitoring. Covers ingesting metrics, logs, and traces from AWS, Azure, GCP, Kubernetes, containers, or hosts into Dynatrace via dtwiz.
---

# Dynatrace Setup with dtwiz

Go CLI that detects your environment and deploys the best Dynatrace monitoring method automatically. Core principle: if we detect it, we enable monitoring for it — zero config, all defaults on.

## Prerequisites

- `dtwiz` installed (`curl -sfL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh | sh`)
- Dynatrace environment URL + API token
- Auth via flags or env vars:

```bash
export DT_ENVIRONMENT=https://<your-tenant-domain>
export DT_ACCESS_TOKEN=dt0c01.****
export DT_PLATFORM_TOKEN=dt0s16.****    # optional, needed for AWS installer
```

Or pass per-command: `--environment`, `--access-token`, `--platform-token`.

## Recommended Initialization

At the start of a task, run these checks to establish context:

```bash
# Verify connection and see system analysis
dtwiz status

# Machine-readable system detection
dtwiz analyze --json

# Ranked recommendations
dtwiz recommend --json
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `dtwiz setup` | Interactive: analyze → recommend → pick → install |
| `dtwiz analyze [--json]` | Detect platform, containers, K8s, agents, cloud, services |
| `dtwiz recommend [--json]` | Ranked ingestion recommendations based on analysis |
| `dtwiz status` | Connection status + system analysis |
| `dtwiz install <method>` | Install a specific ingestion method |
| `dtwiz update <method>` | Patch an existing method configuration |
| `dtwiz uninstall <method>` | Remove an installed method |
| `dtwiz version` | Print the dtwiz version |

All `install`, `update`, and `uninstall` commands support `--dry-run`.

## Install Methods

| Method | Command | What it does |
|--------|---------|-------------|
| **OneAgent** | `dtwiz install oneagent` | Full-stack host monitoring |
| **Kubernetes** | `dtwiz install kubernetes` | Deploy Dynatrace Operator (cloudNativeFullStack) |
| **Docker** | `dtwiz install docker` | OneAgent for Docker containers |
| **OTel** | `dtwiz install otel` | OTel Collector + application instrumentation |
| **OTel Collector** | `dtwiz install otel-collector` | OTel Collector only (no app instrumentation) |
| **OTel Java** | `dtwiz install otel-java` | OpenTelemetry Java auto-instrumentation |
| **OTel Python** | `dtwiz install otel-python` | OpenTelemetry Python auto-instrumentation |
| **AWS** | `dtwiz install aws` | CloudFormation integration (all services, all regions) |
| **Azure** | `dtwiz install azure` | Azure Monitor integration |
| **GCP** | `dtwiz install gcp` | Google Cloud Platform integration |

## Key Concepts

### URL Families

Two URL families — using the wrong one causes 404s or auth errors:

| Family | Pattern | Auth header | Use for |
|--------|---------|-------------|---------|
| **Classic** | `<env-id>.<domain>/api/...` | `Api-Token dt0c01.*` | `/api/v1`, `/api/v2`, OneAgent download, OTel ingest |
| **Platform** | `<env-id>.apps.<domain>/platform/...` | `Bearer <token>` | DQL/Grail queries, Platform APIs |

### Token Types

| Prefix | Type | Auth header |
|--------|------|-------------|
| `dt0c01.*` | API token | `Api-Token <token>` |
| `dt0s16.*` | Platform token | `Bearer <token>` |

### Dry Run

Preview any install/update/uninstall without executing:

```bash
dtwiz install otel --dry-run
dtwiz uninstall oneagent --dry-run
```

### Zero-Config Defaults

- OneAgent: full-stack mode
- Kubernetes: `cloudNativeFullStack`
- AWS: all services + all regions

## Common Workflows

```bash
# Full interactive setup (recommended for first-time use)
dtwiz setup

# Analyze only — no install
dtwiz analyze

# Install a specific method with preview first
dtwiz install otel --dry-run
dtwiz install otel

# Patch an existing OTel Collector config with Dynatrace exporter
dtwiz update otel

# Remove OneAgent
dtwiz uninstall oneagent
```

## Common Issues

**Cloud/K8s not detected:**
- Kubernetes → connect to a cluster with `kubectl` first
- AWS → sign in with `aws configure`
- Azure → sign in with `az login`
- GCP → sign in with `gcloud auth login`

**404 errors:**
- Wrong URL family. Classic endpoints must not have `.apps.` in the URL. Platform endpoints must.

**403 errors:**
- Wrong auth header. DQL/Platform endpoints need `Bearer` auth, even for `dt0c01.*` tokens. Classic API endpoints need `Api-Token` auth.

**Token prefix mismatch:**
- `dt0c01.*` = API token (Classic API)
- `dt0s16.*` = Platform token (new APIs)

## Safety Reminders

- Use `--dry-run` before destructive operations
- All installs show a preview and prompt `Apply? [Y/n]` before executing
- Uninstall commands show what will be removed and require confirmation
