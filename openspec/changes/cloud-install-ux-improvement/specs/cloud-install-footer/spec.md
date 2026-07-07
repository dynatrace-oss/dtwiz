# Cloud Install Footer Specification

## ADDED Requirements

### Requirement: Cloud installs show Clouds app footer

After a successful AWS, GCP, or Azure install, the watch screen footer SHALL display "See your cloud resources in the Clouds app" with a link labelled "Open Clouds" pointing to `<appsURL>/ui/apps/dynatrace.clouds/smartscape/services`.

#### Scenario: AWS install footer

- **WHEN** `dtwiz install aws` completes and the watch screen renders
- **THEN** the footer shows "See your cloud resources in the Clouds app" and "Open Clouds" linking to the tenant's Clouds app URL

#### Scenario: GCP install footer

- **WHEN** `dtwiz install gcp` completes and the watch screen renders
- **THEN** the footer shows "See your cloud resources in the Clouds app" and "Open Clouds" linking to the tenant's Clouds app URL

#### Scenario: Azure install footer

- **WHEN** `dtwiz install azure` completes and the watch screen renders
- **THEN** the footer shows "See your cloud resources in the Clouds app" and "Open Clouds" linking to the tenant's Clouds app URL

### Requirement: Non-cloud installs keep QuickStart footer

For all other install methods, the watch screen footer SHALL continue to show "See all your data and findings in Dynatrace QuickStart" with an "Open Dynatrace QuickStart" link.

#### Scenario: OneAgent/OTel/Docker/K8s footer unchanged

- **WHEN** any non-cloud install completes and the watch screen renders
- **THEN** the footer shows the QuickStart link, not the Clouds app link

