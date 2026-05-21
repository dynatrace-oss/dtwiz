# Testing

## Table of Contents

- [Running Tests](#running-tests)
- [Writing Unit Tests](#writing-unit-tests)
- [Writing Integration Tests](#writing-integration-tests)
- [Mocking Conventions](#mocking-conventions)

## Running Tests

```bash
# Run unit tests (with coverage)
make test

# Run unit tests and enforce the coverage threshold
make test-coverage

# Run a specific package
go test ./pkg/recommender/...

# Run a specific test function
go test -run TestGenerateRecommendations_Kubernetes ./pkg/recommender/

# Run with race detection
go test -race ./pkg/...
```

Integration tests require a live Dynatrace environment and are opt-in:

```bash
# Copy the template and fill in your credentials
cp .e2e.env.example .e2e.env

# Run integration tests
make test-integration
```

## Writing Unit Tests

Unit tests live next to the code they test (`pkg/foo/foo_test.go`).

**Naming:** `TestFunctionName_Scenario` — e.g., `TestGenerateRecommendations_Kubernetes`.

**Package:** use the `_test` suffix (`package recommender_test`) so tests only access exported symbols.

**Structure:** prefer table-driven tests for multiple scenarios:

```go
func TestAuthHeader(t *testing.T) {
    tests := []struct {
        name  string
        token string
        want  string
    }{
        {
            name:  "API token gets Api-Token scheme",
            token: "dt0c01.abc",
            want:  "Api-Token dt0c01.abc",
        },
        {
            name:  "platform token gets Bearer scheme",
            token: "dt0s16.xyz",
            want:  "Bearer dt0s16.xyz",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := installer.AuthHeader(tt.token)
            if got != tt.want {
                t.Errorf("AuthHeader(%q) = %q, want %q", tt.token, got, tt.want)
            }
        })
    }
}
```

Use `httptest.NewServer` when testing code that makes HTTP calls — no real network needed:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, `{"status":"ok"}`)
}))
defer srv.Close()
```

Use `t.TempDir()` for any files written during a test — Go cleans it up automatically after the test.

## Writing Integration Tests

Integration tests live in `test/e2e/` and carry the `//go:build integration` build tag. They run against a real Dynatrace environment and verify that installer commands actually work end-to-end.

**Required environment variables:**

| Variable | Description |
|---|---|
| `TEST_DT_ENVIRONMENT` | Your Dynatrace environment URL |
| `TEST_DT_ACCESS_TOKEN` | Classic API token (`dt0c01.*`) |
| `TEST_DT_PLATFORM_TOKEN` | Platform token (`dt0s16.*`) |

Set these in `.e2e.env` (gitignored). `make test-integration` loads the file automatically.

**Test setup:** call `integration.SetupIntegration(t)` at the start of each test. It validates the environment variables, creates a shared `client.Client`, and returns a `TestEnv` with a unique `TestID` and a `t.TempDir()` for scratch files.

```go
//go:build integration

package e2e_test

import "github.com/dynatrace-oss/dtwiz/test/integration"

func TestMyInstaller(t *testing.T) {
    env := integration.SetupIntegration(t)

    svcName := env.TestID + "-my-svc"
    // call installer functions, pass env.EnvURL / env.AccessToken / env.PlatformToken
    // use env.Client to query Dynatrace and assert the expected state
}
```

**Fixtures:** sample applications for language-specific OTel tests live in `test/fixtures/`. Each fixture is a minimal runnable app. Use them as the target directory when testing `install otel-*` commands.

**Assertions:** use `test/integration/grail` helpers to query Dynatrace via DQL and verify that telemetry actually arrived. Poll with a timeout rather than sleeping — the `grail.WaitFor*` helpers handle this.

## Mocking Conventions

The following test helpers are defined in `pkg/installer/testhelpers_test.go` and `pkg/installer/otel_test.go`. Use them instead of reimplementing the same patterns.

| Helper | Use when |
|---|---|
| `setTestWorkingDir(t, dir)` | Code under test uses `os.Getwd()` as a scan root (project detection functions) |
| `setTestStdin(t, input)` | Code under test prompts the user for input |
| `captureStdout(t, fn)` | Asserting on printed output |
| `managedProcessHelperCommand(t, mode)` | Testing code that spawns and manages subprocesses |

Both `setTestWorkingDir` and `captureStdout` hold a package-level mutex (`cwdMu` / `stdoutMu`) for the duration of the test because `os.Chdir` and `os.Stdout` are process-global. Tests using these helpers must not run in parallel.

For **binary availability** (e.g. `java`, `node` on PATH), use `t.Setenv("PATH", "")` to simulate the binary being absent, or point PATH at a temp directory containing a stub script.

For **HTTP calls**, use `httptest.NewServer` — pass `srv.URL` to the function under test instead of a real Dynatrace URL. See `pkg/installer/oneagent_test.go` for examples covering success, auth failure, and unexpected status codes.
