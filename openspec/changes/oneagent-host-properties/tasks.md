# Tasks

## 1. Feature flag

- [ ] 1.1 Register `OneAgentHostProperties` in `pkg/featureflags/featureflags.go` (`oneagent-host-properties`, `DTWIZ_ONEAGENT_HOST_PROPERTIES`, default `false`, description ending "(development only)"). Verify with new cases in `pkg/featureflags/featureflags_test.go`: default off, env `true`/`1` on, and the flag appears in `List()`.

## 2. Validation and installer argv

- [ ] 2.1 Add `HostProperties []string` to `InstallOptions` in `pkg/installer/oneagent/oneagent.go`.
- [ ] 2.2 Add `ValidateHostProperties` in `pkg/installer/oneagent/` that checks each value against the spec rules (`KEY` or `KEY=VALUE`, `[A-Za-z0-9._-]`, at most one `=`, non-empty key and value, ≤256 chars). Verify with a table-driven test covering valid key-only, valid key-value, whitespace, `a=b=c`, `=x`, `x=`, unicode, and 256- and 257-character values.
- [ ] 2.3 Append one `--set-host-property=<value>` per property, in order, in `BuildInstallCommand` for Linux and Windows. Verify with `install_agent_test.go` cases: Linux and Windows with properties, order preserved, and no `--set-host-property` when none are given.
- [ ] 2.4 Print a `Host props:` line in `printDryRun` when properties are set. Verify with an `oneagent_test.go` case asserting the line and the `--set-host-property` arguments in the `Command:` line.

## 3. Command wiring

- [ ] 3.1 Add the repeatable `--host-property` flag to `installOneAgentCmd` in `cmd/install.go`, hidden by default. Pass the values into `InstallOptions`.
- [ ] 3.2 In `installOneAgentCmd.RunE`, before credentials are resolved: when `--host-property` is set and the feature flag is off, return the experimental-feature error naming `--oneagent-host-properties` / `DTWIZ_ONEAGENT_HOST_PROPERTIES=true`. Otherwise run `ValidateHostProperties` and return its error. Verify with `cmd/install_test.go` cases: flag off with a property gives the error, flag on with an invalid property gives the validation error, and neither case makes a network call.
- [ ] 3.3 In the install help function, set the `--host-property` flag's `Hidden` field to the inverse of the feature flag. Verify with a `cmd/install_test.go` case: help output lacks `--host-property` by default and lists it with `DTWIZ_ONEAGENT_HOST_PROPERTIES=true`.
- [ ] 3.4 Document `--host-property` and the feature flag in `README.md` (OneAgent flags) and add an `[Unreleased]` CHANGELOG entry naming the flag. Verify that `make lint` and the markdown lint pass.

## 4. Integration check

- [ ] 4.1 Run `make test` and `make lint`. Then run `DTWIZ_ONEAGENT_HOST_PROPERTIES=true dtwiz install oneagent --dry-run --host-property team=payments --host-property env=prod` on Linux and Windows snapshot builds, and confirm the preview matches the spec.

## Workflow follow-up

- Archive this change with `/opsx:archive` after the implementation PR merges.
- GA: a follow-up change removes the `oneagent-host-properties` flag and its checks.
