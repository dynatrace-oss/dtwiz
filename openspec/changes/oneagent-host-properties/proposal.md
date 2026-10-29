# Proposal: OneAgent Host Properties

## Why

Teams that roll out OneAgent at scale tag every host with ownership and context, such as team, environment and cost center, so hosts can be segmented and alerts routed from day one. Today `dtwiz install oneagent` can only set a host group, so these tags have to be added by hand after install, or with a separate `oneagentctl` call on every host.

## What Changes

- Add a repeatable `--host-property` flag to `dtwiz install oneagent`. Each value is passed to the OneAgent installer as `--set-host-property=<value>` on Linux and Windows. Values are either `KEY` or `KEY=VALUE`, matching the installer's own syntax.
- Validate each value before any download or credential check, so a bad property fails fast with a clear error.
- Show the properties in the dry-run preview and in the installer command line.
- Gate the behavior behind a new feature flag, `oneagent-host-properties` (`--oneagent-host-properties` / `DTWIZ_ONEAGENT_HOST_PROPERTIES`). It is off by default. While it is off, `--host-property` is hidden from help and using it returns an error that names the feature flag. This is the same pattern as `install docker` with `--experimental`.

## Capabilities

### New Capabilities

- `oneagent-host-properties`: Assign custom host properties to a host when installing OneAgent through `dtwiz install oneagent`, including validation, preview and feature-flag gating.

### Modified Capabilities

## Impact

- `cmd/install.go`: adds the new `--host-property` flag, the feature-flag check, and hiding the flag in help.
- `pkg/installer/oneagent/`: adds a `HostProperties` field to `InstallOptions`, validation of the values, the argv construction in `BuildInstallCommand`, and the preview in `printDryRun`.
- `pkg/featureflags/featureflags.go`: registers the new `OneAgentHostProperties` flag.
- Other install methods (kubernetes, otel, docker) are unaffected. `install oneagent` behaves exactly as before when `--host-property` is not passed, whatever the feature flag is set to.
- Self-monitoring is unchanged. Events record command names only, so property values never leave the machine.
- Rollback: turn the feature flag off, or revert the change. No persisted state is introduced.
