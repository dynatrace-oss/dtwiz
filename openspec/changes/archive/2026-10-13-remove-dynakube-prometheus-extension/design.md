# Design

## Context

`pkg/installer/kubernetes/dynakube.tmpl` renders two DynaKube objects: a monitoring DynaKube (#1) and an agents DynaKube (#2). DynaKube #2 previously included:

```yaml
extensions:
  prometheus: {}
```

This block was removed directly from the template in a prior commit (`feat: adjust dynakube template`), and `pkg/installer/kubernetes/install_test.go` was updated to match. The standalone spec was not updated at the time, so `openspec/specs/k8s-distro-aware-manifest/spec.md` still asserted the block as required — this was caught when `make test-coverage` failed on the stale test, and the test was already fixed; this change only catches up the spec.

## Goals / Non-Goals

**Goals:**

- Bring `openspec/specs/k8s-distro-aware-manifest/spec.md` back in sync with the already-shipped template and test behavior.

**Non-Goals:**

- Re-introduce the `extensions.prometheus: {}` block (not desired — its removal was the intended behavior).
- Change any other part of the DynaKube template or installer.

## Decisions

- Treat this as a documentation-only spec correction via the standard change/apply/archive workflow, since the underlying code is already merged on this branch. `tasks.md` reflects a single documentation task.

## Risks / Trade-offs

- None — this is a textual spec correction with no behavioral surface.
