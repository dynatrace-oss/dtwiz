# Spec: autoconfirm flag

## ADDED Requirements

### Requirement: --yes / -y flag on install, update, and uninstall command groups

The `install`, `update`, and `uninstall` command groups SHALL each expose a `--yes` / `-y` persistent flag that suppresses all interactive confirmation prompts.

#### Scenario: --yes skips confirmProceed

- **WHEN** `--yes` or `-y` is passed to any install, update, or uninstall subcommand
- **THEN** `confirmProceed()` SHALL return `true` without printing the prompt or reading stdin
- **AND** execution SHALL proceed as if the user had pressed Enter

#### Scenario: --yes is not set

- **WHEN** neither `--yes` nor `-y` is passed
- **THEN** `confirmProceed()` SHALL behave as before: print the prompt and wait for user input

#### Scenario: --yes works with --dry-run

- **WHEN** both `--yes` and `--dry-run` are passed
- **THEN** `--dry-run` takes precedence: no actions are executed and no prompts are shown

---

### Requirement: confirmProceed lives in shared installer utilities

`confirmProceed()` and the `AutoConfirm` package-level variable SHALL be defined in `pkg/installer/installer.go`, not in `pkg/installer/kubernetes.go`.

#### Scenario: All installers share one confirmProceed

- **WHEN** any installer (kubernetes, otel, otel-python, aws, aws-lambda, aws-uninstall, oneagent-uninstall, kubernetes-uninstall, otel-update, otel-uninstall) calls `confirmProceed()`
- **THEN** it SHALL call the single definition in `installer.go`
- **AND** `kubernetes.go` SHALL NOT define its own `confirmProceed()`
