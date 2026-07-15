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
dtwiz update <method>        # otel (patch existing OTel Collector config) | azure (reconcile monitoring config)
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

- `APIURL()` — strip `.apps.` → classic URL (also trims trailing slash)
- `AppsURL()` — insert `.apps.` → platform URL
- `ExtractTenantID()` — first DNS label from URL

**DQL endpoint always needs `Bearer` auth** — even for `dt0c01.*` tokens. `Api-Token` scheme → 403.

## Auth & token rules

| Token prefix | Type | Usage |
|---|---|---|
| `dt0c01.*` | API token | Classic API (`Api-Token` header) |
| `dt0s16.*` | Platform token | New API |

`AuthHeader()`: `dt0c01.*` → `Api-Token <token>`, everything else → `Bearer <token>`.

Credentials resolved from: `--environment` flag → `DT_ENVIRONMENT` env var; `--platform-token` flag → `DT_PLATFORM_TOKEN` env var. The Classic API **access token is opt-in and flag-only**: `--access-token` activates access-token auth and is intentionally **not** read from `DT_ACCESS_TOKEN`, so a leftover env var can never silently switch Classic API calls onto it. When `--access-token` is absent, the platform token is used for Classic API calls too.

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

## Development practices

### Core principles

- Write idiomatic Go (effective Go, standard project layout)
- **Cross-platform first:** always handle Unix (Linux/macOS) and Windows differences
- Test everything: unit tests and integration tests
- Use OS-specific file suffixes and build tags: `_windows.go`, `_linux.go`, `_darwin.go` are automatically recognized by Go; for "non-Windows" logic, use `_unix.go` with an explicit `//go:build !windows` constraint

### Cross-platform development

**File paths:**

```go
// ALWAYS use filepath.Join for constructing paths
agentPath := filepath.Join(installDir, "opentelemetry-javaagent.jar")

// Use os.UserHomeDir() or os.UserConfigDir() for user-specific locations
configDir, err := os.UserConfigDir()

// Environment variable path separator
func envSeparator() string {
    if runtime.GOOS == "windows" {
        return ";"
    }
    return ":"
}
```

**File permissions:**

```go
// Windows ignores Unix permissions — only chmod on Unix
if runtime.GOOS != "windows" {
    os.Chmod(scriptPath, 0755)
}
```

**Process execution & detection:**

- Unix: parse `ps` output
- Windows: use WMI (via PowerShell)
- Separate non-trivial platform divergence into build-tagged files (e.g., `_unix.go` with `//go:build !windows`, `_windows.go` with `//go:build windows`)

### Error handling

- Wrap with context: `fmt.Errorf("failed to do X: %w", err)`
- Include platform info on failures: `runtime.GOOS`, `runtime.GOARCH`
- Distinguish user errors (bad config, missing env var) from system errors (network, permissions)

### Code quality

- Run `golangci-lint` with the existing configuration (see `.golangci.yml`)
- No `//nolint` without justification comment
- Never shell out unnecessarily — use Go stdlib or libraries
- Validate all user inputs (endpoints, paths, service names)
- Use `context.Context` propagation throughout
- Handle SIGINT/SIGTERM gracefully for long-running operations
- Use `logger.Debug()` for diagnostic log lines that help with troubleshooting (process detection, API calls, config generation, etc.)

## Build & release

```sh
make build          # go build with ldflags version
make test           # go test ./...
make lint           # golangci-lint
```

Archives: `dtwiz_{version}_{os}_{arch}.tar.gz` (Linux/macOS), `.zip` (Windows).

### Release checklist

1. **Verify clean state:** `git status` — no uncommitted changes should remain after the release commit.
2. **Run tests:** `make test` — all must pass.
3. **Run lint:** `make lint` — check for new issues (pre-existing warnings are acceptable).
4. **Update `CHANGELOG.md`:**
   - Move items from `[Unreleased]` into a new `[x.y.z] - YYYY-MM-DD` section.
   - Follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format with `Added`, `Changed`, `Fixed`, `Removed` subsections as needed.
   - Update footer links: add `[x.y.z]` compare link, update `[Unreleased]` to compare against the new tag.
5. **Commit:** `git add -A && git commit -m "chore: release vx.y.z"`
6. **Tag:** `git tag vx.y.z`
7. **Push:** `git push origin main --tags`
8. **Release:** `GITHUB_TOKEN=$(gh auth token) goreleaser release --clean`
9. **Verify:** `gh release list --limit 3` — confirm the new release appears.
