# OneAgent Detection: Container False Positive Fix

## ADDED Requirements

### Requirement: Trust `systemctl` only when systemd is the running init

`detectOneAgent()` SHALL consult `systemctl is-active --quiet oneagent` only
when `/run/systemd/system` exists as a directory (`sd_booted(3)` convention).
On non-systemd hosts — where `systemctl` may be a compatibility shim that exits
0 for any invocation — detection SHALL rely solely on the `oneagentctl` fallback.

#### Scenario: Container with systemctl shim, no OneAgent

- **GIVEN** a container without systemd (`/run/systemd/system` does not exist)
- **AND** a `systemctl` shim in PATH that exits 0 for any invocation
- **AND** `oneagentctl` is not in PATH
- **WHEN** `dtwiz status` runs
- **THEN** the `systemctl` check is skipped entirely
- **AND** system analysis reports OneAgent as `<none>`

#### Scenario: systemd host with an active oneagent unit

- **GIVEN** a host where systemd is the running init (`/run/systemd/system`
  exists as a directory)
- **AND** the `oneagent` systemd unit is active
- **WHEN** `dtwiz status` runs
- **THEN** `systemctl is-active --quiet oneagent` exits 0
- **AND** system analysis reports OneAgent as `running`

#### Scenario: Non-systemd host with oneagentctl in PATH

- **GIVEN** a host without systemd
- **AND** `oneagentctl --version` succeeds
- **WHEN** `dtwiz status` runs
- **THEN** system analysis reports OneAgent as `running` via the fallback check

#### Scenario: Non-systemd host with neither systemd nor oneagentctl

- **GIVEN** a host without systemd
- **AND** `oneagentctl` is not in PATH
- **WHEN** `dtwiz status` runs
- **THEN** system analysis reports OneAgent as `<none>`

### Requirement: Windows detection is unaffected

The Windows implementation (`detect_oneagent_windows.go`, separate build tag)
does not use systemd and SHALL keep its existing behavior.

#### Scenario: Windows host

- **GIVEN** a Windows host
- **WHEN** `dtwiz status` runs
- **THEN** OneAgent detection behaves exactly as before this change
