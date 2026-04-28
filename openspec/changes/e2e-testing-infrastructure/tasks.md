# Tasks: E2E Testing Infrastructure

## 1. Client Test Constructor (`pkg/client/`)

- [ ] 1.1 Add `NewForTesting(t *testing.T) *Client` to `pkg/client/client.go` — reads `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN`, calls `t.Fatal` if either is missing, derives Classic + Platform URLs from the single env URL, returns `*Client` with verbosity 0
- [ ] 1.2 Add unit test for `NewForTesting` in `pkg/client/client_test.go` — test cases: both vars set (success), missing environment (fatal), missing token (fatal), Classic URL input derives correct Platform URL, Platform URL input derives correct Classic URL

## 2. Test Infrastructure (`test/integration/`)

- [ ] 2.1 Create `test/integration/setup.go` — `SetupE2ETest(t *testing.T)` function that validates `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` are set (calls `t.Fatal` if not), generates unique test ID `dtwiz-test-{unix-ts}-{random}`, returns a `TestEnv` struct with client (`NewForTesting`), test ID, and temp dir (`t.TempDir()`)
- [ ] 2.2 Create `test/integration/dtquery.go` — `WaitForTraces(ctx context.Context, client *client.Client, serviceName string, opts ...PollOption) ([]TraceRecord, error)` function that polls DQL endpoint via `PlatformClient` filtering by `service.name`, default 60s timeout / 2s interval, returns traces or timeout error with the queried service name

## 3. Fixture App (`test/fixtures/`)

- [ ] 3.1 Create `test/fixtures/python-flask/app.py` — minimal Flask app with one GET endpoint (`/`) that returns a simple response; app listens on a configurable port (env var or default 18080)
- [ ] 3.2 Create `test/fixtures/python-flask/requirements.txt` — contains `flask` (pinned or unpinned, minimal)

## 4. Python E2E Test (`test/e2e/`)

- [ ] 4.1 Create `test/e2e/suite_test.go` with `//go:build integration` — shared test helpers: `copyFixture(t, fixtureDir, destDir)` to copy fixture app into temp dir, `startApp(t, dir, port)` to run the instrumented app as a subprocess with `t.Cleanup` kill, `triggerRequest(url)` HTTP GET helper
- [ ] 4.2 Create `test/e2e/python_test.go` with `//go:build integration` — `TestPythonOTelAutoInstrumentation`: checks `python3` available (skip if not), calls `SetupE2ETest`, copies flask fixture, runs `dtwiz install otel-python` on the temp dir, starts the app with `opentelemetry-instrument`, sends HTTP request, calls `WaitForTraces` with unique service name, asserts trace count > 0

## 5. Makefile & Gitignore

- [ ] 5.1 Add `test-integration` target to `Makefile` — loads `.e2e-tests.env` if present (`ifneq (,$(wildcard .e2e-tests.env))` / `include .e2e-tests.env` / `export`), checks `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` are set (prints error to stderr + `exit 1` if missing), runs `go test -v -tags integration -timeout 5m ./test/e2e/...`
- [ ] 5.2 Add `.e2e-tests.env` to `.gitignore`
- [ ] 5.3 Add `.e2e-tests.env.example` to the repo — contains the two required env vars (`TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN`) with placeholder values, committed to VCS as a reference for contributors
- [ ] 5.4 Update `.PHONY` in `Makefile` to include `test-integration`

## 6. Verification

- [ ] 6.1 Run `make test` — confirm it completes in ~2 sec with no E2E test output
- [ ] 6.2 Run `make test-integration` without credentials — confirm descriptive error to stderr and non-zero exit
- [ ] 6.3 Run `make lint` — confirm no new lint issues in added code
