# Spec: Setup Recommendations

## Overview

`dtwiz setup` generates a ranked recommendation list and lets the user pick an ingestion
method to install. This spec covers the rules for what is shown, what is actionable, and
how the UI behaves when some recommendations are informational-only.

---

## Requirements

### Requirement: `dtwiz setup` displays Done entries before the actionable list

The `setup` command SHALL print Done (✓) entries above the numbered option list so the
user has context before making a selection.

#### Scenario: OneAgent running — Done entry shown above OTel option

- **GIVEN** `OneAgentRunning = true`
- **WHEN** `dtwiz setup` renders the recommendations section
- **THEN** a `✓ Dynatrace OneAgent is already running` line is printed with its description
- **THEN** the numbered OTel option(s) appear below it

#### Scenario: No Done entries exist

- **GIVEN** `OneAgentRunning = false`
- **WHEN** `dtwiz setup` renders the recommendations section
- **THEN** no ✓ line is printed before the numbered list

---

### Requirement: When all recommendations are Done, exit cleanly without prompting

When `actionable` is empty (all recommendations have `Done=true`), `dtwiz setup` SHALL
print the Done entries with their descriptions and exit with code 0 — no prompt, no
numbered list, no "Cancel" option.

#### Scenario: All recommendations are Done

- **GIVEN** every recommendation in the list has `Done=true`
- **WHEN** `dtwiz setup` renders the section
- **THEN** each Done entry is printed with its ✓ badge and description
- **THEN** the command exits with code 0 and no confirmation prompt is shown
