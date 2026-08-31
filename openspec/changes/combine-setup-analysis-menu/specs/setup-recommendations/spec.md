## ADDED Requirements

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

---

## MODIFIED Requirements

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
