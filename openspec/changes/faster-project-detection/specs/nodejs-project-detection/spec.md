# Node.js Project Detection

## ADDED Requirements

### Requirement: Deeply nested projects are discovered

The project scanner SHALL NOT impose a recursion depth limit when descending under CWD. Projects nested more than 15 directory levels below CWD SHALL be discovered.

#### Scenario: package.json nested deeper than 15 levels below CWD

- **GIVEN** a `package.json` exists in a directory more than 15 levels below CWD
- **AND** no excluded directory (e.g. `node_modules`, `.git`) appears anywhere in the path
- **WHEN** the project scanner runs
- **THEN** that directory is included as a `ScannedProject`

### Requirement: Windows system directories are excluded from scanning

The project scanner SHALL skip Windows system directories regardless of platform. Specifically the directory names `Windows`, `System32`, `SysWOW64`, `WinSxS`, `ProgramData`, `AppData`, and `$Recycle.Bin`, as well as any directory name beginning with `$`, SHALL be skipped.

#### Scenario: package.json under a Windows system directory is ignored

- **GIVEN** a `package.json` exists inside a directory named `System32` (or any other Windows system directory in the list above) under CWD
- **WHEN** the project scanner runs
- **THEN** that `package.json` is NOT included as a `ScannedProject`

#### Scenario: package.json under a $-prefixed directory is ignored

- **GIVEN** a `package.json` exists inside a directory whose name begins with `$` (e.g. `$Recycle.Bin`, `$tmp`)
- **WHEN** the project scanner runs
- **THEN** that `package.json` is NOT included as a `ScannedProject`
