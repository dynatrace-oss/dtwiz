# Cloud Recommend UX Specification

## ADDED Requirements

### Requirement: Setup menu distinguishes install vs update for Azure and GCP

When Azure or GCP is already configured, the recommender SHALL emit a distinct update method (`MethodAzureUpdate` / `MethodGCPUpdate`) and title ("Azure cloud services (update)" / "GCP cloud services (update)"). Setup SHALL dispatch on these constants directly, without local boolean state.

#### Scenario: Azure already configured — menu shows update

- **WHEN** `dtwiz setup` runs and Azure is already configured
- **THEN** the menu shows "Azure cloud services (update)" and selecting it runs the update flow

#### Scenario: GCP already configured — menu shows update

- **WHEN** `dtwiz setup` runs and GCP is already configured
- **THEN** the menu shows "GCP cloud services (update)" and selecting it runs the update flow

#### Scenario: Azure not configured — menu shows install

- **WHEN** `dtwiz setup` runs and Azure is not configured
- **THEN** the menu shows "Azure cloud services" and selecting it runs the install flow

#### Scenario: GCP not configured — menu shows install

- **WHEN** `dtwiz setup` runs and GCP is not configured
- **THEN** the menu shows "GCP cloud services" and selecting it runs the install flow

### Requirement: Connection pre-check runs before recommendations

Setup SHALL check Azure and GCP connection status before generating recommendations, so the recommender receives accurate `AzureConfigured` / `GCPConfigured` state.

#### Scenario: Pre-check feeds recommender

- **WHEN** `dtwiz setup` runs
- **THEN** Azure and GCP connection checks complete before `GenerateRecommendations` is called

### Requirement: OTel recommendation titles use consistent wording

The OTel update recommendation SHALL read "update existing OpenTelemetry Collector" (not "patch"). The new collector recommendation SHALL read "via new OpenTelemetry Collector".

#### Scenario: OTel update title

- **WHEN** an OTel Collector is already running and `dtwiz setup` or `dtwiz recommend` lists recommendations
- **THEN** the title contains "update existing OpenTelemetry Collector"

#### Scenario: New OTel collector title

- **WHEN** no OTel Collector is running and `dtwiz setup` or `dtwiz recommend` lists recommendations
- **THEN** the title contains "via new OpenTelemetry Collector"

