# Tasks: E2E Testing Infrastructure

## 1. Test Infrastructure (`test/integration/`)

- [x] 1.1 Create `test/integration/setup.go` — `SetupIntegration(t *testing.T)` function that validates `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` are set (calls `t.Fatal` if not), constructs a `*client.Client` via `client.New()` with URLs derived from `TEST_DT_ENVIRONMENT` and tokens from `TEST_DT_ACCESS_TOKEN`/`TEST_DT_PLATFORM_TOKEN`, generates unique test ID `dtwiz-test-{unix-ts}-{random}` using `crypto/rand`, returns a `TestEnv` struct with client, test ID, temp dir (`t.TempDir()`), and raw credential values (`EnvURL`, `AccessToken`, `PlatformToken`)
- [x] 1.2 Create `test/integration/grail/` sub-package with:
  - `types.go` — `TraceRecord`, `PollOption`, `pollConfig`, `grailResponse`, path constants (`grailExecutePath`, `grailPollPath`), `WithTimeout`/`WithInterval` options
  - `execute.go` — `executeDQL`: POST to `/query:execute`; handles `SUCCEEDED` (inline records) and `RUNNING` (delegates to `pollDQL`)
  - `poll.go` — `pollDQL`: GET `/query:poll?request-token=<token>` up to 10 retries / 1s interval until `SUCCEEDED`
  - `helpers.go` — `checkDQLStatus`, `sleepOrCancel`, `tracesByServiceQuery` (builds `smartscapeNodes "SERVICE", from: -30m, to: now() | filter name == "<svcName>"`)
  - `wait.go` — `WaitForTraces(ctx, *client.Client, serviceName, ...PollOption)` outer poll loop (default 60s / 2s); `RequireTraces` wrapper that fatals on error or empty result

## 2. Fixture App (`test/fixtures/`)

- [x] 2.1 Create `test/fixtures/python-flask/app.py` — minimal Flask app with one GET endpoint (`/`) that returns a simple response; app listens on a configurable port (env var or default 18080)
- [x] 2.2 Create `test/fixtures/python-flask/requirements.txt` — contains `flask` (pinned or unpinned, minimal)

## 3. Python E2E Test (`test/e2e/`)

- [x] 3.1 Create helpers in `test/integration/` (no build tag — pure function definitions, compile harmlessly): `fixture.go` (`PrepareFixture`, `CopyFixture`), `http.go` (`TriggerRequest`, `TriggerRequestOnPort` — uses `http.Client{Timeout: 10s}` with `NewRequestWithContext`), `process.go` (`ServiceName`, `WaitForPort`, `RegisterPortCleanup`, `KillProcessOnPort`)
- [x] 3.2 Create `test/e2e/otel_test.go` with `//go:build integration` — `TestOTelAutoInstrumentation` with table-driven `otelCase` struct; for Python: checks `python3` available (skip if not), calls `SetupIntegration`, copies flask fixture via `PrepareFixture`, sets `installer.AutoConfirm = true` to suppress interactive prompts, calls `installer.InstallOtelPython` directly (installs packages AND starts the instrumented process), waits for port via `WaitForPort`, sends request via `TriggerRequestOnPort`, calls `grail.RequireTraces` (180s timeout / 20s interval override), logs trace count

## 4. makefile & Gitignore

- [x] 4.1 Add `test-integration` target to `makefile` — loads `.e2e.env` if present (top-level `ifneq (,$(wildcard .e2e.env))` / `include .e2e.env` / `export`), checks `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` are set (prints error to stderr + `exit 1` if missing), runs `go test -v -race -tags integration -timeout 5m ./test/e2e/...`
- [x] 4.2 Add `.e2e.env` to `.gitignore`
- [x] 4.3 Add `.e2e.env.example` to the repo — contains all three required env vars (`TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, `TEST_DT_PLATFORM_TOKEN`) with placeholder values and a comment instructing contributors to `cp .e2e.env.example .e2e.env` and fill in their credentials; committed to VCS
- [x] 4.4 Update `.PHONY` in `makefile` to include `test-integration`

## 5. Verification

- [x] 5.1 Run `make test`
- [x] 5.2 Run `make test-integration` without credentials — confirm descriptive error to stderr and non-zero exit
- [x] 5.3 Run `make lint` — confirm no new lint issues in added code
