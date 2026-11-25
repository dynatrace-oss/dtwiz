# Spec Delta

## ADDED Requirements

### Requirement: `DotNetOtel` flag gates .NET OpenTelemetry support

The registry SHALL contain a flag with CLI name `--dotnet-otel`, environment variable `DTWIZ_DOTNET_OTEL` and default `false`. It SHALL follow the same resolution order and appear in `--help`, `List()` and `dtwiz status` like the other flags.

#### Scenario: Default off

- **GIVEN** neither `DTWIZ_DOTNET_OTEL` nor `--dotnet-otel` is set
- **WHEN** the flag is checked
- **THEN** it is disabled

#### Scenario: CLI flag overrides env var

- **GIVEN** `DTWIZ_DOTNET_OTEL=false` is set
- **AND** `--dotnet-otel` is passed
- **WHEN** the flag is checked
- **THEN** it is enabled

#### Scenario: Listed in help

- **GIVEN** the root command registered the flags
- **WHEN** the user runs `dtwiz --help`
- **THEN** `--dotnet-otel` appears with its description
