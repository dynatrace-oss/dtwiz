# Design: Remove Parent Directory Scanning

## Context

`scanProjectDirs` in `pkg/installer/otel_runtime_scan.go` drives OTel instrumentation project discovery for all runtimes (Python, Node.js, Java). It calls `walkCandidateDirs(workingDir, 2, ...)`, where the second argument (`parentLevels=2`) causes the scanner to also walk up to 2 ancestor directories of the working directory after scanning the tree rooted at `workingDir`.

The user-facing message ("Scanning `/home/user/myapp`") names only the working directory, creating a silent inconsistency.

## Goals / Non-Goals

**Goals:**

- Scan scope matches what the CLI tells the user: working directory and its subtree only.
- No silent traversal of directories outside the working directory tree.

**Non-Goals:**

- Adding a flag to re-enable parent scanning.
- Changing how `walkCandidateDirs` itself works (the `parentLevels` parameter remains; we just pass `0`).

## Decisions

**Pass `parentLevels=0` instead of `2`.**

The only call site is `scanProjectDirs` (line 247). Changing the literal from `2` to `0` is sufficient. The `walkCandidateDirs` function already handles `parentLevels=0` correctly (the loop body never executes).

Alternatives considered:

- Remove the `parentLevels` parameter entirely — rejected; would be a larger refactor for no gain, and the parameter may be useful in future callers.
- Keep parent scanning but update the message — rejected; the user's expectation is that scanning stays within the current directory.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| User runs `dtwiz` from a subdirectory (e.g. `my-project/src/`) and their project root is no longer auto-detected | Documented behaviour; the tip message already suggests running from the project root |
