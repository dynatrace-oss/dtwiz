# Design

## Context

The e2e test infra (established in `2026-05-05-e2e-testing-infrastructure`) wires up a `TestEnv`, fixture preparation, port waiting, request triggering, and Grail trace polling. Adding a new language case means providing a fixture app, a skip check, and an install function.

## Goals / Non-Goals

**Goals:** Add Node.js and Java to the existing parallel test table; keep fixture apps minimal; make installer entry points testable without interactive prompts.

**Non-Goals:** Multi-project or multi-module Java testing; Windows CI support.

## Decisions

**1. `projectPath` parameter on installer entry points**
Both `InstallOtelJava` and `InstallOtelNode` grew a `projectPath string` parameter. When non-empty, the installer skips `scanJavaProjects()` / `scanNodeProjects()` and the prompt, using the supplied path directly. Empty string preserves the interactive CLI flow. CLI callers pass `""`.

**2. `os.Setenv` + `t.Cleanup` instead of `t.Setenv`**
Go's testing package panics if `t.Setenv` is called after `t.Parallel()`. Since each language uses a distinct env var, there is no actual data race — the restriction is overly conservative here. Using `os.Setenv` with a `t.Cleanup` restore is the idiomatic workaround.

**3. Java fixture built as a fat JAR**
`detectJavaEntrypoints` looks for `java -jar` invocations. The Maven fixture uses `maven-assembly-plugin` to produce a fat JAR, ensuring the detector finds a runnable entrypoint rather than falling back to `mvn exec:java`.

**4. Node fixture creates `node_modules/` manually**
`InstallOtelNode` checks that `node_modules/` exists before launching. The fixture has no npm dependencies, so the directory is created with `os.MkdirAll` in the test rather than running `npm install`.
