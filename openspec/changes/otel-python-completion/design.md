## Context

OTel Python instrumentation (`dtwiz install otel-python`) is fully implemented with project detection, entrypoint detection, virtualenv management, package installation, process launch, and Dynatrace verification. However, there is no `dtwiz uninstall otel-python` command, and the installer lacks pre-flight validation checks. The install flow was validated on macOS only.

Current Python install flow: detect projects → user selects one → detect entrypoints → stop running processes → create venv → install packages → launch with `opentelemetry-instrument` → verify in Dynatrace.

## Goals / Non-Goals

**Goals:**
- Implement `dtwiz uninstall otel-python` to cleanly reverse what `install` did
- Add pre-flight validation at the start of `InstallOtelPython()`
- Support `--dry-run` for the uninstall command

**Non-Goals:**
- Changing the existing install flow logic
- Supporting multi-project instrumentation
- Persistent configuration management (tracking what was installed where)

## Decisions

**1. Uninstall detects instrumented processes by OTel environment variables**
Find running Python processes that have `OTEL_SERVICE_NAME` or `opentelemetry-instrument` in their command line. This reliably identifies processes that were launched by our installer.

Alternative: Track installed projects in a state file — rejected because it adds state management complexity. Process detection is sufficient and mirrors how the collector uninstall works.

**2. Uninstall removes OTel packages but preserves the virtualenv**
Run `pip uninstall opentelemetry-distro opentelemetry-exporter-otlp` in the project's virtualenv. Don't delete the virtualenv itself — the user may have other packages in it.

Alternative: Delete the entire virtualenv — too destructive, may remove user's other dependencies.

**3. Pre-flight validation checks**
At the start of `InstallOtelPython()`, validate:
- `python3` in PATH
- `pip3` or `pip` available
- `venv` module available (`python3 -m venv --help`)
- Fail with clear error message if any check fails

## Risks / Trade-offs

- [Process detection may miss detached processes] → Accept this limitation. Document that users should manually stop processes launched outside of dtwiz.
- [Uninstall in wrong virtualenv] → Detect virtualenv from the process's Python path to target the correct env.
