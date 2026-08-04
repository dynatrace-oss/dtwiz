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
dtwiz watch [--from <time>]  # poll Dynatrace every 5s and display live ingest summary
dtwiz install <method>       # oneagent | kubernetes | otel | otel-collector | aws | aws-lambda | azure | gcp
dtwiz update <method>        # otel (patch existing OTel Collector config) | azure | gcp (reconcile monitoring config)
dtwiz uninstall <method>     # oneagent | kubernetes | otel | aws | aws-lambda | azure | gcp | self
```

All install/update/uninstall commands support `--dry-run` and `--yes`/`-y` (skip confirmation prompts). Destructive commands show a preview and prompt for confirmation.

**Hidden/experimental methods** (require `--experimental` or `DTWIZ_EXPERIMENTAL=true`): `install docker`, `install demo`, `update otel`.

**Post-install watch:** After a successful (non-dry-run) install, most commands automatically call `WatchIngest()` to poll until new data appears.

## Project structure

```text
main.go                         # entry point → cmd.Execute()
cmd/
  root.go                       # cobra root, persistent flags; setupClientFromCreds(), setupClient()
  auth.go                       # getDtEnvironment(), accessToken(), platformToken(), URL helpers
  analyze.go, recommend.go, setup.go, status.go, version.go, watch.go
  install.go, update.go, uninstall.go
pkg/
  analyzer/                     # system detection (platform, Docker, K8s, OneAgent, OTel, AWS, Azure, services)
  client/                       # typed Dynatrace HTTP client: Client, ClassicClient, PlatformClient
  display/                      # centralized output formatting: Header(), PrintSectionDivider(), colors
  extensions/                   # Dynatrace Extensions v2 API (install, monitoring configs)
  featureflags/                 # feature flag registry: AllRuntimes, Experimental; CLI flags + env vars
  logger/                       # structured logging: Init(debug, verbosity), Debug(), Verbose()
  recommender/                  # recommendation engine (priority-ranked, method-based)
  installer/                    # shared cross-cutting utilities only — no method logic here
    installer.go                # AuthHeader(), APIURL(), AppsURL(), ExtractTenantID(), RunCommand()
                                # ErrInstallCancelled, ShouldProceed(), ConfirmProceed(), AutoConfirm
    cmdrunner.go                # CmdRunner type; RealRunner, ExecLookPath (stubbable in tests)
    concurrent.go               # RunConcurrently() — fan-out goroutines, joins errors
    retry.go                    # retry helpers for transient cloud CLI errors
    ingest_watch.go             # WatchIngest(), IngestTimeFormat
    self_uninstall_unix.go, self_uninstall_windows.go
    sudo_unix.go, sudo_windows.go
    aws/                        # AWS CloudFormation + Lambda install/uninstall
    azure/                      # Azure Monitor install/update/uninstall + DT API helpers
    gcp/                        # GCP integration install/update/uninstall + DT API helpers
    kubernetes/                 # Dynatrace Operator install/uninstall; dynakube.tmpl
    oneagent/                   # OneAgent download, install, uninstall, connectivity check
    otel/                       # OTel Collector install/update/uninstall; otel.tmpl

  **Folder-per-method rule:** every `dtwiz install <method>` maps to a subfolder under `installer/`
  (e.g. `install otel` → `installer/otel/`, `install kubernetes` → `installer/kubernetes/`). The
  root `installer/` package contains only shared utilities used across methods.
scripts/
  install.sh, install.ps1      # curl|sh installer scripts
```

## Dynatrace URL families (critical)

Two URL families — getting this wrong causes 404s or auth errors:

| Family | Pattern | Auth | Use for |
|---|---|---|---|
| **Classic** (no `.apps.`) | `<env-id>.<domain>/api/...` | `Bearer <platform-token>` | `/api/v1`, `/api/v2`, OneAgent download, OTel ingest |
| **Platform** (with `.apps.`) | `<env-id>.apps.<domain>/platform/...` | `Bearer <platform-token>` | DQL/Grail queries, Platform APIs |

**URL conversion helpers in `pkg/installer/installer.go`:**

- `APIURL()` — strip `.apps.` → classic URL (also trims trailing slash)
- `AppsURL()` — insert `.apps.` → platform URL
- `ExtractTenantID()` — first DNS label from URL

**DQL endpoint always needs `Bearer` auth.**

## Auth & token rules

Only the **platform token** (`dt0s16.*`) is used. The Classic API access token (`dt0c01.*`) is deprecated and supported only for legacy environments via the opt-in `--access-token` flag — it is intentionally **not** read from `DT_ACCESS_TOKEN`.

`AuthHeader()`: `dt0c01.*` → `Api-Token <token>`, everything else → `Bearer <token>`.

Credentials resolved from: `--environment` flag → `DT_ENVIRONMENT` env var; `--platform-token` flag → `DT_PLATFORM_TOKEN` env var. When `--access-token` is absent (the default), the platform token is used for all API calls including the Classic API.

## Key design rules

- **Prefer dtctl/SDK over custom code:** Before implementing a Dynatrace API call or integration helper from scratch, check whether the Dynatrace SDK or `dtctl` already provides the capability. Prefer library calls over raw HTTP requests to avoid reimplementing auth, pagination, retries, and error handling.

## External reference docs

Use these docs as implementation references when changing related installers, specs, or tests. Prefer product docs over assumptions when behavior, required environment variables, package names, permissions, or setup flows are unclear. If a linked doc conflicts with existing code or specs, do not silently follow either one — update the spec or implementation deliberately and call out the behavior change.

Dynatrace docs pages may fail through generic webpage extraction. To read them, use the integrated browser page reader first; if that is unavailable, fetch the raw HTML with `curl -L` and strip/search the content locally.

| Area | Reference | Use for |
|---|---|---|
| Node.js OTel | https://docs.dynatrace.com/docs/ingest-from/opentelemetry/walkthroughs/nodejs | Node.js package setup, auto-instrumentation launch behavior, and Dynatrace OTLP environment variables |
| Python OTel | https://docs.dynatrace.com/docs/ingest-from/opentelemetry/walkthroughs/python/python-auto | Python auto-instrumentation packages, launch conventions, and Dynatrace OTLP environment variables |
| Java OTel | https://docs.dynatrace.com/docs/ingest-from/opentelemetry/walkthroughs/java/java-auto | Java agent setup, launch flags, and Dynatrace OTLP environment variables |
| Kubernetes | https://docs.dynatrace.com/docs/ingest-from/setup-on-k8s/deployment/platform-observability | Dynatrace Operator platform-observability deployment flow and Kubernetes prerequisites |
| AWS | https://docs.dynatrace.com/docs/ingest-from/amazon-web-services/create-an-aws-connection/aws-connection-api-cli | AWS connection setup, required permissions, and CLI/API behavior |
| Azure | https://docs.dynatrace.com/docs/ingest-from/microsoft-azure-services/create-an-azure-connection/azure-connection-cli | Azure connection setup, required permissions, and CLI behavior |
| GCP | https://docs.dynatrace.com/docs/ingest-from/google-cloud-platform/create-a-gcp-connection#deploy-in-gcp | GCP connection deployment, required permissions, and setup flow |

## CLI conventions

- **Args validation:** All leaf commands must set `Args: cobra.NoArgs`. Parent commands with required subcommands use `Args: cobra.MinimumNArgs(1)`.
- **`--dry-run` pattern:** Defined once as a `PersistentFlags().BoolVar()` on the parent command (`installCmd`, `updateCmd`, `uninstallCmd`), shared by all subcommands via a package-level variable.
- **`--yes`/`-y` auto-confirm:** Same pattern as `--dry-run` — one `PersistentFlags().BoolVarP()` per verb group, sets `installer.AutoConfirm`. When true, `ConfirmProceed()` returns immediately without prompting.
- **`-v` / `--debug` flags:** Defined on root. `logger.Init(debugFlag, verbosityFlag)` is called in `PersistentPreRun` on root and reproduced in every `PersistentPreRun` that overrides root's.
- **Feature flags:** Registered in `pkg/featureflags/`. Applied via `featureflags.ApplyCLIOverrides(cmd.Flags())` in `PersistentPreRun`. Each flag has a CLI name (e.g. `--experimental`), an env var (e.g. `DTWIZ_EXPERIMENTAL`), and a default. Currently registered: `AllRuntimes`, `Experimental`.
- **`ErrInstallCancelled` sentinel:** Installers return `installer.ErrInstallCancelled` when the user declines the confirmation prompt. Command handlers must check `errors.Is(err, installer.ErrInstallCancelled)` and treat it as a clean exit (return nil).
- **`ShouldProceed(dryRun, verb)` helper:** Standard gate at the end of every install/update/uninstall preview. Handles dry-run short-circuit and the confirmation prompt in one call. Returns `(proceed bool, err error)` — on decline returns `(false, ErrInstallCancelled)`.
- **Typed API client:** Commands that call Dynatrace APIs create a `*client.Client` via `setupClientFromCreds(envURL, classicTok, platformTok)` or `setupClient()` (resolves+validates credentials internally). The client exposes `.Classic` and `.Platform` sub-clients backed by `resty`.
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

## Specs & change workflow (openspec)

When the user talks about "specs", "a spec", "writing a spec", or "a change" they mean the **openspec** workflow under `openspec/`. Use the right skill — don't improvise:

| User intent | Skill to invoke |
|---|---|
| Explore / think through an idea | `openspec-explore` → `/opsx:explore` |
| Propose a new change (design + tasks) | `openspec-propose` → `/opsx:propose` |
| Implement tasks from an existing change | `openspec-apply-change` → `/opsx:apply` |
| Finalize and archive a completed change | `openspec-archive-change` → `/opsx:archive` |

**Key paths:**

- Active changes: `openspec/changes/<name>/` — contains `proposal.md`, `design.md`, `tasks.md`
- Archived changes: `openspec/changes/archive/`
- Standalone specs (no implementation yet): `openspec/specs/<name>/`

**When to create a spec:** any non-trivial feature or architectural decision should go through `openspec-propose` before implementation begins. For small, clearly scoped bug fixes or documentation edits, skip it.

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
