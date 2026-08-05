# Design

## Context

`detectOneAgent()` (Unix build) runs two sequential checks and returns true on
the first success:

1. `systemctl is-active --quiet oneagent`
2. `oneagentctl --version`

Devcontainer-family images (used by GitHub Codespaces and commonly run as plain
Docker containers) replace `systemctl` with a shim:

```sh
#!/bin/sh
set -e
if [ -d "/run/systemd/system" ]; then
    exec /bin/systemctl "$@"
else
    # prints '"systemd" is not running in this container...' and exits 0
```

When systemd is absent the shim exits 0 for *any* arguments, so check 1 is a
guaranteed false positive in these environments. dtwiz trainings run primarily
in exactly these containers.

## Goals / Non-Goals

**Goals:**

- Never report a OneAgent that is not there, without breaking detection on real
  systemd hosts.
- Keep the change minimal and testable.

**Non-Goals:**

- Detecting OneAgents that run without systemd and without `oneagentctl` in
  PATH (e.g. agents inside sibling containers). That is a broader detection
  feature, not a bug fix.
- Parsing `systemctl` output text (fragile, locale-dependent).

## Decisions

### Guard: `/run/systemd/system` directory check

Alternatives considered:

1. **Parse `systemctl is-active` stdout** — the real command prints
   `active`/`inactive`; the shim prints a prose warning. Rejected: output
   parsing is fragile and locale/tool-version dependent.
2. **Process scan (`pgrep -f oneagentwatchdog`)** — would also catch
   non-systemd agent installs, but changes detection semantics beyond the bug
   being fixed. Out of scope (could be a follow-up capability).
3. **`[ -d /run/systemd/system ]` guard (chosen)** — the documented
   `sd_booted(3)` convention for "systemd is the running init". Notably it is
   the exact test the shim itself performs before delegating to the real
   systemctl, so the guard is by construction at least as accurate as the shim.

### Testability: package-level `systemdRunDir` variable

`os.Stat` on a hardcoded absolute path cannot be exercised from tests on
arbitrary CI hosts. Hoisting the path into a package-level variable follows the
package's existing injection style (cf. `sleeper func(time.Duration)` in the
installer packages) and lets tests point it at a temp directory. The shim
behavior itself is reproduced in tests with a PATH-injected `systemctl` script
that exits 0, asserting that detection ignores it when the marker directory is
absent and trusts it when present.
