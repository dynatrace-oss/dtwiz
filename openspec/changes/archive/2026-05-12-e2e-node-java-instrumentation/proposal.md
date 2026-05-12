# Proposal

## Why

`TestOTelAutoInstrumentation` only covered Python. Node.js and Java installers had no integration test coverage, leaving regressions undetected.

## What Changes

- Add Node.js and Java test cases to `TestOTelAutoInstrumentation`, each as a parallel subtest
- Add minimal fixture apps (`test/fixtures/node-http/`, `test/fixtures/java-maven/`) that start an HTTP server and respond `200 OK`
- Add a `projectPath` parameter to `InstallOtelJava` and `InstallOtelNode` so tests can target a fixture directory directly, bypassing filesystem scanning and interactive prompts
- Expose `--project` flag on `dtwiz install otel-node` and `dtwiz install otel-java`, consistent with `otel` and `otel-python`, and wire it through to the installer
- Restore port injection via per-language env vars (`TEST_FLASK_APP_PORT`, `TEST_NODE_APP_PORT`, `TEST_JAVA_APP_PORT`) using `os.Setenv` + `t.Cleanup` instead of `t.Setenv` (which panics inside parallel subtests)

## Capabilities

### Modified Capabilities

- `e2e-test-infra`: extended with Node.js and Java test cases
