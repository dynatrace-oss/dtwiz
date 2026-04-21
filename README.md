# dtwiz

**Dynatrace Ingest CLI** — analyzes your system and deploys the best Dynatrace observability method.

`dtwiz` is a Go CLI that analyzes your system and deploys the best Dynatrace observability method automatically.

> **Early Development**: This project is in active development. If you encounter any bugs or issues, please [file a GitHub issue](https://github.com/dynatrace-oss/dtwiz/issues/new). Contributions and feedback are welcome!

## Quickstart

Run the following commands in your terminal/console to install and launch `dtwiz`:

### Linux / macOS

```bash
export DT_ENVIRONMENT="https://<your-tenant-domain>"
export DT_ACCESS_TOKEN="dt0c01.XXXX..."
export DT_PLATFORM_TOKEN="dt0s16.XXXX..."
source <(curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh)
dtwiz setup
```

> Requires bash or zsh. Using `source <(...)` makes `dtwiz` available in your current terminal immediately — no need to open a new one.

### Windows (PowerShell)

```powershell
$env:DT_ENVIRONMENT="https://<your-tenant-domain>"
$env:DT_ACCESS_TOKEN="dt0c01.XXXX..."
$env:DT_PLATFORM_TOKEN="dt0s16.XXXX..."
irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.ps1 | iex
dtwiz setup
```

## Prerequisites

Set the following environment variables before running `dtwiz`:

| Variable | Description |
|----------|-------------|
| `DT_ENVIRONMENT` | Your Dynatrace environment URL (e.g. `https://<your-tenant-domain>`) |
| `DT_ACCESS_TOKEN` | Classic API token (`dt0c01.*`) — used for OneAgent installer download, OTel ingest, etc. |
| `DT_PLATFORM_TOKEN` | Platform token (`dt0s16.*`) — used for AWS integration and DQL log verification |

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

## Available commands

| Command | Description |
|---------|-------------|
| `dtwiz setup` | Interactive analyze → recommend → install workflow |
| `dtwiz analyze` | Detect platform, containers, K8s, existing agents, cloud, and services |
| `dtwiz recommend` | Generate ranked ingestion recommendations |
| `dtwiz install oneagent` | Install Dynatrace OneAgent on this host |
| `dtwiz install kubernetes` | Deploy Dynatrace Operator on Kubernetes |
| `dtwiz install docker` | Install OneAgent for Docker |
| `dtwiz install otel` | Install/configure OpenTelemetry Collector |
| `dtwiz install aws` | Set up Dynatrace AWS CloudFormation integration |
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

## Example workflow

```bash
# 1. Set credentials
export DT_ENVIRONMENT="https://<your-tenant-domain>"
export DT_ACCESS_TOKEN="dt0c01.XXXX..."
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

Credentials are read from `DT_ENVIRONMENT`, `DT_ACCESS_TOKEN`, and `DT_PLATFORM_TOKEN` environment variables — `dtwiz` never stores tokens itself.
