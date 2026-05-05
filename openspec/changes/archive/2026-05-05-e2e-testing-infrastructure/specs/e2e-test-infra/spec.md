# E2E Test Infrastructure

## ADDED Requirements

### Requirement: Build tag separation

All E2E test files in `test/e2e/` SHALL use the `//go:build integration` build tag. The tag acts as a gating mechanism: it excludes E2E files from default builds (`go test ./...` without `-tags integration`) and includes them when explicitly requested (`go test -tags integration`). Whether it operates as exclusion or inclusion depends on the invocation — both are intentional behaviors of the same tag.

Helper files in `test/integration/` SHALL NOT carry the build tag. They are pure function definitions with no `init()` side effects; they compile harmlessly in any build and are only reachable from tagged test files in `test/e2e/`. Adding the tag to helpers is unnecessary and would add noise without benefit.

#### Scenario: Default test run excludes E2E

- **WHEN** a developer runs `go test ./...` or `make test`
- **THEN** no files under `test/e2e/` are compiled or executed

#### Scenario: Integration tag includes E2E

- **WHEN** a developer runs `go test -tags integration ./test/e2e/...`
- **THEN** all E2E test files are compiled and executed

### Requirement: Environment variable gating

The `make test-integration` target SHALL require `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` to be set. If any is missing, the target SHALL print a descriptive error message to stderr and exit with a non-zero status code.

#### Scenario: Missing TEST_DT_ENVIRONMENT

- **WHEN** `make test-integration` is run without `TEST_DT_ENVIRONMENT` set
- **THEN** an error message naming the missing variable is printed to stderr and the process exits non-zero

#### Scenario: Missing TEST_DT_ACCESS_TOKEN

- **WHEN** `make test-integration` is run without `TEST_DT_ACCESS_TOKEN` set
- **THEN** an error message naming the missing variable is printed to stderr and the process exits non-zero

#### Scenario: Missing TEST_DT_PLATFORM_TOKEN

- **WHEN** `make test-integration` is run without `TEST_DT_PLATFORM_TOKEN` set
- **THEN** an error message naming the missing variable is printed to stderr and the process exits non-zero

#### Scenario: All variables set

- **WHEN** `make test-integration` is run with `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` set
- **THEN** `go test -tags integration ./test/e2e/...` is executed

### Requirement: .e2e.env file loading

The `make test-integration` target SHALL support two credential loading mechanisms, in precedence order:

1. `make VAR=value` command-line overrides (highest — always win in Make regardless of file content)
2. `.e2e.env` file in the project root, loaded via `include .e2e.env` at Make parse time
3. Shell environment variables (lowest — overridden by `.e2e.env` assignments)

The makefile SHALL conditionally include `.e2e.env` only when the file exists:

```makefile
ifneq (,$(wildcard .e2e.env))
include .e2e.env
export
endif
```

The `.e2e.env` file SHALL use plain `KEY=VALUE` Make/shell syntax (one var per line, no `export` prefix). It SHALL be listed in `.gitignore`. A `.e2e.env.example` file with placeholder values for all three vars SHALL be committed to VCS, with instructions to `cp .e2e.env.example .e2e.env` and fill in credentials.

If any required variable is missing after loading, the target SHALL print a descriptive error to stderr and exit non-zero.

#### Scenario: .e2e.env file present, no shell vars

- **WHEN** a `.e2e.env` file exists with all three vars and none are set in the shell
- **THEN** the makefile loads those values and runs the integration tests

#### Scenario: .e2e.env file takes precedence over shell vars

- **WHEN** `TEST_DT_ENVIRONMENT` is set in the shell and a different value exists in `.e2e.env`
- **THEN** the `.e2e.env` value is used, because `include` assignments override imported shell env vars in Make

#### Scenario: No .e2e.env file, vars set in shell

- **WHEN** no `.e2e.env` file exists and all three vars are set in the shell
- **THEN** the makefile proceeds using shell env vars without error

#### Scenario: Missing variable after loading

- **WHEN** a required variable is not set in the shell or in `.e2e.env`
- **THEN** the target prints an error to stderr with copy-paste setup instructions and exits non-zero

### Requirement: Unique test naming

Each E2E test run SHALL generate a unique identifier in the format `dtwiz-test-{unix-timestamp}-{random-string}`. This identifier SHALL be used as the `OTEL_SERVICE_NAME` for instrumented test apps to prevent data collisions.

#### Scenario: Parallel test runs

- **WHEN** two E2E test runs execute simultaneously
- **THEN** each generates a distinct service name and queries only its own traces

### Requirement: Temporary directory isolation

E2E tests SHALL use `t.TempDir()` for all fixture app directories. No test artifacts SHALL persist after the test completes (pass or fail).

#### Scenario: Test failure cleanup

- **WHEN** an E2E test fails mid-execution
- **THEN** the temp directory and its contents are automatically removed by `t.TempDir()`

### Requirement: Client construction via `New()`

`SetupIntegration` SHALL construct the client by calling `client.New()` directly with URLs derived from `TEST_DT_ENVIRONMENT` via the existing `APIURL()`/`AppsURL()` helpers, exercising the same HTTP client used in production. `TEST_DT_ACCESS_TOKEN` is wired to `ClassicClient` (Classic API and installer operations); `TEST_DT_PLATFORM_TOKEN` is wired to `PlatformClient` (DQL queries and Platform APIs).

#### Scenario: Valid credentials

- **WHEN** `TEST_DT_ENVIRONMENT`, `TEST_DT_ACCESS_TOKEN`, and `TEST_DT_PLATFORM_TOKEN` are set
- **THEN** `SetupIntegration` returns a `TestEnv` with a `*Client` having both `ClassicClient` and `PlatformClient` configured

#### Scenario: Missing environment variable

- **WHEN** `TEST_DT_ENVIRONMENT` is not set
- **THEN** `SetupIntegration` calls `t.Fatal` with a message naming the missing variable

#### Scenario: Missing access token variable

- **WHEN** `TEST_DT_ACCESS_TOKEN` is not set
- **THEN** `SetupIntegration` calls `t.Fatal` with a message naming the missing variable

#### Scenario: Missing platform token variable

- **WHEN** `TEST_DT_PLATFORM_TOKEN` is not set
- **THEN** `SetupIntegration` calls `t.Fatal` with a message naming the missing variable

### Requirement: URL family derivation

`SetupIntegration` SHALL derive both the Classic URL and Platform URL from the single `TEST_DT_ENVIRONMENT` value, using the existing URL family helpers (`APIURL()`/`AppsURL()`).

#### Scenario: Classic URL input

- **WHEN** `TEST_DT_ENVIRONMENT` is `https://abc123.live.dynatrace.com`
- **THEN** Classic URL is `https://abc123.live.dynatrace.com` and Platform URL is `https://abc123.apps.dynatrace.com`

#### Scenario: Platform URL input

- **WHEN** `TEST_DT_ENVIRONMENT` is `https://abc123.apps.dynatrace.com`
- **THEN** Classic URL is `https://abc123.live.dynatrace.com` and Platform URL is `https://abc123.apps.dynatrace.com`

### Requirement: DQL trace query

The test infrastructure SHALL provide a helper function that queries the Dynatrace DQL API for service entities matching a given service name. The function SHALL use the `PlatformClient` from `pkg/client/` via its resty accessor (`.HTTP()`), keeping the Bearer token encapsulated.

The DQL query SHALL use an entity-level `smartscapeNodes` query:

`smartscapeNodes "SERVICE", from: -30m, to: now() | filter name == "<svcName>"`

This queries service entities within a 30-minute window filtered by name. Span-level `fetch spans` queries are not used — they proved unreliable (returned 0 records even when matching spans were ingested).

#### Scenario: Traces found

- **WHEN** the helper queries DQL with a service name that has ingested traces
- **THEN** it returns the matching trace records

#### Scenario: No traces found

- **WHEN** the helper queries DQL with a service name that has no traces
- **THEN** it returns an empty result set without error

### Requirement: Async DQL execution

The DQL execute endpoint (`/platform/storage/query/v1/query:execute`) MAY return a `RUNNING` state with a `requestToken` instead of immediate results. The helper SHALL handle this two-step flow:

1. POST to `/query:execute` — if response state is `SUCCEEDED`, return records inline
2. If state is `RUNNING`, poll GET `/platform/storage/query/v1/query:poll?request-token=<token>` until state becomes `SUCCEEDED`, up to a fixed retry limit (10 retries, 1s apart)

Any other state SHALL be treated as an error.

#### Scenario: Query completes immediately

- **WHEN** the DQL execute endpoint returns `SUCCEEDED` with records
- **THEN** the helper returns those records without polling

#### Scenario: Query deferred (RUNNING state)

- **WHEN** the DQL execute endpoint returns `RUNNING` with a `requestToken`
- **THEN** the helper polls the poll endpoint until `SUCCEEDED` and returns the records

#### Scenario: Poll retries exceeded

- **WHEN** the poll endpoint returns `RUNNING` for all retry attempts
- **THEN** the helper returns an error indicating the retry limit was exceeded

### Requirement: Polling with timeout

The trace verification helper SHALL poll the DQL API at a configurable interval until traces are found or a timeout is reached. The default timeout SHALL be 60 seconds with a 2-second poll interval.

#### Scenario: Traces arrive within timeout

- **WHEN** traces are ingested after a delay but before the timeout
- **THEN** the helper returns successfully once traces are found

#### Scenario: Timeout exceeded

- **WHEN** no traces appear within the configured timeout
- **THEN** the helper returns an error indicating the timeout was exceeded and the service name that was queried

### Requirement: DQL query scope

The DQL query SHALL filter service entities by name matching the unique test identifier. The query targets the `smartscapeNodes "SERVICE"` entity source with a 30-minute look-back window.

#### Scenario: Query specificity

- **WHEN** multiple services are sending traces to the same tenant
- **THEN** the query returns only entities matching the exact test service name
