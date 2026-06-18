# dtwiz

**Dynatrace Ingest CLI** — analyzes your system and deploys the best Dynatrace observability method.

`dtwiz` is a Go CLI that analyzes your system and deploys the best Dynatrace observability method automatically.

> **Early Development**: This project is in active development. If you encounter any bugs or issues, please [file a GitHub issue](https://github.com/dynatrace-oss/dtwiz/issues/new). Contributions and feedback are welcome!

## Quickstart

Run the following commands in your terminal/console to install and launch `dtwiz`:

### Linux / macOS

```bash
export DT_ENVIRONMENT="https://<your-tenant-domain>"
export DT_PLATFORM_TOKEN="dt0s16.XXXX..."
source <(curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh)
dtwiz setup
```

> Requires bash or zsh. Using `source <(...)` makes `dtwiz` available in your current terminal immediately — no need to open a new one.

### Windows (PowerShell)

```powershell
$env:DT_ENVIRONMENT="https://<your-tenant-domain>"
$env:DT_PLATFORM_TOKEN="dt0s16.XXXX..."
irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.ps1 | iex
dtwiz setup
```

## Prerequisites

Set the following environment variables before running `dtwiz`:

| Variable | Description |
|----------|-------------|
| `DT_ENVIRONMENT` | Your Dynatrace environment URL (e.g. `https://<your-tenant-domain>`) |
| `DT_PLATFORM_TOKEN` | Platform token (`dt0s16.*`) — primary credential; used for Platform/DQL and (by default) Classic API calls |

For legacy environments where the platform token lacks Classic API access, opt into a Classic API token (`dt0c01.*`) by passing `--access-token` explicitly. It is intentionally **not** read from `DT_ACCESS_TOKEN` — so a leftover env var can never silently change which token authenticates Classic API calls.

## Installation

**Linux / macOS:**

```bash
source <(curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh)
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.ps1 | iex
```

**From source:**

```bash
git clone https://github.com/dynatrace-oss/dtwiz.git
cd dtwiz
make install
```

### Install channels

| Channel | Linux / macOS | Windows (PowerShell) |
|---------|---------------|----------------------|
| Latest stable release *(default)* | `source <(curl -sSL .../install.sh)` | `irm .../install.ps1 \| iex` |
| Latest `main` branch snapshot | `DTWIZ_CHANNEL=main source <(curl -sSL .../install.sh)` | `$env:DTWIZ_CHANNEL="main"; irm .../install.ps1 \| iex` |
| Specific PR/branch snapshot | `DTWIZ_TAG=snapshot-<branch> source <(curl -sSL .../install.sh)` | `$env:DTWIZ_TAG="snapshot-<branch>"; irm .../install.ps1 \| iex` |

The `main` channel is rebuilt automatically on every push to `main` and is intended for testing unreleased changes. It is not recommended for production use.

## Available commands

| Command | Description |
|---------|-------------|
| `dtwiz setup` | Interactive analyze → recommend → install workflow |
| `dtwiz analyze` | Detect platform, containers, K8s, existing agents, cloud, and services |
| `dtwiz recommend` | Generate ranked ingestion recommendations |
| `dtwiz install oneagent` | Install Dynatrace OneAgent on this host |
| `dtwiz install kubernetes` | Deploy Dynatrace Operator on Kubernetes |
| `dtwiz install docker` | Install Dynatrace OneAgent for Docker |
| `dtwiz install otel` | Install/configure OTel Collector and instrument your application |
| `dtwiz install otel-collector` | Install the Dynatrace OpenTelemetry Collector only |
| `dtwiz install otel-java` | Instrument a Java project with OpenTelemetry |
| `dtwiz install otel-node` | Instrument a Node.js project with OpenTelemetry |
| `dtwiz install otel-python` | Instrument a Python project with OpenTelemetry |
| `dtwiz install demo` | Download the "schnitzel" demo app and instrument it end-to-end |
| `dtwiz install aws` | Set up Dynatrace AWS CloudFormation integration |
| `dtwiz install aws-lambda` | Install Dynatrace Lambda Layer on all functions |
| `dtwiz install azure` | Set up Dynatrace Azure Monitor integration *(coming soon)* |
| `dtwiz install gcp` | Set up Dynatrace Google Cloud Platform integration *(coming soon)* |
| `dtwiz uninstall oneagent` | Uninstall Dynatrace OneAgent from this host |
| `dtwiz uninstall kubernetes` | Remove Dynatrace Operator and DynaKube resources from Kubernetes |
| `dtwiz uninstall otel` | Kill running OTel Collector processes and remove installation files |
| `dtwiz uninstall aws` | Remove the Dynatrace AWS CloudFormation stack and monitoring configuration |
| `dtwiz uninstall self` | Remove the dtwiz binary and its PATH entry |
| `dtwiz update otel` | Patch an existing OTel Collector config with the Dynatrace exporter |
| `dtwiz watch` | Live-watch for newly ingested data in Dynatrace (services, logs, traces, etc.) |
| `dtwiz status` | Show Dynatrace connection status and system state |

## Flags

### `--yes` / `-y`

Skip all confirmation prompts and apply changes automatically. Available on all `install`, `update`, and `uninstall` subcommands.

```bash
dtwiz install otel --yes
dtwiz uninstall oneagent -y
```

### `--project <path>`

Point `install otel`, `install otel-python`, `install otel-java`, or `install otel-node` at a specific project directory instead of scanning interactively.

```bash
dtwiz install otel --project ./my-service
dtwiz install otel-python --project ./my-python-app
```

## Demo

`dtwiz install demo` sets up a complete end-to-end observability demo in one command:

1. Downloads and extracts the [schnitzel](https://github.com/dietermayrhofer/schnitzel) demo app — a 4-service Python application — into `./schnitzel/` in your current directory.
2. Installs Python if not present (via `brew` on macOS, `apt` on Debian/Ubuntu, `dnf` on RHEL/Fedora, `winget` on Windows).
3. Instruments the app with OpenTelemetry and starts sending traces, metrics, and logs to your Dynatrace environment.

```bash
dtwiz install demo          # interactive confirmation before applying
dtwiz install demo --yes    # skip confirmation
```

## Example workflow

```bash
# 1. Set credentials
export DT_ENVIRONMENT="https://<your-tenant-domain>"
export DT_PLATFORM_TOKEN="dt0s16.XXXX..."

# 2. Analyze the current system
dtwiz analyze

# 3. Get ranked recommendations
dtwiz recommend

# 4. Install the recommended method (e.g., Kubernetes)
dtwiz install kubernetes

# 5. Check status
dtwiz status
```

## JSON output

`analyze` and `recommend` support `--json` for structured output:

```bash
dtwiz analyze --json | jq .platform
dtwiz recommend --json | jq '.[0].method'
```

## Building

```bash
cd dtwiz
make build        # builds ./dtwiz binary
make test         # runs go test ./...
make install      # installs to $GOPATH/bin
make clean        # removes build artifacts
```

## Architecture

```text
dtwiz/
├── main.go
├── cmd/
│   ├── root.go       # Cobra root + persistent flags
│   ├── auth.go       # credential resolution (getDtEnvironment, accessToken, platformToken)
│   ├── analyze.go
│   ├── recommend.go
│   ├── setup.go
│   ├── install.go
│   └── status.go
└── pkg/
    ├── analyzer/     # System detection (platform, Docker, K8s, agents, cloud, services)
    ├── recommender/  # Recommendation engine
    └── installer/    # Shared utilities + per-method stubs
```

Credentials are read from the `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN` environment variables (or the `--environment` / `--platform-token` flags); a legacy Classic API access token is supplied only via the explicit `--access-token` flag. `dtwiz` never stores tokens itself.
