# E2E Test Infrastructure

## ADDED Requirements

### Requirement: Build tag separation

All E2E test files in `test/e2e/` SHALL use the `//go:build integration` build tag. The tag acts as a gating mechanism: it excludes E2E files from default builds (`go test ./...` without `-tags integration`) and includes them when explicitly requested (`go test -tags integration`). Whether it operates as exclusion or inclusion depends on the invocation — both are intentional behaviors of the same tag.

#### Scenario: Default test run excludes E2E

- **WHEN** a developer runs `go test ./...` or `make test`
- **THEN** no files under `test/e2e/` are compiled or executed

#### Scenario: Integration tag includes E2E

- **WHEN** a developer runs `go test -tags integration ./test/e2e/...`
- **THEN** all E2E test files are compiled and executed

### Requirement: Environment variable gating

The `make test-integration` target SHALL require `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` to be set. If either is missing, the target SHALL print a descriptive error message to stderr and exit with a non-zero status code.

#### Scenario: Missing TEST_DT_ENVIRONMENT

- **WHEN** `make test-integration` is run without `TEST_DT_ENVIRONMENT` set
- **THEN** an error message naming the missing variable is printed to stderr and the process exits non-zero

#### Scenario: Missing TEST_DT_ACCESS_TOKEN

- **WHEN** `make test-integration` is run without `TEST_DT_ACCESS_TOKEN` set
- **THEN** an error message naming the missing variable is printed to stderr and the process exits non-zero

#### Scenario: Both variables set

- **WHEN** `make test-integration` is run with both `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` set
- **THEN** `go test -tags integration ./test/e2e/...` is executed

### Requirement: .env file loading

The `make test-integration` target SHALL load variables from a `.e2e-tests.env` file in the project root if it exists. Shell environment variables SHALL take precedence over `.e2e-tests.env` values. The `.e2e-tests.env` file SHALL be listed in `.gitignore`. A `.e2e-tests.env.example` file with placeholder values for `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` SHALL be committed to VCS as a reference.

#### Scenario: .e2e-tests.env file present

- **WHEN** a `.e2e-tests.env` file exists with `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN`
- **THEN** the Makefile loads those values and runs the integration tests

#### Scenario: Shell overrides .e2e-tests.env

- **WHEN** `TEST_DT_ENVIRONMENT` is set in the shell and a different value exists in `.e2e-tests.env`
- **THEN** the shell value is used

#### Scenario: No .e2e-tests.env file

- **WHEN** no `.e2e-tests.env` file exists and env vars are set in the shell
- **THEN** the Makefile proceeds using shell env vars without error

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

### Requirement: NewForTesting constructor

`pkg/client/` SHALL expose a `NewForTesting(t *testing.T) *Client` function that creates a fully configured `Client` by reading `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` environment variables.

#### Scenario: Valid credentials

- **WHEN** `TEST_DT_ENVIRONMENT` and `TEST_DT_ACCESS_TOKEN` are set
- **THEN** `NewForTesting` returns a `*Client` with both `ClassicClient` and `PlatformClient` configured

#### Scenario: Missing environment variable

- **WHEN** `TEST_DT_ENVIRONMENT` is not set
- **THEN** `NewForTesting` calls `t.Fatal` with a message naming the missing variable

#### Scenario: Missing token variable

- **WHEN** `TEST_DT_ACCESS_TOKEN` is not set
- **THEN** `NewForTesting` calls `t.Fatal` with a message naming the missing variable

### Requirement: URL family derivation

`NewForTesting` SHALL derive both the Classic URL and Platform URL from the single `TEST_DT_ENVIRONMENT` value, using the existing URL family logic (strip `.apps.` for Classic, insert `.apps.` for Platform).

#### Scenario: Classic URL input

- **WHEN** `TEST_DT_ENVIRONMENT` is `https://abc123.live.com`
- **THEN** Classic URL is `https://abc123.live.com` and Platform URL is `https://abc123.apps.com`

#### Scenario: Platform URL input

- **WHEN** `TEST_DT_ENVIRONMENT` is `https://abc123.apps.com`
- **THEN** Classic URL is `https://abc123.live.com` and Platform URL is `https://abc123.apps.com`

### Requirement: No changes to existing client API

`NewForTesting` SHALL be an additive function. The existing `New()` constructor and all existing types SHALL remain unchanged.

#### Scenario: Existing code unaffected

- **WHEN** `NewForTesting` is added to `pkg/client/`
- **THEN** all existing unit tests for `pkg/client/` continue to pass without modification

### Requirement: DQL trace query

The test infrastructure SHALL provide a helper function that queries the Dynatrace DQL API for traces matching a given service name. The function SHALL use the `PlatformClient` from `pkg/client/`.

#### Scenario: Traces found

- **WHEN** the helper queries DQL with a service name that has ingested traces
- **THEN** it returns the matching trace records

#### Scenario: No traces found

- **WHEN** the helper queries DQL with a service name that has no traces
- **THEN** it returns an empty result set without error

### Requirement: Polling with timeout

The trace verification helper SHALL poll the DQL API at a configurable interval until traces are found or a timeout is reached. The default timeout SHALL be 60 seconds with a 2-second poll interval.

#### Scenario: Traces arrive within timeout

- **WHEN** traces are ingested after a delay but before the timeout
- **THEN** the helper returns successfully once traces are found

#### Scenario: Timeout exceeded

- **WHEN** no traces appear within the configured timeout
- **THEN** the helper returns an error indicating the timeout was exceeded and the service name that was queried

### Requirement: DQL query scope

The DQL query SHALL filter traces by `service.name` attribute matching the unique test identifier. The query SHALL target the `dt.entity.spans` or equivalent Grail table.

#### Scenario: Query specificity

- **WHEN** multiple services are sending traces to the same tenant
- **THEN** the query returns only traces matching the exact test service name
