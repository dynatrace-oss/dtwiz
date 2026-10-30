---
name: dtwiz
description: Deploy Dynatrace observability with dtwiz — analyze systems, get ranked recommendations, and install monitoring (OneAgent, Kubernetes, Docker, OTel, AWS, Azure, GCP). Use when the user wants to set up, manage, or ingest data into Dynatrace monitoring. Covers ingesting metrics, logs, and traces from AWS, Azure, GCP, Kubernetes, containers, or hosts into Dynatrace via dtwiz. Also use when the user asks to onboard this machine, connect it to Dynatrace, or get data into Dynatrace for the first time.
---

# Dynatrace Setup with dtwiz

Go CLI that detects your environment and deploys the best Dynatrace monitoring method automatically. Core principle: if we detect it, we enable monitoring for it — zero config, all defaults on.

## Onboarding a new user

When the user wants to onboard a machine for the first time, read and follow
[ONBOARDING.md](ONBOARDING.md) (raw URL:
`https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/skills/dtwiz/ONBOARDING.md`). It
covers the guided, step-by-step flow and how to handle the platform token without it entering
the chat. The rest of this file is the command reference.

## Token handling (always)

- Never ask the user for their platform token in the chat and never accept one there. If one
  is pasted, tell the user to revoke it and create a new one.
- Never print the token, write it to a file, or pass it as `--platform-token`. Use the
  `DT_PLATFORM_TOKEN` environment variable, set by the user in their own terminal with hidden
  input.

## Prerequisites

- `dtwiz` installed:
  - macOS/Linux: `source <(curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh)`
  - Windows (PowerShell): `irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.ps1 | iex`
- Dynatrace environment URL + platform token (`dt0s16.*`)
- Auth via flags or env vars:

```bash
export DT_ENVIRONMENT=https://<your-tenant-domain>
export DT_PLATFORM_TOKEN=dt0s16.****
```

Or pass per-command: `--environment`, `--platform-token`.

The environment URL may be given with or without `https://`. The Classic API access token (`dt0c01.*`) is deprecated; ignore it.

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
| --------- | ------------- |
| `dtwiz setup` | Interactive: analyze → recommend → pick → install |
| `dtwiz analyze [--json]` | Detect platform, containers, K8s, agents, cloud, services |
| `dtwiz recommend [--json]` | Ranked ingestion recommendations based on analysis |
| `dtwiz status` | Connection status + system analysis |
| `dtwiz install <method>` | Install a specific ingestion method |
| `dtwiz update <method>` | Reconcile an existing configuration (`azure`, `gcp`; `otel` is experimental) |
| `dtwiz uninstall <method>` | Remove an installed method (`oneagent`, `kubernetes`, `otel`, `aws`, `aws-lambda`, `azure`, `gcp`, `self`) |
| `dtwiz watch [--from <time>]` | Poll Dynatrace and show a live ingest summary |
| `dtwiz version` | Print the dtwiz version |

All `install`, `update`, and `uninstall` commands support `--dry-run` (preview only) and `--yes`/`-y` (skip confirmation prompts). After a successful install, dtwiz watches until new data arrives.

## Install Methods

| Method | Command | What it does |
| -------- | --------- | ------------- |
| **OneAgent** | `dtwiz install oneagent` | Full-stack host monitoring |
| **Kubernetes** | `dtwiz install kubernetes` | Deploy Dynatrace Operator with a DynaKube (application monitoring, Kubernetes monitoring, log monitoring) |
| **Docker** (experimental) | `dtwiz install docker` | OneAgent for Docker containers; needs `--experimental` or `DTWIZ_EXPERIMENTAL=true` |
| **OTel** | `dtwiz install otel` | OTel Collector + application instrumentation |
| **OTel Collector** | `dtwiz install otel-collector` | OTel Collector only (no app instrumentation) |
| **OTel Java** | `dtwiz install otel-java` | OpenTelemetry Java auto-instrumentation |
| **OTel Python** | `dtwiz install otel-python` | OpenTelemetry Python auto-instrumentation |
| **OTel Node.js** | `dtwiz install otel-node` | OpenTelemetry Node.js auto-instrumentation |
| **AWS** | `dtwiz install aws` | CloudFormation integration for the current AWS account and region |
| **AWS Lambda** | `dtwiz install aws-lambda` | Dynatrace Lambda Layer on all functions |
| **Azure** | `dtwiz install azure` | Azure Monitor integration |
| **GCP** | `dtwiz install gcp` | Google Cloud Platform integration |

## Key Concepts

### Dry Run

Preview any install/update/uninstall without executing. (The guided onboarding flow in
[ONBOARDING.md](ONBOARDING.md) deliberately skips this; use it for everything else.)

```bash
dtwiz install otel --dry-run
dtwiz uninstall oneagent --dry-run
```

### Zero-Config Defaults

- OneAgent: full-stack mode
- Kubernetes: DynaKube with application, Kubernetes and log monitoring
- AWS: default feature sets, current account and region

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

**Connection or auth errors:**

- Run `dtwiz status` and check the environment URL and platform token. If the token is expired or wrong, create a new one on the quickstart page (see [ONBOARDING.md](ONBOARDING.md)).

## Safety Reminders

- Use `--dry-run` before destructive operations
- Without `--yes`, installs show a preview and prompt `Apply? [Y/n]` before executing
- Without `--yes`, uninstall commands show what will be removed and require confirmation
