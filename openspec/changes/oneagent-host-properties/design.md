# Design: OneAgent Host Properties

## Context

`dtwiz install oneagent` already passes one optional installer setting: `--host-group`, which becomes `--set-host-group=<value>` in `BuildInstallCommand` (`pkg/installer/oneagent/install_agent.go`). The installer argv is never run through a shell:

- Linux runs it with `exec`.
- Windows runs it with `ShellExecuteEx`, and `quoteWindowsArgs` quotes each argument.

The OneAgent installer accepts `--set-host-property=<value>` on Linux and Windows, repeatable, with `KEY` or `KEY=VALUE` values. Its rules: alphanumerics plus `.`, `_` and `-`, no whitespace, no `=` other than the delimiter, and at most 256 characters including the delimiter.

## Goals / Non-Goals

**Goals:**

- Mirror the `--host-group` path for a repeatable value, reusing the same argv and preview code paths.
- Fail fast on values the installer would reject.

**Non-Goals:**

- Host properties for `install kubernetes` (DynaKube) or `install otel`. Those have their own mechanisms and are separate changes if wanted.
- Changing properties on an already-installed agent (`oneagentctl --set-host-property`).
- Reading properties from cloud tags automatically. The user passes them explicitly.
- Host tags (`--set-host-tag`), a separate installer setting.

## Decisions

### `--host-property` as a repeatable string array, not a map

The flag is a `StringArray`, and values are passed to the installer unchanged and in order.

- **Why:** a map (`StringToString`) would sort or deduplicate keys, drop bare `KEY` values, and split on commas, which differs from the installer's own syntax. Passing values through unchanged keeps dtwiz's behavior identical to the documented installer flag.
- **Alternative:** `--host-property team=payments,env=prod` as comma-separated. Rejected because it's harder to read, and dtwiz's other repeatable flags are repeated rather than comma-separated.

### Validation lives in the oneagent package and runs first in the command

A `ValidateHostProperties([]string) error` function in `pkg/installer/oneagent` checks each value against the installer's rules. The command handler calls it right after the feature-flag check, before credentials are resolved.

- **Why:** a bad value is a usage error. It shouldn't cost a token validation round trip or a 200 MB download before failing.
- **Alternative:** let the installer reject it. Rejected because the installer error arrives late, and dtwiz can't show it in the preview.

### Feature flag `oneagent-host-properties`, using the `install docker` gating pattern

The new `OneAgentHostProperties` flag is registered in `pkg/featureflags` with default `false`. Following the existing `install docker` / `--experimental` pattern:

- `RunE` returns a clear error when `--host-property` is passed and the flag is off.
- The install help function sets the `--host-property` flag's `Hidden` field to the inverse of the feature flag, so the option only shows in help when the feature is on.

- **Why the name:** it matches the change name and the earlier `oneagent-poc` flag, and it reads clearly next to the `--host-property` option it gates.
- **GA:** a single follow-up change removes the flag and its checks, as was done for `oneagent-poc`.

### Preview

`printDryRun` gets a `Host props:` line after `Host group:`. The existing `Command:` line shows the arguments automatically, because it is built by the same `BuildInstallCommand`.

## Risks / Trade-offs

- [The installer tightens or loosens its rules later] → Validation is a single function with a table-driven test, so it's easy to update. A rule that's too strict blocks a value the installer would accept, which is safer than the reverse.
- [Users pass sensitive values as properties] → Values only go to the local installer and appear in the local preview and debug log, the same as `--host-group`. Self-monitoring never records them.

## Migration Plan

None. The change is additive and off by default. Rollback is turning the flag off or reverting.
