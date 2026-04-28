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

### 2. `TEST_*` env vars with `.env` file support

**Choice:** `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` env vars, loadable from `.e2e-tests.env` via Makefile.

**Alternatives considered:**

- *Reusing `DT_ENVIRONMENT`/`DT_ACCESS_TOKEN`* — risk of accidentally running tests against a production tenant configured in the user's shell.
- *Config file (YAML/JSON)* — over-engineered for two values.

**Rationale:** `TEST_` prefix makes intent explicit and prevents accidental production use. `.e2e-tests.env` file avoids repeated exports. Makefile loads it with `include .e2e-tests.env` guarded by `ifneq (,$(wildcard .e2e-tests.env))`. Missing credentials produce a clear error to stderr and exit non-zero — no silent skip.

### 3. `NewForTesting()` on `pkg/client/`

**Choice:** Add a `NewForTesting(t *testing.T)` constructor that reads `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN`, calls `t.Fatal` if missing, and returns a `*Client` with verbosity off.

**Alternatives considered:**

- *Separate test client package* — duplicates resty setup, diverges over time.
- *Calling `New()` directly in tests* — requires tests to replicate URL family logic (Classic vs Platform).

**Rationale:** Keeps the real client as the single source of truth. `NewForTesting` handles URL family derivation internally (test env URL → Classic URL + Platform URL). The `testing.T` parameter makes the intent clear and ensures failures are reported through the test framework.

### 4. `t.TempDir()` for isolation (no CleanupTracker)

**Choice:** Each test creates a temp directory for its fixture app. Go's `t.TempDir()` auto-cleans on test completion, even on failure.

**Alternatives considered:**

- *dtctl-style CleanupTracker with LIFO deletion* — needed for API resources with dependencies (workflows, dashboards). dtwiz E2E tests create local filesystem artifacts and short-lived processes, not persistent API resources.

**Rationale:** Auto-instrumentation tests modify a temp project directory, not the host system. The instrumented process is killed when the test ends. No persistent resources to track.

### 5. DQL polling for trace verification

**Choice:** Query the DT Grail/DQL API (`/platform/storage/query/v1/query:execute`) for traces matching the unique test service name. Poll with configurable interval and timeout.

**Alternatives considered:**

- *Local-only verification (check installed packages, env vars)* — misses the most important assertion: did traces actually reach Dynatrace?
- *Classic API v2 metrics/traces endpoints* — DQL is the current standard for Grail-based environments.

**Rationale:** The whole point of E2E is verifying the data pipeline end-to-end. The PlatformClient already has Bearer auth and the `.apps.` URL needed for DQL. Polling handles ingestion latency naturally.

### 6. Unique service naming: `dtwiz-test-{unix-ts}-{random}`

**Choice:** Each test run generates a unique service name used as `OTEL_SERVICE_NAME`. DQL queries filter by this name.

**Rationale:** Prevents conflicts between parallel test runs or leftover data from previous runs. The timestamp component aids debugging (map test run to traces).

### 7. `make test-integration` fails on missing credentials

**Choice:** Makefile target checks `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` before invoking `go test`. If either is missing, print an error to stderr and `exit 1`.

**Alternatives considered:**

- *`t.Skip()` in Go* — exit 0 masks misconfiguration; developer thinks tests passed when they didn't run.

**Rationale:** `make test-integration` is an explicit opt-in. If the developer runs it, they expect it to work. Silent skipping defeats the purpose.

### 8. Fixtures in `test/fixtures/` (sibling to `test/e2e/`)

**Choice:** Static fixture apps live in `test/fixtures/<runtime>/` (e.g., `test/fixtures/python-flask/`). Test helpers copy them into `t.TempDir()` at runtime.

**Rationale:** Keeps fixture code separate from test logic. Future runtimes add a new directory under `test/fixtures/` without touching existing tests. Copying into temp dir ensures isolation.

## Risks / Trade-offs

- **[Tenant dependency]** → E2E tests require a live Dynatrace tenant with `openTelemetryTrace.ingest` and DQL query scopes on the token. Mitigation: clear error message with required scopes when auth fails.
- **[Python availability]** → Python E2E test requires Python 3 + pip on the test machine. Mitigation: test checks for `python3` and skips with a descriptive message if missing.
- **[Ingestion latency]** → Traces may take 5-30 seconds to appear in DQL results. Mitigation: configurable poll timeout (default 60s), 2s poll interval.
- **[Flakiness from DT API]** → DQL endpoint could be temporarily unavailable. Mitigation: resty retry (already in client) + generous timeout. Accept that E2E tests are inherently less stable than unit tests.
- **[No CI]** → E2E tests only run locally. Risk of regressions between releases. Mitigation: document in CONTRIBUTING.md that `make test-integration` should be run before releasing installer changes. CI integration is a follow-up.
