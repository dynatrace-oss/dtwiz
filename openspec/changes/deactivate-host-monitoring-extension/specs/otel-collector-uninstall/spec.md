# Spec: otel-collector-uninstall (delta)

## ADDED Requirements

### Requirement: Uninstall command resolves credentials when experimental is enabled

The command SHALL always resolve Dynatrace environment credentials before invoking the uninstall flow, consistent with other uninstall commands. If credentials cannot be resolved, the command SHALL exit with an error before showing any preview. Whether the credentials are used is determined internally by the uninstall flow based on the experimental flag.

#### Scenario: Credentials resolved successfully

- **GIVEN** Dynatrace environment credentials are available via flags or environment
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** credentials are resolved and passed to the uninstall flow

#### Scenario: Credentials missing

- **GIVEN** no environment URL or platform token is available
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the command exits with a credential error before any preview or destructive action
