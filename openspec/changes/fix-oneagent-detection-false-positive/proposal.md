# Proposal

## Why

`dtwiz status` (and any output driven by `analyzer.AnalyzeSystem()`) reports
`OneAgent: running` on containers that have no OneAgent installed. Verified on a
devcontainer image run via plain Docker: the image ships a `systemctl`
compatibility shim at `/usr/local/bin/systemctl` that prints a warning and
**exits 0 for any invocation** when systemd is not running. `detectOneAgent()`
trusts that exit code, so its first check (`systemctl is-active --quiet
oneagent`) always "succeeds" and every user of such an image — devcontainers,
GitHub Codespaces, docker-based training environments — sees a OneAgent that
does not exist.

Verified reproduction (container, no OneAgent installed):

```
$ systemctl is-active --quiet oneagent; echo "exit=$?"
"systemd" is not running in this container due to its overhead. ...
exit=0

$ command -v oneagentctl
(nothing)

$ dtwiz status
OneAgent:          running        <- false positive
```

## What Changes

- `detectOneAgent()` in `pkg/analyzer/detect_oneagent.go` only runs the
  `systemctl` check when systemd is actually the running init system:
  `/run/systemd/system` exists as a directory (the `sd_booted(3)` convention —
  the same guard the devcontainer shim itself uses before delegating to the
  real systemctl).
- The systemd marker path is hoisted to a package-level `systemdRunDir`
  variable so tests can redirect it.
- New unit tests in `pkg/analyzer/detect_oneagent_test.go` cover the guard and
  both detection paths, including a faithful reproduction of the shim
  (a PATH-injected `systemctl` script that exits 0).
- The `oneagentctl --version` fallback and the Windows implementation
  (`detect_oneagent_windows.go`, separate build tag) are unchanged.

## Capabilities

### Modified Capabilities

- `oneagent-detection`: on non-systemd hosts (containers), the `systemctl` exit
  code is no longer trusted; detection falls through to `oneagentctl`. Behavior
  on systemd hosts is unchanged.

## Impact

- Affected code: `pkg/analyzer/detect_oneagent.go` (guard + variable hoist),
  `pkg/analyzer/detect_oneagent_test.go` (new).
- No interface, CLI, or output-format changes; only the correctness of the
  reported OneAgent state on non-systemd hosts.
- Rollback: revert the guard and inline the path constant; the fallback check
  is untouched either way.
