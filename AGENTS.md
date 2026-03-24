# dtwiz — Agent Context

## What is dtwiz

Go CLI that analyzes a system and deploys the best Dynatrace observability method automatically. User runs `dtwiz setup` → tool detects environment → recommends ingestion method → installs it.

**Core principle:** If we detect it, we enable monitoring for it — zero config, all defaults on.

## CLI commands

```text
dtwiz setup                  # interactive: analyze → recommend → pick → install
dtwiz analyze [--json]       # detect platform, containers, K8s, agents, cloud, services
dtwiz recommend [--json]     # ranked ingestion recommendations
dtwiz status                 # connection status + system analysis
dtwiz install <method>       # oneagent | kubernetes | docker | otel | otel-python | aws | azure | gcp
dtwiz update <method>        # otel (patch existing OTel Collector config)
dtwiz uninstall <method>     # oneagent | kubernetes | otel | aws | self
```

All install/update/uninstall commands support `--dry-run`. Destructive commands show a preview and prompt for confirmation.

## Project structure

```text
main.go                         # entry point → cmd.Execute()
cmd/
  root.go                       # cobra root, persistent flags: --environment, --access-token, --platform-token
  auth.go                       # getDtEnvironment(), accessToken(), platformToken(), URL helpers
  analyze.go, recommend.go, setup.go, status.go, version.go
  install.go, update.go, uninstall.go
pkg/
  analyzer/                     # system detection (platform, Docker, K8s, OneAgent, OTel, AWS, Azure, services)
  recommender/                  # recommendation engine (priority-ranked, method-based)
  installer/                    # shared utilities (URL/token helpers, RunCommand) + per-method installers
    installer.go                # AuthHeader(), APIURL(), AppsURL(), ExtractTenantID(), RunCommand()
    oneagent.go, kubernetes.go, docker.go, otel.go, otel_update.go, otel_python.go, aws.go
    dynakube.tmpl, otel.tmpl, aws.tmpl   # embedded Go templates
    sudo_unix.go, sudo_windows.go
scripts/
  install.sh, install.ps1      # curl|sh installer scripts
```

## Dynatrace URL families (critical)

Two URL families — getting this wrong causes 404s or auth errors:

| Family | Pattern | Auth | Use for |
|---|---|---|---|
| **Classic** (no `.apps.`) | `<env-id>.<domain>/api/...` | `Api-Token dt0c01.*` | `/api/v1`, `/api/v2`, OneAgent download, OTel ingest |
| **Platform** (with `.apps.`) | `<env-id>.apps.<domain>/platform/...` | `Bearer <token>` | DQL/Grail queries, Platform APIs |

**URL conversion helpers in `pkg/installer/installer.go`:**

- `APIURL()` / `ClassicAPIURL()` — strip `.apps.` → classic URL
- `AppsURL()` — insert `.apps.` → platform URL
- `ExtractTenantID()` — first DNS label from URL

**DQL endpoint always needs `Bearer` auth** — even for `dt0c01.*` tokens. `Api-Token` scheme → 403.

## Auth & token rules

| Token prefix | Type | Usage |
|---|---|---|
| `dt0c01.*` | API token | Classic API (`Api-Token` header) |
| `dt0s16.*` | Platform token | New API |

`AuthHeader()`: `dt0c01.*` → `Api-Token <token>`, everything else → `Bearer <token>`.

Credentials resolved from: `--environment`/`--access-token`/`--platform-token` flags → `DT_ENVIRONMENT`/`DT_ACCESS_TOKEN`/`DT_PLATFORM_TOKEN` env vars.

## Key design rules

- **Zero-config defaults:** OneAgent full-stack mode, K8s `cloudNativeFullStack`, AWS all services + all regions.

## CLI conventions

- **Args validation:** All leaf commands must set `Args: cobra.NoArgs`. Parent commands with required subcommands use `Args: cobra.MinimumNArgs(1)`.
- **`--dry-run` pattern:** Defined once as a `PersistentFlags().BoolVar()` on the parent command (`installCmd`, `updateCmd`, `uninstallCmd`), shared by all subcommands via a package-level variable.
- **Verb-noun command tree:** Top-level verbs are `install`, `update`, `uninstall`. Methods are subcommands (`dtwiz install otel`, `dtwiz update otel`). New operations get their own verb — don't nest verbs under `install`.

## UX: transparency before execution

Before running any block of commands or applying changes, always show the user a compact preview:

- Print the commands or actions that will run — one line each, no noise.
- If a config file is generated or modified, print its full contents inline.
- End with a single confirmation prompt: `Apply? [Y/n]` — default is **yes** (Enter = proceed).

**Reduce to the max:** Surface only what the user needs to stay informed. Omit internal details, progress spam, and redundant labels. Every line of output must earn its place — if it doesn't help the user understand what happened or what went wrong, cut it.

## Build & release

```sh
make build          # go build with ldflags version
make test           # go test ./...
make lint           # golangci-lint

# Release:
git tag v0.x.y && git push origin v0.x.y
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

Archives: `dtwiz_{version}_{os}_{arch}.tar.gz` (Linux/macOS), `.zip` (Windows).
