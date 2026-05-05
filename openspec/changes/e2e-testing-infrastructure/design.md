# Context

dtwiz currently has unit tests only (`make test` runs `go test ./pkg/...`). All installer tests mock `RunCommand`, meaning the actual shell execution, package installation, and DT API communication are never exercised. The `pkg/client/` package provides a dual HTTP client (`ClassicClient` + `PlatformClient`) built on resty, with a single `New()` constructor that takes pre-resolved URLs and tokens. There is no test-specific client constructor.

The sibling project dtctl uses a similar E2E pattern (build tags, env gating, cleanup tracker, fixtures) but keeps E2E tests local-only — not in CI. We follow the same approach.

## Goals / Non-Goals

**Goals:**

- Reusable E2E test infrastructure that future runtime tests (Node, Java, Go) can plug into with minimal effort
- One proven E2E test (Python OTel auto-instrumentation) that exercises the full install → run → verify-traces → cleanup cycle
- Clear separation: `make test` stays fast, `make test-integration` is opt-in and requires credentials
- DQL-based trace verification against a real Dynatrace tenant

**Non-Goals:**

- Node.js, Java, or Go runtime E2E tests (follow-up changes)
- OTel Collector, Kubernetes, or cloud provider E2E tests
- CI/CD pipeline integration
- Performance or load testing
- Changes to existing unit tests or test infrastructure

## Decisions

### 1. Build tag (`//go:build integration`) for test separation

**Choice:** Compile-time file gating via build tag — used as an exclusion mechanism for default runs (`go test ./...` skips E2E files) and as an inclusion mechanism for explicit integration runs (`go test -tags integration`). Whether the tag acts as exclusion or inclusion depends on the invocation; both perspectives are valid and the tag enables both.

**Alternatives considered:**

- *Env var gating with `t.Skip()`* — simpler, but tests still compile into the binary, appear as SKIP in output, and add parse-time overhead.
- *`-run` flag convention* — fragile, not enforced, easy to forget.
- *Separate Go module in `test/`* — heavyweight, adds module maintenance burden.

**Rationale:** Build tags are the standard Go idiom. `go test ./...` never touches E2E files. Zero compile overhead. Matches dtctl's proven pattern.

The tag applies only to files in `test/e2e/`, not to helper packages in `test/integration/`. Helpers are pure function definitions — they compile harmlessly without the tag and are only callable from tagged test files. Tagging them would add noise without enforcing any meaningful constraint.

### 2. `TEST_*` env vars with `.e2e.env` file support

**Choice:** `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` env vars, with two supported loading mechanisms (in precedence order):

1. `make VAR=value` command-line overrides (highest — always win in Make)
2. `.e2e.env` file in the project root, loaded via `include .e2e.env` at Make parse time (overrides shell env vars)

The makefile conditionally includes `.e2e.env` at the top level:

```makefile
ifneq (,$(wildcard .e2e.env))
include .e2e.env
export
endif
```

This runs at parse time — no subshell concerns, no chaining required. The `export` directive propagates all Make variables (including those from the file) into recipe subshells. The file uses plain `KEY=VALUE` syntax so developers can also `source .e2e.env` directly in their shell.

Missing credentials produce a clear, actionable error to stderr — no silent skip.

**Alternatives considered:**

- *Shell-based loader in recipe (`export $(grep ...)`)* — avoids Make syntax in the file and gives shell vars higher precedence, but requires all recipe steps to be chained in a single subshell invocation to prevent export loss. More fragile and harder to read.
- *Reusing `DT_ENVIRONMENT`/`DT_ACCESS_TOKEN`/`DT_PLATFORM_TOKEN`* — risk of accidentally running tests against a production tenant configured in the user's shell.
- *Config file (YAML/JSON)* — overengineered for three values.

**Rationale:** `include` is simpler and has no subshell gotchas. File-takes-precedence-over-shell is acceptable here: the `.e2e.env` file IS the intended credential source for this target; shell vars being overridden is expected and even desirable (guards against stray prod credentials in the environment). `make VAR=value` still allows one-off overrides when needed. All three vars are required: `TEST_DT_ACCESS_TOKEN` (`dt0c01.*`) for Classic API and installer operations; `TEST_DT_PLATFORM_TOKEN` (`dt0s16.*`) for DQL trace queries via `PlatformClient`. A single `TEST_DT_ENVIRONMENT` URL is sufficient — Classic and Platform URLs are both derived from it internally via `APIURL()` and `AppsURL()`.

### 3. Use `New()` directly in E2E tests

**Choice:** E2E tests call `pkg/client/client.New()` directly, reading `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` in `SetupIntegration` and deriving Classic + Platform URLs via the existing `APIURL()`/`AppsURL()` helpers.

**Alternatives considered:**

- *`NewForTesting(t *testing.T)` constructor in `client.go`* — imports `testing` in production code, which is a Go anti-pattern; adds a constructor that exists solely to wrap URL derivation logic already available via helpers.
- *Separate test client package* — duplicates resty setup, diverges over time.

**Rationale:** Using `New()` directly keeps production code free of test-only dependencies and exercises the same HTTP client path used in production, making E2E tests more representative.

### 4. `t.TempDir()` for isolation (no CleanupTracker)

**Choice:** Each test creates a temp directory for its fixture app. Go's `t.TempDir()` auto-cleans on test completion, even on failure.

**Alternatives considered:**

- *dtctl-style CleanupTracker with LIFO deletion* — needed for API resources with dependencies (workflows, dashboards). dtwiz E2E tests create local filesystem artifacts and short-lived processes, not persistent API resources.

**Rationale:** Auto-instrumentation tests modify a temp project directory, not the host system. The instrumented process is killed when the test ends. No persistent resources to track.

### 5. DQL polling for trace verification

**Choice:** Query the DT Grail/DQL API for traces matching the unique test service name. Implemented as a dedicated `test/integration/grail/` sub-package. Poll with a configurable interval and timeout.

**DQL query format:** Uses a `smartscapeNodes` entity-level query rather than a span-level `fetch spans` query:

`smartscapeNodes "SERVICE", from: -30m, to: now() | filter name == "<svcName>"`

This queries entity nodes by service name within a 30-minute window. Filtering by `service.name` at the span level caused DQL to return 0 records even when spans were ingested; the entity-level query proved reliable.

**Async execute/poll flow:** The DQL API returns a `RUNNING` state with a `requestToken` for long-running queries instead of immediate results. The `grail/` package implements a two-step flow:

1. POST to `/platform/storage/query/v1/query:execute` — returns either `SUCCEEDED` (records inline) or `RUNNING` (+ `requestToken`)
2. If `RUNNING`, poll GET `/platform/storage/query/v1/query:poll?request-token=<token>` until `SUCCEEDED` (up to `dqlPollMaxRetries=10` attempts, 1s between retries)

The outer `WaitForTraces` loop then re-runs the full execute call at the configured interval until traces appear or the timeout is exceeded.

DQL requests are made via `PlatformClient.HTTP()` (resty) so the Bearer token stays encapsulated and resty's retry logic applies.

Default poll config is 60s timeout / 2s interval. The Python E2E test overrides to 180s / 20s to account for OTel pipeline latency and ingestion time.

**Alternatives considered:**

- *Local-only verification (check installed packages, env vars)* — misses the most important assertion: did traces actually reach Dynatrace?
- *Classic API v2 metrics/traces endpoints* — DQL is the current standard for Grail-based environments.
- *Exposing `PlatformClient.AuthHeader()` for raw `net/http` requests* — leaks the token as a plain string; using resty via `.HTTP()` keeps auth encapsulated and gets retry logic for free.
- *Single-file `grail_client.go`* — the async execute/poll logic, types, and helpers warranted a dedicated sub-package to keep concerns separated.

**Rationale:** The whole point of E2E is verifying the data pipeline end-to-end. The PlatformClient already has Bearer auth and the `.apps.` URL needed for DQL. The entity-level `smartscapeNodes` query proved more reliable than span-level queries. Polling handles ingestion latency naturally.

### 6. Unique service naming: `dtwiz-test-{unix-ts}-{random}`

**Choice:** Each test run generates a unique service name used as `OTEL_SERVICE_NAME`. Format: `dtwiz-test-{unix-ts}-{random}-{lang}` where the random suffix is generated with `crypto/rand` (hex-encoded). DQL queries filter by this name.

**Rationale:** Prevents conflicts between parallel test runs or leftover data from previous runs. The timestamp component aids debugging (map test run to traces). `crypto/rand` guarantees uniqueness even for runs started within the same second.

### 7. `make test-integration` fails on missing credentials

**Choice:** makefile target checks `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` before invoking `go test`. If any is missing, print an error to stderr and `exit 1`.

**Alternatives considered:**

- *`t.Skip()` in Go* — exit 0 masks misconfiguration; developer thinks tests passed when they didn't run.

**Rationale:** `make test-integration` is an explicit opt-in. If the developer runs it, they expect it to work. Silent skipping defeats the purpose.

### 8. Fixtures in `test/fixtures/` (sibling to `test/e2e/`)

**Choice:** Static fixture apps live in `test/fixtures/<runtime>/` (e.g., `test/fixtures/python-flask/`). Test helpers copy them into `t.TempDir()` at runtime.

**Rationale:** Keeps fixture code separate from test logic. Future runtimes add a new directory under `test/fixtures/` without touching existing tests. Copying into temp dir ensures isolation.

## Risks / Trade-offs

- **[Tenant dependency]** → E2E tests require a live Dynatrace tenant with `openTelemetryTrace.ingest` and DQL query scopes on the token. Mitigation: clear error message with required scopes when auth fails.
- **[Python availability]** → Python E2E test requires Python 3 + pip on the test machine. Mitigation: test checks for `python3` and skips with a descriptive message if missing.
- **[Ingestion latency]** → Traces may take 5-30 seconds to appear in DQL results. Mitigation: configurable poll timeout (default 60s / 2s interval); Python E2E test uses 180s / 5s override to account for OTel pipeline warmup.
- **[Flakiness from DT API]** → DQL endpoint could be temporarily unavailable. Mitigation: resty retry (already in client) + generous timeout. Accept that E2E tests are inherently less stable than unit tests.
- **[No CI]** → E2E tests only run locally. Risk of regressions between releases. Mitigation: document in CONTRIBUTING.md that `make test-integration` should be run before releasing installer changes. CI integration is a follow-up.
