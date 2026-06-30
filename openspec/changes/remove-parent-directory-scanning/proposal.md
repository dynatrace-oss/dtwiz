## Why

When scanning for OTel project instrumentation, the CLI tells the user "Scanning `<dir>`" but silently also scans up to 2 parent directories — behaviour that is inconsistent with the message and unexpected to the user. Removing parent-directory scanning makes the tool do exactly what it says.

## What Changes

- `scanProjectDirs` in `pkg/installer/otel_runtime_scan.go` calls `walkCandidateDirs` with `parentLevels=2`; change to `parentLevels=0` so only the working directory and its subdirectories are scanned.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `otel-project-scan`: Scanning scope restricted to working directory tree only; parent directories no longer traversed.

## Impact

- `pkg/installer/otel_runtime_scan.go` — one-line change to `walkCandidateDirs` call.
- Users running `dtwiz` from a subdirectory of their project root (e.g. `my-project/src/`) will no longer have the project root auto-detected. They must run from the project root or a directory that contains the project.
