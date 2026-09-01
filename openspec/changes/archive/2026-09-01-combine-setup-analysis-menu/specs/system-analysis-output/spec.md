# Spec: System Analysis Output (delta)

## MODIFIED Requirements

### Requirement: The System Analysis block is printed only by `dtwiz analyze` and `dtwiz status`

The System Analysis block (`info.Summary()`) SHALL be printed by `dtwiz analyze` and
`dtwiz status` only. `dtwiz setup` SHALL NOT print the System Analysis block; detection
context is instead surfaced inline on each recommendation menu entry.

#### Scenario: `dtwiz analyze` prints the System Analysis block

- **GIVEN** the user runs `dtwiz analyze`
- **WHEN** analysis completes
- **THEN** the full System Analysis block is printed to stdout

#### Scenario: `dtwiz status` prints the System Analysis block

- **GIVEN** the user runs `dtwiz status`
- **WHEN** analysis completes
- **THEN** the full System Analysis block is printed to stdout

#### Scenario: `dtwiz setup` does NOT print the System Analysis block

- **GIVEN** the user runs `dtwiz setup`
- **WHEN** analysis completes and the recommendation menu is shown
- **THEN** no System Analysis block is printed; detection context appears inline on each menu entry instead
