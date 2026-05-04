# Why

dtwiz unit tests mock all shell execution (`RunCommand`) and external dependencies, which means the most failure-prone code paths — real package installations, actual trace ingestion, DT API auth — are never exercised. The Python OTel auto-instrumentation installer alone has ~150 lines of `Execute()` logic (venv detection, pip install, opentelemetry-bootstrap) that is entirely mocked in tests. We need an E2E test foundation that verifies the full install → instrument → trace-ingestion → cleanup cycle against a real Dynatrace tenant.

## What Changes

- Add `test/e2e/` directory with `//go:build integration` build tag for compile-time separation from unit tests
- Add `test/integration/` with shared setup (env gating, unique naming) and DQL trace polling helper
- Add `test/fixtures/` with reusable test app scaffolds (starting with a minimal Flask app)
- E2E tests call `client.New()` directly with URLs derived from `TEST_DT_ENVIRONMENT` via `APIURL()`/`AppsURL()` helpers and tokens from `TEST_DT_ACCESS_TOKEN`/`TEST_DT_PLATFORM_TOKEN` — no test-specific constructor needed
- Add `make test-integration` makefile target that loads `.e2e.env` if present and runs integration tests; fails with a descriptive error to stderr if credentials are missing
- Add `.e2e.env` to `.gitignore`
- Add `.e2e.env.example` to VCS with placeholder values for all three vars (`TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, `TEST_DT_PLATFORM_TOKEN`)
- Implement one real E2E test: Python OTel auto-instrumentation lifecycle (install → run instrumented app → verify traces in DT via DQL → cleanup via `t.TempDir()`)

## Capabilities

### New Capabilities

- `e2e-test-infra`: Shared E2E test infrastructure — env var gating (`TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, `TEST_DT_PLATFORM_TOKEN`), unique test naming, `t.TempDir()` isolation, `.e2e.env` loading via makefile, `make test-integration` target, direct `client.New()` construction with URL derivation via `APIURL()`/`AppsURL()`, and DQL-based trace polling helper

### Modified Capabilities

_None — all changes are in `test/` and `makefile`. `pkg/client/` is unchanged._

## Impact

- **`pkg/client/`**: No changes — E2E tests use `PlatformClient.HTTP()` (the existing resty accessor) for DQL requests. Token stays encapsulated inside the client.
- **`makefile`**: New `test-integration` target. Existing `test` target unchanged.
- **`.gitignore`**: `.e2e.env` added. `.e2e.env.example` added (committed to VCS).
- **Test runtime**: `make test` stays fast (~2 sec). `make test-integration` requires a live DT tenant and takes 30-60 sec.
- **Dependencies**: No new Go dependencies. Test fixtures require Python 3 + pip available on the test machine.
