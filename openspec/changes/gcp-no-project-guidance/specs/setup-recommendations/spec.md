# Spec Delta

## MODIFIED Requirements

### Requirement: Undetected infrastructure options appear as unavailable entries

`dtwiz setup` SHALL display Kubernetes, AWS, Azure, and GCP as non-selectable greyed
entries when they are not detected, grouped below the selectable options under a
"Sign in to unlock:" label. Each unavailable entry SHALL show the exact CLI command
a user must run to connect the provider and unlock the entry. When unlocking takes more than one command, the entry SHALL show each command in backticks, so commands and prose are clearly separated. Unavailable entries SHALL
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
- **THEN** it is NOT included in the numbered selection list

#### Scenario: GCP not detected

- **GIVEN** no GCP project is detected and `gcloud` has no authenticated account
- **WHEN** `dtwiz setup` renders the menu
- **THEN** a greyed, non-numbered GCP entry appears showing `gcloud auth login`
- **THEN** it is NOT included in the numbered selection list

#### Scenario: GCP project configured but not signed in

- **GIVEN** `gcloud` has an active project configured but no authenticated account
- **WHEN** `dtwiz setup` renders the menu
- **THEN** GCP is not offered as a selectable option
- **THEN** a greyed, non-numbered GCP entry appears showing `gcloud auth login`

#### Scenario: gcloud warnings are not mistaken for a signed-in account

- **GIVEN** `gcloud` has no authenticated account and prints a warning while listing accounts
- **WHEN** `dtwiz setup` renders the menu
- **THEN** the greyed GCP entry shows `gcloud auth login`, not the project-selection hint

#### Scenario: GCP authenticated without a project

- **GIVEN** `gcloud` has an authenticated account but no active project
- **WHEN** `dtwiz setup` renders the menu
- **THEN** a greyed, non-numbered GCP entry appears showing `` `gcloud projects list`, then `gcloud config set project <PROJECT_ID>` ``
- **THEN** it is NOT included in the numbered selection list

#### Scenario: Provider already detected — no unavailable entry shown

- **GIVEN** an AWS account is detected
- **WHEN** `dtwiz setup` renders the menu
- **THEN** no unavailable AWS entry appears in the greyed section

#### Scenario: Unavailable entries excluded from `dtwiz recommend` output

- **GIVEN** one or more providers are not detected
- **WHEN** `dtwiz recommend` renders its recommendation list
- **THEN** no unavailable/greyed entries appear in the output
