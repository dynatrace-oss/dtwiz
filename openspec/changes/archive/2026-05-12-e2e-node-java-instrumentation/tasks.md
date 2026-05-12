# Tasks

- [x] Add `projectPath` parameter to `InstallOtelJava`
- [x] Add `projectPath` parameter to `InstallOtelNode`
- [x] Expose `--project` flag on `dtwiz install otel-node` and wire it through to `InstallOtelNode`
- [x] Expose `--project` flag on `dtwiz install otel-java` and wire it through to `InstallOtelJava`
- [x] Add `test/fixtures/node-http/` fixture app
- [x] Add `test/fixtures/java-maven/` fixture app (fat JAR via maven-assembly-plugin)
- [x] Add Node.js test case to `TestOTelAutoInstrumentation`
- [x] Add Java test case to `TestOTelAutoInstrumentation`
- [x] Restore `portEnv` field on `otelCase` for all three languages
- [x] Replace `t.Setenv` with `os.Setenv` + `t.Cleanup` to support parallel subtests
