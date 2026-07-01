# Design: Remove Parent Directory Scanning

## Context

`scanProjectDirs` in `pkg/installer/otel_runtime_scan.go` drives OTel instrumentation project discovery for all runtimes (Python, Node.js, Java). It calls `walkCandidateDirs(workingDir, 2, ...)`, where the second argument (`parentLevels=2`) causes the scanner to also walk up to 2 ancestor directories of the working directory after scanning the tree rooted at `workingDir`.

`findNodeOtelDirs` in `pkg/installer/otel_uninstall.go` performs the same `walkCandidateDirs(cwd, 2, ...)` call during Node.js uninstall to locate `.otel/` directories to remove.

The user-facing message ("Scanning `/home/user/myapp`") names only the working directory, creating a silent inconsistency. Keeping the uninstall scope aligned with install avoids a situation where uninstall could find and remove directories that install would no longer create.

## Goals / Non-Goals

**Goals:**

- Scan scope matches what the CLI tells the user: working directory and its subtree only.
- No silent traversal of directories outside the working directory tree.

**Non-Goals:**

- Adding a flag to re-enable parent scanning.
- Changing how `walkCandidateDirs` itself works (the `parentLevels` parameter remains; we just pass `0`).

## Decisions

**Pass `parentLevels=0` instead of `2` at both call sites.**

Call sites: `scanProjectDirs` (install) and `findNodeOtelDirs` (uninstall). Changing the literal from `2` to `0` at each is sufficient. The `walkCandidateDirs` function already handles `parentLevels=0` correctly (the ancestor loop never executes).

Alternatives considered:

- Remove the `parentLevels` parameter entirely — rejected; would be a larger refactor for no gain, and the parameter may be useful in future callers.
- Keep parent scanning in uninstall for backwards compat (clean up previously installed dirs) — rejected; install no longer creates dirs in parent directories, so uninstall reaching into parents would be inconsistent and surprising.
- Keep parent scanning but update the message — rejected; the user's expectation is that scanning stays within the current directory.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| User runs `dtwiz` from a subdirectory (e.g. `my-project/src/`) and their project root is no longer auto-detected | Documented behaviour; the tip message already suggests running from the project root |
| Previously installed `.otel/` dirs in parent directories are not cleaned up by uninstall | Acceptable: those dirs were created by an older version; the user can remove them manually. Install no longer creates them there. |
