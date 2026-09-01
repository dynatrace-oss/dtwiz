# Spec: Setup Recommendations

## Purpose

`dtwiz setup` generates a ranked recommendation list and lets the user pick an ingestion
method to install. This spec covers the rules for what is shown, what is actionable, and
how the UI behaves when some recommendations are informational-only.

---

## Requirements

### Requirement: `dtwiz setup` displays Done entries before the actionable list

The `setup` command SHALL print Done (✓) entries above the numbered option list so the
user has context before making a selection. Each Done entry SHALL include its detection
context on a second line in muted style.

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

---

### Requirement: The recommendation list is introduced by a signal lead-in

Both the interactive menu and the non-interactive recommendation listing SHALL print a single lead-in immediately below the section divider and above the entries, naming the signal types every recommendation ingests: logs, metrics, and traces. The signal types SHALL NOT be repeated inside individual entry titles.

#### Scenario: Interactive menu rendered

- **GIVEN** at least one actionable recommendation exists
- **WHEN** `dtwiz setup` renders the recommendations section
- **THEN** a lead-in naming logs, metrics, and traces is printed between the divider and the first entry

#### Scenario: Non-interactive listing rendered

- **GIVEN** at least one recommendation exists
- **WHEN** `dtwiz recommend` renders its list
- **THEN** the same lead-in is printed between the divider and the first entry

#### Scenario: Entry titles omit signal types

- **GIVEN** the lead-in has been printed
- **WHEN** any recommendation entry is rendered
- **THEN** its title describes what is monitored and by which method, and does not restate logs, metrics, or traces

---

### Requirement: Host-scoped recommendation titles state scope and method

Every recommendation that monitors the local host SHALL be titled with a shared scope phrase naming the host and its services, followed by a parenthetical naming the method that distinguishes it from the others. Two host-scoped entries SHALL NOT be distinguishable only by a trailing method name appended to an otherwise unrelated phrase, and no host-scoped title SHALL describe its scope as a machine's services.

#### Scenario: New collector option

- **GIVEN** the recommendation to deploy a new OpenTelemetry Collector is produced
- **WHEN** it is rendered in either the menu or the listing
- **THEN** its title names the host and its services, and identifies the method as a new OpenTelemetry Collector

#### Scenario: Existing collector option

- **GIVEN** an OpenTelemetry Collector is already running on the host
- **WHEN** the resulting recommendation is rendered
- **THEN** its title names the host and its services, and identifies the method as the existing OpenTelemetry Collector

#### Scenario: OneAgent option

- **GIVEN** the recommendation to install OneAgent on the host is produced
- **WHEN** it is rendered
- **THEN** its title names the host and its services, and identifies the method as OneAgent

#### Scenario: All three host-scoped options offered together

- **GIVEN** a collector is running, the platform supports OneAgent, and experimental recommendations are enabled
- **WHEN** the menu is rendered
- **THEN** all three entries share the same scope phrase and differ only in the method named in their parenthetical

---

### Requirement: Recommendation identity is independent of title text

Selection and dispatch SHALL key on a recommendation's method identifier rather than its title. Changing title wording SHALL NOT change which installer a selection invokes, nor the set or order of recommendations produced.

#### Scenario: Selecting a retitled entry

- **GIVEN** a recommendation's title wording has changed
- **WHEN** the user selects that entry in the menu
- **THEN** the same installer runs as before the wording changed

#### Scenario: Recommendation set unchanged

- **GIVEN** the same analyzed system
- **WHEN** recommendations are generated
- **THEN** the same methods are produced in the same order as before the wording changed

---

### Requirement: Each recommendation entry shows inline detection context

Every recommendation entry rendered in `dtwiz setup` SHALL display a second line
immediately below its title carrying detection context that confirms what was found.
For the host recommendation the context line SHALL include the hostname, OS name, and
architecture. For cloud and infrastructure recommendations the context line SHALL include
the key identifiers detected (cluster name, account ID, subscription ID, or project ID
as appropriate). The context line SHALL be rendered in muted style so it is visually
subordinate to the title.

#### Scenario: Host entry detection context shown

- **GIVEN** `dtwiz setup` renders the host recommendation
- **WHEN** the entry is displayed
- **THEN** a second line in muted style shows the hostname, OS name, and architecture

#### Scenario: Kubernetes entry detection context shown

- **GIVEN** a Kubernetes cluster is detected
- **WHEN** `dtwiz setup` renders the Kubernetes recommendation
- **THEN** a second line in muted style shows the cluster name and node count

#### Scenario: AWS entry detection context shown

- **GIVEN** an AWS account is detected
- **WHEN** `dtwiz setup` renders the AWS recommendation
- **THEN** a second line in muted style shows the account ID and region

---

### Requirement: Undetected infrastructure options appear as unavailable entries

`dtwiz setup` SHALL display Kubernetes, AWS, Azure, and GCP as non-selectable greyed
entries when they are not detected, grouped below the selectable options under a
"Sign in to unlock:" label. Each unavailable entry SHALL show the exact CLI command
a user must run to connect the provider and unlock the entry. Unavailable entries SHALL
NOT be numbered and SHALL NOT be selectable.

#### Scenario: Kubernetes not detected

- **GIVEN** no Kubernetes cluster is detected
- **WHEN** `dtwiz setup` renders the menu
- **THEN** a greyed, non-numbered Kubernetes entry appears showing `kubectl config use-context <name>`
- **THEN** it is NOT included in the numbered selection list

#### Scenario: AWS not detected

- **GIVEN** no AWS account is detected
- **WHEN** `dtwiz setup` renders the menu
- **THEN** a greyed, non-numbered AWS entry appears showing `aws configure`
- **THEN** it is NOT included in the numbered selection list

#### Scenario: Azure not detected

- **GIVEN** no Azure subscription is detected
- **WHEN** `dtwiz setup` renders the menu
- **THEN** a greyed, non-numbered Azure entry appears showing `az login`

#### Scenario: GCP not detected

- **GIVEN** no GCP project is detected
- **WHEN** `dtwiz setup` renders the menu
- **THEN** a greyed, non-numbered GCP entry appears showing `gcloud auth login`

#### Scenario: Provider already detected — no unavailable entry shown

- **GIVEN** an AWS account is detected
- **WHEN** `dtwiz setup` renders the menu
- **THEN** no unavailable AWS entry appears in the greyed section

#### Scenario: Unavailable entries excluded from `dtwiz recommend` output

- **GIVEN** one or more providers are not detected
- **WHEN** `dtwiz recommend` renders its recommendation list
- **THEN** no unavailable/greyed entries appear in the output
