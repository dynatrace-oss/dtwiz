# Spec Delta

## Purpose

Define how `dtwiz install oneagent` assigns custom host properties to the host at install time. Hosts then arrive in Dynatrace already tagged with ownership and context such as team, environment or cost center.

## ADDED Requirements

### Requirement: Host properties are passed to the OneAgent installer

When the `oneagent-host-properties` feature flag is enabled, `dtwiz install oneagent` SHALL accept a repeatable `--host-property` flag. Each value SHALL be passed to the OneAgent installer as a separate `--set-host-property=<value>` argument, in the order given, on both Linux and Windows.

#### Scenario: Single key-value property

- **GIVEN** the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user runs `dtwiz install oneagent --host-property team=payments`
- **THEN** the OneAgent installer is run with `--set-host-property=team=payments`

#### Scenario: Multiple properties keep their order

- **GIVEN** the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user runs `dtwiz install oneagent --host-property team=payments --host-property env=prod --host-property pci`
- **THEN** the installer is run with `--set-host-property=team=payments`, `--set-host-property=env=prod` and `--set-host-property=pci`, in that order

#### Scenario: Windows installer receives the same properties

- **GIVEN** the `oneagent-host-properties` feature flag is enabled and dtwiz runs on Windows
- **WHEN** the user runs `dtwiz install oneagent --host-property env=prod`
- **THEN** the Windows installer is run with `--set-host-property=env=prod`

#### Scenario: No properties given

- **GIVEN** any feature flag state
- **WHEN** the user runs `dtwiz install oneagent` without `--host-property`
- **THEN** the installer command is exactly the same as before this change, with no `--set-host-property` argument

### Requirement: Host property values are validated before installing

Each `--host-property` value SHALL be either `KEY` or `KEY=VALUE`. Keys and values SHALL contain only letters, digits, `.`, `_` and `-`. Each value SHALL be at most 256 characters, including the `=`. An invalid value SHALL stop the command with an error naming that value, before any credential check, download or installer run.

#### Scenario: Value with whitespace is rejected

- **GIVEN** the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user runs `dtwiz install oneagent --host-property "team=web shop"`
- **THEN** the command fails with an error naming `team=web shop` and stating the allowed format
- **THEN** nothing is downloaded or installed

#### Scenario: Value with more than one equals sign is rejected

- **GIVEN** the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user runs `dtwiz install oneagent --host-property a=b=c`
- **THEN** the command fails with an error naming `a=b=c`

#### Scenario: Empty key or empty value is rejected

- **GIVEN** the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user passes `--host-property =prod` or `--host-property team=`
- **THEN** the command fails with an error naming the invalid value

#### Scenario: Value longer than 256 characters is rejected

- **GIVEN** the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user passes a `--host-property` value of 257 characters
- **THEN** the command fails with an error stating the 256-character limit

### Requirement: Host properties appear in the preview

When host properties are given, the dry-run preview of `dtwiz install oneagent` SHALL list them, and the previewed installer command SHALL include each `--set-host-property` argument.

#### Scenario: Dry run lists the properties

- **GIVEN** the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user runs `dtwiz install oneagent --dry-run --host-property team=payments --host-property env=prod`
- **THEN** the preview shows a host properties line listing `team=payments` and `env=prod`
- **THEN** the previewed installer command includes `--set-host-property=team=payments --set-host-property=env=prod`
- **THEN** nothing is installed

### Requirement: Host properties are gated by the `oneagent-host-properties` feature flag

The `--host-property` flag SHALL only take effect when the `oneagent-host-properties` feature flag is enabled, through `--oneagent-host-properties` or `DTWIZ_ONEAGENT_HOST_PROPERTIES=true`. The flag is disabled by default. While it is disabled, `--host-property` SHALL be hidden from `dtwiz install oneagent --help`, and passing it SHALL fail with an error that names the feature flag.

#### Scenario: Flag disabled, property passed

- **GIVEN** the `oneagent-host-properties` feature flag is disabled (the default)
- **WHEN** the user runs `dtwiz install oneagent --host-property team=payments`
- **THEN** the command fails with an error stating that host properties are an experimental feature, enabled with `--oneagent-host-properties` or `DTWIZ_ONEAGENT_HOST_PROPERTIES=true`
- **THEN** nothing is downloaded or installed

#### Scenario: Flag disabled, help output

- **GIVEN** the `oneagent-host-properties` feature flag is disabled
- **WHEN** the user runs `dtwiz install oneagent --help`
- **THEN** `--host-property` is not listed

#### Scenario: Flag enabled through the environment

- **GIVEN** `DTWIZ_ONEAGENT_HOST_PROPERTIES=true` is set
- **WHEN** the user runs `dtwiz install oneagent --help`
- **THEN** `--host-property` is listed

### Requirement: Host property values are not sent to self-monitoring

Self-monitoring events for `dtwiz install oneagent` SHALL NOT include host property keys or values.

#### Scenario: Install with properties

- **GIVEN** self-monitoring is enabled and the `oneagent-host-properties` feature flag is enabled
- **WHEN** the user runs `dtwiz install oneagent --host-property team=payments`
- **THEN** no self-monitoring event contains `team` or `payments`
