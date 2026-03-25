## Context

OTel Python instrumentation (`dtwiz install otel-python`) is fully implemented with project detection, entrypoint detection, virtualenv management, package installation, process launch, and Dynatrace verification. However, there is no `dtwiz uninstall otel-python` command, and the installer lacks pre-flight validation checks. The install flow was validated on macOS only.

Current Python install flow: detect projects → user selects one → detect entrypoints → stop running processes → create venv → install packages → launch with `opentelemetry-instrument` → verify in Dynatrace.

## Goals / Non-Goals

**Goals:**
- Add pre-flight validation at the start of `InstallOtelPython()`

**Non-Goals:**
- Changing the existing install flow logic
- Supporting multi-project instrumentation
- Persistent configuration management (tracking what was installed where)

## Decisions

**1. Pre-flight validation checks**
At the start of `InstallOtelPython()`, validate:
- `python3` in PATH
- `pip3` or `pip` available
- `venv` module available (`python3 -m venv --help`)
- Fail with clear error message if any check fails

## Risks / Trade-offs

- None identified for the validation approach.
