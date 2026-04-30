# Tasks: E2E Testing Infrastructure

## 1. Test Infrastructure (`test/integration/`)

- [x] 1.1 Create `test/integration/setup.go` — `SetupIntegration(t *testing.T)` function that validates `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` are set (calls `t.Fatal` if not), constructs a `*client.Client` via `client.New()` with URLs derived from `TEST_DT_ENVIRONMENT` and tokens from `TEST_DT_ACCESS_TOKEN`/`TEST_DT_PLATFORM_TOKEN`, generates unique test ID `dtwiz-test-{unix-ts}-{random}`, returns a `TestEnv` struct with client, test ID, and temp dir (`t.TempDir()`)
- [x] 1.2 Create `test/integration/grail_client.go` — `WaitForTraces(ctx context.Context, client *client.Client, serviceName string, opts ...PollOption) ([]TraceRecord, error)` function that polls DQL endpoint via `PlatformClient` filtering by `service.name`, default 60s timeout / 2s interval, returns traces or timeout error with the queried service name

## 2. Fixture App (`test/fixtures/`)

- [x] 2.1 Create `test/fixtures/python-flask/app.py` — minimal Flask app with one GET endpoint (`/`) that returns a simple response; app listens on a configurable port (env var or default 18080)
- [x] 2.2 Create `test/fixtures/python-flask/requirements.txt` — contains `flask` (pinned or unpinned, minimal)

## 3. Python E2E Test (`test/e2e/`)

- [x] 3.1 Create `test/integration/helpers.go` with `//go:build integration` — exported helpers in package `integration`: `CopyFixture(t, fixtureDir, destDir)` to copy fixture app into temp dir, `StartApp(t, dir, port)` to run the instrumented app as a subprocess with `t.Cleanup` kill, `TriggerRequest(url)` HTTP GET helper
- [x] 3.2 Create `test/e2e/python_test.go` with `//go:build integration` — `TestPythonOTelAutoInstrumentation`: checks `python3` available (skip if not), calls `SetupIntegration`, copies flask fixture via `integration.CopyFixture`, runs `dtwiz install otel-python` on the temp dir, starts the app with `opentelemetry-instrument` via `integration.StartApp`, sends HTTP request via `integration.TriggerRequest`, calls `WaitForTraces` with unique service name, asserts trace count > 0

## 4. makefile & Gitignore

- [x] 4.1 Add `test-integration` target to `makefile` — loads `.e2e.env` if present (top-level `ifneq (,$(wildcard .e2e.env))` / `include .e2e.env` / `export`), checks `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` are set (prints error to stderr + `exit 1` if missing), runs `go test -v -tags integration -timeout 5m ./test/e2e/...`
- [x] 4.2 Add `.e2e.env` to `.gitignore`
- [x] 4.3 Add `.e2e.env.example` to the repo — contains the three required env vars (`TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN`) with placeholder values and a comment instructing contributors to `cp .e2e.env.example .e2e.env` and fill in their credentials; committed to VCS
- [x] 4.4 Update `.PHONY` in `makefile` to include `test-integration`

## 5. Verification

- [ ] 5.1 Run `make test`
- [ ] 5.2 Run `make test-integration` without credentials — confirm descriptive error to stderr and non-zero exit
- [ ] 5.3 Run `make lint` — confirm no new lint issues in added code
