# E2E Integration Tests — Node.js and Java OTel Instrumentation

## ADDED Requirements

### Requirement: Node.js and Java test cases in TestOTelAutoInstrumentation

`TestOTelAutoInstrumentation` SHALL include test cases for Node.js and Java alongside the existing Python case. Each case SHALL run as a parallel subtest.

#### Scenario: Node.js subtest

- **GIVEN** `node` and `npm` are in `PATH`
- **WHEN** `TestOTelAutoInstrumentation/node` runs
- **THEN** `InstallOtelNode` is called with the fixture app directory, the app starts on port `18082`, a request is triggered, and traces appear in Grail for the test service

#### Scenario: Java subtest

- **GIVEN** `java` and `mvn` are in `PATH`
- **WHEN** `TestOTelAutoInstrumentation/java` runs
- **THEN** the fixture app is built via `mvn clean package`, `InstallOtelJava` is called with the fixture app directory, the app starts on port `18081`, a request is triggered, and traces appear in Grail for the test service

### Requirement: Port injection via environment variable

Each test case SHALL inject its port into the fixture app via a per-language env var before the installer launches the app. The env var SHALL be restored after the subtest completes.

| Language | Env var              | Default port |
|----------|----------------------|--------------|
| Python   | `TEST_FLASK_APP_PORT` | 18080        |
| Node.js  | `TEST_NODE_APP_PORT`  | 18082        |
| Java     | `TEST_JAVA_APP_PORT`  | 18081        |

Because `t.Setenv` panics inside parallel subtests, the implementation SHALL use `os.Setenv` with a `t.Cleanup` restore instead.

### Requirement: projectPath parameter on installer entry points

`InstallOtelJava` and `InstallOtelNode` SHALL accept an optional `projectPath string` parameter. When non-empty, the installer SHALL skip filesystem scanning and the interactive project-selection prompt, and use the provided path directly. An empty string SHALL preserve the existing interactive behaviour.

### Requirement: --project flag on otel-node and otel-java CLI commands

`dtwiz install otel-node` and `dtwiz install otel-java` SHALL expose a `--project <path>` flag, consistent with `dtwiz install otel` and `dtwiz install otel-python`. The flag value SHALL be passed through to `InstallOtelNode` / `InstallOtelJava` as `projectPath`.

#### Scenario: --project skips interactive scan

- **WHEN** `dtwiz install otel-node --project ./my-app` is run
- **THEN** the Node.js installer uses `./my-app` directly and does not scan the filesystem or prompt for project selection

#### Scenario: --project skips interactive scan (Java)

- **WHEN** `dtwiz install otel-java --project ./my-app` is run
- **THEN** the Java installer uses `./my-app` directly and does not scan the filesystem or prompt for project selection

### Requirement: Fixture apps

Two minimal fixture apps SHALL exist under `test/fixtures/`:

- `node-http/` — bare HTTP server reading `TEST_NODE_APP_PORT` (default `18082`)
- `java-maven/` — Maven fat-JAR HTTP server reading `TEST_JAVA_APP_PORT` (default `18081`)

Each fixture app SHALL respond `200 OK` to any request on its configured port.
