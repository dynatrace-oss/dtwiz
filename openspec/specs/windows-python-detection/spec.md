# Windows Python Detection

## Purpose

Define how `dtwiz analyze` detects Python on Windows, ignoring Windows Store stub executables.

## Requirements

### Requirement: Ignore Windows Store Python stubs

Python detection SHALL ignore Windows Store launch aliases and continue searching PATH for a real Python 3 interpreter.

#### Scenario: Store stub appears before real Python

- **WHEN** a Windows Store Python stub appears earlier on PATH than a real Python 3 interpreter
- **THEN** Python detection SHALL skip the stub
- **AND** it SHALL select the real Python 3 interpreter later on PATH

#### Scenario: No real Python 3 exists

- **WHEN** PATH contains only Python stubs or non-Python-3 executables
- **THEN** Python detection SHALL report that Python 3 was not found
